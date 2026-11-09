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
