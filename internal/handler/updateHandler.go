package handler

import (
	"net/http"
	models "practice/internal/model"
	"strconv"
	"strings"
)

func UpdateHandler(res http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodPost {
		http.Error(res, "Only POST requests are allowed!", http.StatusBadRequest)
		return
	}

	contentType := req.Header.Get("Content-Type")
	if contentType != "text/plain" {
		http.Error(res, "Only contentType text/plain", http.StatusBadRequest)
		return
	}

	path := strings.TrimPrefix(req.URL.Path, "/update/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		http.Error(res, "Invalid URL format. Expected /update/<TYPE>/<NAME>/<MEANING>", http.StatusNotFound)
		return
	}

	if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
		http.Error(res, "Invalid URL format. Expected /update/<TYPE>/<NAME>/<MEANING>", http.StatusNotFound)
		return
	}

	typeMetric := getTypeMetric(parts[0])

	if typeMetric == "null" {
		http.Error(res, "Invalid <TYPE> metric. Only <gauge>(float64) or <counter>(int64) values are supported.", http.StatusBadRequest)
		return
	}

	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		http.Error(res, "Invalid URL format. Expected /update/<TYPE>/<NAME>/<MEANING>", http.StatusNotFound)
		return
	}

	if len(parts) < 3 || strings.TrimSpace(parts[2]) == "" {
		http.Error(res, "Invalid URL format. Expected /update/<TYPE>/<NAME>/<MEANING>", http.StatusNotFound)
		return
	}

	if !parseMetricValue(typeMetric, parts[2]) {
		http.Error(res, "Invalid <MEANING> metric. Only <gauge>(float64) or <counter>(int64) values are supported.", http.StatusBadRequest)
		return
	}

	res.WriteHeader(http.StatusOK)
}

func getTypeMetric(typeMetric string) string {
	if typeMetric == models.Gauge {
		return "float64"
	}

	if typeMetric == models.Counter {
		return "int64"
	}

	return "null"
}

func parseMetricValue(typeMetric, valueStr string) bool {
	switch typeMetric {
	case models.Gauge:
		if _, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return true
		} else {
			return false
		}
	case models.Counter:
		if _, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return true
		} else {
			return false
		}
	default:
		return false
	}
}
