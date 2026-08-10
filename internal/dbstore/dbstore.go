package dbstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	model "practice/internal/model"
	"practice/internal/storage"
)

type Storage struct {
	db *sql.DB
}

var _ storage.MetricStorage = (*Storage)(nil)

func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}

func (s *Storage) Save(ctx context.Context, m *model.Metric) error {
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

func (s *Storage) Get(ctx context.Context, mtype model.MetricType, name string) (*model.Metric, error) {
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
