package handlers

import (
	"fmt"
	"net/http"
	model "practice/internal/model"
	"practice/internal/storage"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store storage.MetricStorage
}

func NewHandler(store storage.MetricStorage) *Handler {
	return &Handler{store: store}
}

// POST /update/{type}/{name}/{value}
func (h *Handler) UpdateHandler(res http.ResponseWriter, req *http.Request) {
	typeStr := chi.URLParam(req, "type")
	name := strings.TrimSpace(chi.URLParam(req, "name"))
	valueStr := strings.TrimSpace(chi.URLParam(req, "value"))

	if typeStr == "" || name == "" || valueStr == "" {
		http.Error(res, "Expected /update/{type}/{name}/{value}", http.StatusBadRequest)
		return
	}

	mtype := model.GetTypeMetric(typeStr)
	if mtype == model.Unknown {
		http.Error(res, "Invalid metric type", http.StatusBadRequest)
		return
	}
	if !model.IsParseMetricValue(mtype, valueStr) {
		http.Error(res, "Invalid metric value", http.StatusBadRequest)
		return
	}

	var m *model.Metric
	switch mtype {
	case model.Gauge:
		v, _ := model.ParseGauge(mtype, valueStr)
		m = model.NewGaugeMetric(name, v)
	case model.Counter:
		v, _ := model.ParseCounter(mtype, valueStr)
		m = model.NewCountMetric(name, v)
	}
	_ = m.CalculateHash()

	if err := h.store.Save(m); err != nil {
		http.Error(res, "storage error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}

// GET /value/{type}/{name}
func (h *Handler) ValueHandler(res http.ResponseWriter, req *http.Request) {
	typeStr := chi.URLParam(req, "type")
	name := strings.TrimSpace(chi.URLParam(req, "name"))

	mtype := model.GetTypeMetric(typeStr)
	if mtype == model.Unknown {
		http.Error(res, "Invalid metric type", http.StatusBadRequest)
		return
	}

	m, ok := h.store.Get(mtype, name)
	if !ok {
		http.Error(res, "metric not found", http.StatusNotFound)
		return
	}

	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(http.StatusOK)

	switch mtype {
	case model.Gauge:
		res.Write([]byte(strconv.FormatFloat(m.Value, 'f', -1, 64)))
	case model.Counter:
		res.Write([]byte(strconv.FormatInt(m.Delta, 10)))
	}
}

// GET / — HTML со списком всех метрик
func (h *Handler) ListHandler(res http.ResponseWriter, req *http.Request) {
	all := h.store.GetAll()

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Metrics</title></head><body>`)
	b.WriteString("<h1>Metrics</h1><ul>")
	if len(all) == 0 {
		b.WriteString("<li>no metrics yet</li>")
	}
	for _, m := range all {
		typeName := model.MetricType(m.MType).String()
		var val string
		if m.MType == int8(model.Gauge) {
			val = strconv.FormatFloat(m.Value, 'f', -1, 64)
		} else {
			val = strconv.FormatInt(m.Delta, 10)
		}
		b.WriteString(fmt.Sprintf("<li><b>%s</b> [%s] = %s</li>", m.ID, typeName, val))
	}
	b.WriteString("</ul></body></html>")

	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(b.String()))
}
