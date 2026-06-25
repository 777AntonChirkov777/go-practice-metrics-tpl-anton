package handlers

import (
	"net/http"
	models "practice/internal/model"
	services "practice/internal/service"
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

	typeMetric := models.GetTypeMetric(parts[0])

	if typeMetric == models.Unknown {
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

	if !models.IsParseMetricValue(typeMetric, parts[2]) {
		http.Error(res, "Invalid <MEANING> metric. Only <gauge>(float64) or <counter>(int64) values are supported.", http.StatusBadRequest)
		return
	}

	services.SaveMetric(typeMetric, parts[1], parts[2])

	res.WriteHeader(http.StatusOK)
}
