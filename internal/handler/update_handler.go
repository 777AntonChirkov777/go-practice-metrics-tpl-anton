package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
)

var (
	storage = make(map[string]map[string]float64)
	mu      sync.RWMutex
)

func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")
	valueStr := chi.URLParam(r, "metricValue")

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		http.Error(w, "invalid value", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if storage[metricType] == nil {
		storage[metricType] = make(map[string]float64)
	}
	storage[metricType][metricName] = value

	w.WriteHeader(http.StatusOK)
}

func ValueHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")

	mu.RLock()
	defer mu.RUnlock()

	typeMap, ok := storage[metricType]
	if !ok {
		http.NotFound(w, r)
		return
	}

	val, ok := typeMap[metricName]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Возвращаем значение в текстовом виде
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%v", val)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, "<html><body><h1>Metrics</h1><table border='1'><tr><th>Type</th><th>Name</th><th>Value</th></tr>")

	for mType, names := range storage {
		for name, val := range names {
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%v</td></tr>", mType, name, val)
		}
	}

	fmt.Fprint(w, "</table></body></html>")
}
