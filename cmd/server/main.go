package main

import (
	"fmt"
	"net/http"
	"os"

	"practice/internal/config"
	handlers "practice/internal/handler"
	"practice/internal/storage"

	"github.com/go-chi/chi/v5"
)

func main() {

	fmt.Println("server start")

	cfg, err := config.ParseServerFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error configuration: %v\n", err)
		os.Exit(1)
	}

	store := storage.NewMemStorage() // загружает данные из файла при старте
	h := handlers.NewHandler(store)

	r := chi.NewRouter()
	r.Get("/", h.ListHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	fmt.Printf("Server is running on %s\n", cfg.Address)
	if err := http.ListenAndServe(cfg.Address, r); err != nil {
		fmt.Fprintf(os.Stderr, "error server connection: %v\n", err)
	}
}
