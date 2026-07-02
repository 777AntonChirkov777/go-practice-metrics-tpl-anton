package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

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

	store := storage.NewMemStorage()
	h := handlers.NewHandler(store)

	r := chi.NewRouter()
	r.Get("/", h.ListHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	addr := cfg.Address
	if strings.HasPrefix(addr, "localhost:") {
		addr = ":" + strings.TrimPrefix(addr, "localhost:")
	}

	fmt.Printf("Server is running on %s\n", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		fmt.Fprintf(os.Stderr, "error server connection: %v\n", err)
		os.Exit(1)
	}
}
