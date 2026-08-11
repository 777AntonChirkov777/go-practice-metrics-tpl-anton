package handlers

import (
	"encoding/json"
	"net/http"
	model "practice/internal/model"
	"strings"
)

const maxBatchBodyBytes = 4 << 20

func (h *Handler) UpdatesJSONHandler(res http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(res, req.Body, maxBatchBodyBytes)

	var in []model.Metrics
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&in); err != nil {
		http.Error(res, "invalid json body", http.StatusBadRequest)
		return
	}
	if dec.More() {
		http.Error(res, "unexpected trailing data after json array", http.StatusBadRequest)
		return
	}

	if len(in) == 0 {
		res.WriteHeader(http.StatusOK)
		return
	}

	batch := make([]*model.Metric, 0, len(in))
	for _, w := range in {
		w.ID = strings.TrimSpace(w.ID)
		if w.ID == "" {
			http.Error(res, "metric id is required", http.StatusBadRequest)
			return
		}

		m, err := w.ToDomain()
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		_ = m.CalculateHash()

		batch = append(batch, m)
	}

	if err := h.store.SaveBatch(req.Context(), batch); err != nil {
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}
