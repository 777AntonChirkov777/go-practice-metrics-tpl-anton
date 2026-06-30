package main

import (
	"log"
	"net/http"

	handlers "practice/internal/handler"
	"practice/internal/storage"

	"github.com/go-chi/chi/v5"
)

func main() {
	log.Println("server start")

	store := storage.NewMemStorage() // загружает данные из файла при старте
	h := handlers.NewHandler(store)

	r := chi.NewRouter()
	r.Get("/", h.ListHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	log.Println("server occupied the port 8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
