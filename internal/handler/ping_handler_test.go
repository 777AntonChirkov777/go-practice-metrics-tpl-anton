package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakePinger struct {
	err error
}

func (f fakePinger) PingContext(ctx context.Context) error {
	return f.err
}

func TestPingHandler(t *testing.T) {
	tests := []struct {
		name       string
		pinger     Pinger
		wantStatus int
	}{
		{
			name:       "database is not configured",
			pinger:     nil,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "database is unreachable",
			pinger:     fakePinger{err: errors.New("connection refused")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "database is alive",
			pinger:     fakePinger{},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPingHandler(tt.pinger)

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rec := httptest.NewRecorder()

			h.Ping(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
