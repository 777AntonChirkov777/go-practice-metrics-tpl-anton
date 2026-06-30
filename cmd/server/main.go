// main.go
package main

import (
	"log"
	"net/http"
	handler "practice/internal/handler"

	"github.com/go-chi/chi/v5"
)

func main() {
	log.Println("server start")

	r := chi.NewRouter()

	// Маршрут обновления метрики (с параметрами в пути)
	r.Post("/update/{metricType}/{metricName}/{metricValue}", handler.UpdateHandler)

	// Получение значения метрики
	r.Get("/value/{metricType}/{metricName}", handler.ValueHandler)

	// Список всех метрик (HTML)
	r.Get("/", handler.IndexHandler)

	log.Println("server occupied the port 8080")
	err := http.ListenAndServe(":8080", r)
	if err != nil {
		panic(err)
	}
}
