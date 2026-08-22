package dbstore

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	model "practice/internal/model"
	"practice/internal/retry"
	"practice/internal/storage"
)

const upsertChunkSize = 1000

type Storage struct {
	db     *sql.DB
	delays []time.Duration
}

var _ storage.MetricStorage = (*Storage)(nil)

func New(db *sql.DB) *Storage {
	return &Storage{db: db, delays: retry.DefaultDelays}
}

func (s *Storage) Save(ctx context.Context, m *model.Metric) error {
	return retry.Do(ctx, "dbstore.Save", s.delays, isRetriable, func(ctx context.Context) error {
		return s.saveOnce(ctx, m)
	})
}

func (s *Storage) saveOnce(ctx context.Context, m *model.Metric) error {
	switch model.MetricType(m.MType) {
	case model.Gauge:
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO gauges (id, value) VALUES ($1, $2)
			 ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value`,
			m.ID, m.Value)
		return err
	case model.Counter:
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO counters (id, delta) VALUES ($1, $2)
			 ON CONFLICT (id) DO UPDATE SET delta = counters.delta + EXCLUDED.delta`,
			m.ID, m.Delta)
		return err
	default:
		return fmt.Errorf("%w: %d", model.ErrUnknownType, m.MType)
	}
}

func (s *Storage) SaveBatch(ctx context.Context, metrics []*model.Metric) error {
	return retry.Do(ctx, "dbstore.SaveBatch", s.delays, isRetriable, func(ctx context.Context) error {
		return s.saveBatchOnce(ctx, metrics)
	})
}

func (s *Storage) saveBatchOnce(ctx context.Context, metrics []*model.Metric) error {
	gauges, counters, err := aggregate(metrics)
	if err != nil {
		return err
	}

	statements := append(gaugeStatements(gauges), counterStatements(counters)...)
	if len(statements) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, st := range statements {
		if _, err := tx.ExecContext(ctx, st.query, st.args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type gaugeRow struct {
	id    string
	value float64
}

type counterRow struct {
	id    string
	delta int64
}

type statement struct {
	query string
	args  []any
}

func aggregate(metrics []*model.Metric) ([]gaugeRow, []counterRow, error) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	for _, m := range metrics {
		switch model.MetricType(m.MType) {
		case model.Gauge:
			gauges[m.ID] = m.Value
		case model.Counter:
			counters[m.ID] += m.Delta
		default:
			return nil, nil, fmt.Errorf("%w: %d", model.ErrUnknownType, m.MType)
		}
	}

	gaugeRows := make([]gaugeRow, 0, len(gauges))
	for id, value := range gauges {
		gaugeRows = append(gaugeRows, gaugeRow{id: id, value: value})
	}
	slices.SortFunc(gaugeRows, func(a, b gaugeRow) int { return cmp.Compare(a.id, b.id) })

	counterRows := make([]counterRow, 0, len(counters))
	for id, delta := range counters {
		counterRows = append(counterRows, counterRow{id: id, delta: delta})
	}
	slices.SortFunc(counterRows, func(a, b counterRow) int { return cmp.Compare(a.id, b.id) })

	return gaugeRows, counterRows, nil
}

func gaugeStatements(rows []gaugeRow) []statement {
	out := make([]statement, 0)
	for chunk := range slices.Chunk(rows, upsertChunkSize) {
		out = append(out, buildGaugeUpsert(chunk))
	}
	return out
}

func counterStatements(rows []counterRow) []statement {
	out := make([]statement, 0)
	for chunk := range slices.Chunk(rows, upsertChunkSize) {
		out = append(out, buildCounterUpsert(chunk))
	}
	return out
}

func buildGaugeUpsert(rows []gaugeRow) statement {
	args := make([]any, 0, len(rows)*2)
	for _, r := range rows {
		args = append(args, r.id, r.value)
	}

	return statement{
		query: `INSERT INTO gauges (id, value) VALUES ` + valuePlaceholders(len(rows)) +
			` ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value`,
		args: args,
	}
}

func buildCounterUpsert(rows []counterRow) statement {
	args := make([]any, 0, len(rows)*2)
	for _, r := range rows {
		args = append(args, r.id, r.delta)
	}

	return statement{
		query: `INSERT INTO counters (id, delta) VALUES ` + valuePlaceholders(len(rows)) +
			` ON CONFLICT (id) DO UPDATE SET delta = counters.delta + EXCLUDED.delta`,
		args: args,
	}
}

func valuePlaceholders(rows int) string {
	var b strings.Builder
	for i := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "($%d, $%d)", i*2+1, i*2+2)
	}
	return b.String()
}

func (s *Storage) Get(ctx context.Context, mtype model.MetricType, name string) (*model.Metric, error) {
	var m *model.Metric

	err := retry.Do(ctx, "dbstore.Get", s.delays, isRetriable, func(ctx context.Context) error {
		var err error
		m, err = s.getOnce(ctx, mtype, name)
		return err
	})
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (s *Storage) getOnce(ctx context.Context, mtype model.MetricType, name string) (*model.Metric, error) {
	switch mtype {
	case model.Gauge:
		var value float64
		if err := s.db.QueryRowContext(ctx,
			`SELECT value FROM gauges WHERE id = $1`, name).Scan(&value); err != nil {
			return nil, notFoundOr(err)
		}
		return model.NewGaugeMetric(name, value), nil
	case model.Counter:
		var delta int64
		if err := s.db.QueryRowContext(ctx,
			`SELECT delta FROM counters WHERE id = $1`, name).Scan(&delta); err != nil {
			return nil, notFoundOr(err)
		}
		return model.NewCountMetric(name, delta), nil
	default:
		return nil, storage.ErrNotFound
	}
}

func (s *Storage) GetAll(ctx context.Context) ([]*model.Metric, error) {
	var all []*model.Metric

	err := retry.Do(ctx, "dbstore.GetAll", s.delays, isRetriable, func(ctx context.Context) error {
		var err error
		all, err = s.getAllOnce(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}

	return all, nil
}

func (s *Storage) getAllOnce(ctx context.Context) ([]*model.Metric, error) {
	gauges, err := s.allGauges(ctx)
	if err != nil {
		return nil, err
	}

	counters, err := s.allCounters(ctx)
	if err != nil {
		return nil, err
	}

	return append(gauges, counters...), nil
}

func (s *Storage) allGauges(ctx context.Context) ([]*model.Metric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, value FROM gauges ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*model.Metric, 0)
	for rows.Next() {
		var (
			id    string
			value float64
		)
		if err := rows.Scan(&id, &value); err != nil {
			return nil, err
		}
		out = append(out, model.NewGaugeMetric(id, value))
	}

	return out, rows.Err()
}

func (s *Storage) allCounters(ctx context.Context) ([]*model.Metric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, delta FROM counters ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*model.Metric, 0)
	for rows.Next() {
		var (
			id    string
			delta int64
		)
		if err := rows.Scan(&id, &delta); err != nil {
			return nil, err
		}
		out = append(out, model.NewCountMetric(id, delta))
	}

	return out, rows.Err()
}

func notFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	}
	return err
}
