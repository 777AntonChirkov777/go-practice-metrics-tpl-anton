package audit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent_JSONShape(t *testing.T) {
	body, err := json.Marshal(Event{
		Timestamp: 12345678,
		Metrics:   []string{"Alloc", "Frees"},
		IPAddress: "192.168.0.42",
	})
	require.NoError(t, err)

	assert.JSONEq(t, `{"ts":12345678,"metrics":["Alloc","Frees"],"ip_address":"192.168.0.42"}`, string(body))
}

func TestEvent_Reset(t *testing.T) {
	metrics := make([]string, 2, 8)
	metrics[0], metrics[1] = "Alloc", "Frees"
	e := &Event{Timestamp: 12345678, Metrics: metrics, IPAddress: "192.168.0.42"}

	e.Reset()

	assert.Zero(t, e.Timestamp)
	assert.Empty(t, e.IPAddress)
	require.NotNil(t, e.Metrics)
	assert.Len(t, e.Metrics, 0)
	assert.Equal(t, 8, cap(e.Metrics))
	assert.Same(t, &metrics[0], &e.Metrics[:1][0])
	assert.Equal(t, []string{"", ""}, metrics)

	var nilEvent *Event
	assert.NotPanics(t, nilEvent.Reset)
}
