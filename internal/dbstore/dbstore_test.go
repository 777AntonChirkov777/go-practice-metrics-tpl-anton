package dbstore

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	model "practice/internal/model"
)

func TestAggregate_SplitsByTypeAndSortsByID(t *testing.T) {
	gauges, counters, err := aggregate([]*model.Metric{
		model.NewGaugeMetric("Sys", 3),
		model.NewCountMetric("PollCount", 1),
		model.NewGaugeMetric("Alloc", 1),
		model.NewCountMetric("Errors", 2),
	})
	if err != nil {
		t.Fatalf("aggregate error: %v", err)
	}

	wantGauges := []gaugeRow{{id: "Alloc", value: 1}, {id: "Sys", value: 3}}
	if len(gauges) != len(wantGauges) {
		t.Fatalf("gauges = %+v, want %+v", gauges, wantGauges)
	}
	for i, want := range wantGauges {
		if gauges[i] != want {
			t.Errorf("gauges[%d] = %+v, want %+v", i, gauges[i], want)
		}
	}

	wantCounters := []counterRow{{id: "Errors", delta: 2}, {id: "PollCount", delta: 1}}
	if len(counters) != len(wantCounters) {
		t.Fatalf("counters = %+v, want %+v", counters, wantCounters)
	}
	for i, want := range wantCounters {
		if counters[i] != want {
			t.Errorf("counters[%d] = %+v, want %+v", i, counters[i], want)
		}
	}
}

func TestAggregate_CollapsesDuplicates(t *testing.T) {
	gauges, counters, err := aggregate([]*model.Metric{
		model.NewCountMetric("PollCount", 1),
		model.NewGaugeMetric("Alloc", 1.5),
		model.NewCountMetric("PollCount", 2),
		model.NewGaugeMetric("Alloc", 2.5),
		model.NewCountMetric("PollCount", 3),
	})
	if err != nil {
		t.Fatalf("aggregate error: %v", err)
	}

	if len(gauges) != 1 || gauges[0].value != 2.5 {
		t.Errorf("gauges = %+v, want single Alloc=2.5 (последний побеждает)", gauges)
	}
	if len(counters) != 1 || counters[0].delta != 6 {
		t.Errorf("counters = %+v, want single PollCount=6 (дельты суммируются)", counters)
	}
}

func TestAggregate_UnknownTypeFails(t *testing.T) {
	_, _, err := aggregate([]*model.Metric{
		model.NewGaugeMetric("Alloc", 1),
		{ID: "Latency", MType: 42},
	})

	if !errors.Is(err, model.ErrUnknownType) {
		t.Fatalf("err = %v, want %v", err, model.ErrUnknownType)
	}
}

func TestAggregate_EmptyBatch(t *testing.T) {
	gauges, counters, err := aggregate(nil)
	if err != nil {
		t.Fatalf("aggregate error: %v", err)
	}
	if len(gauges) != 0 || len(counters) != 0 {
		t.Errorf("gauges = %+v, counters = %+v, want both empty", gauges, counters)
	}
}

func TestBuildGaugeUpsert(t *testing.T) {
	st := buildGaugeUpsert([]gaugeRow{{id: "Alloc", value: 1.5}, {id: "Sys", value: 2.5}})

	const want = `INSERT INTO gauges (id, value) VALUES ($1, $2), ($3, $4)` +
		` ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value`
	if st.query != want {
		t.Errorf("query = %q, want %q", st.query, want)
	}

	wantArgs := []any{"Alloc", 1.5, "Sys", 2.5}
	if len(st.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", st.args, wantArgs)
	}
	for i := range wantArgs {
		if st.args[i] != wantArgs[i] {
			t.Errorf("args[%d] = %v, want %v", i, st.args[i], wantArgs[i])
		}
	}
}

func TestBuildCounterUpsert(t *testing.T) {
	st := buildCounterUpsert([]counterRow{{id: "PollCount", delta: 7}})

	const want = `INSERT INTO counters (id, delta) VALUES ($1, $2)` +
		` ON CONFLICT (id) DO UPDATE SET delta = counters.delta + EXCLUDED.delta`
	if st.query != want {
		t.Errorf("query = %q, want %q", st.query, want)
	}

	wantArgs := []any{"PollCount", int64(7)}
	if len(st.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", st.args, wantArgs)
	}
	for i := range wantArgs {
		if st.args[i] != wantArgs[i] {
			t.Errorf("args[%d] = %v, want %v", i, st.args[i], wantArgs[i])
		}
	}
}

func TestGaugeStatements_ChunkBoundaries(t *testing.T) {
	cases := []struct {
		rows           int
		wantStatements int
		wantLastArgs   int
	}{
		{rows: 0, wantStatements: 0},
		{rows: 1, wantStatements: 1, wantLastArgs: 2},
		{rows: upsertChunkSize, wantStatements: 1, wantLastArgs: upsertChunkSize * 2},
		{rows: upsertChunkSize + 1, wantStatements: 2, wantLastArgs: 2},
		{rows: upsertChunkSize*2 + 3, wantStatements: 3, wantLastArgs: 6},
	}

	for _, c := range cases {
		rows := make([]gaugeRow, c.rows)
		for i := range rows {
			rows[i] = gaugeRow{id: strconv.Itoa(i), value: float64(i)}
		}

		statements := gaugeStatements(rows)
		if len(statements) != c.wantStatements {
			t.Errorf("rows=%d: statements = %d, want %d", c.rows, len(statements), c.wantStatements)
			continue
		}

		total := 0
		for i, st := range statements {
			total += len(st.args)
			if !strings.HasPrefix(st.query, "INSERT INTO gauges (id, value) VALUES ($1, $2)") {
				t.Errorf("rows=%d: chunk %d must renumber placeholders from $1", c.rows, i)
			}
			if len(st.args) > upsertChunkSize*2 {
				t.Errorf("rows=%d: chunk %d has %d args, over the %d limit",
					c.rows, i, len(st.args), upsertChunkSize*2)
			}
		}
		if total != c.rows*2 {
			t.Errorf("rows=%d: chunks carry %d args in total, want %d", c.rows, total, c.rows*2)
		}
		if c.wantStatements > 0 {
			if last := statements[len(statements)-1]; len(last.args) != c.wantLastArgs {
				t.Errorf("rows=%d: last chunk args = %d, want %d", c.rows, len(last.args), c.wantLastArgs)
			}
		}
	}
}

func TestCounterStatements_ChunkBoundaries(t *testing.T) {
	rows := make([]counterRow, upsertChunkSize+1)
	for i := range rows {
		rows[i] = counterRow{id: strconv.Itoa(i), delta: int64(i)}
	}

	statements := counterStatements(rows)
	if len(statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(statements))
	}
	if len(statements[0].args) != upsertChunkSize*2 {
		t.Errorf("first chunk args = %d, want %d", len(statements[0].args), upsertChunkSize*2)
	}
	if len(statements[1].args) != 2 {
		t.Errorf("second chunk args = %d, want 2", len(statements[1].args))
	}
	if !strings.HasPrefix(statements[1].query, "INSERT INTO counters (id, delta) VALUES ($1, $2)") {
		t.Error("second chunk must renumber placeholders from $1")
	}
}
