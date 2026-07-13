package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	//configF "practice/internal/config/flag"
	config "practice/internal/config"
	handlers "practice/internal/handler"
	"practice/internal/storage"
	"strings"

	"github.com/go-chi/chi/v5"
)

func main() {
	fmt.Println("server start")

	cfg, err := config.GetServerConfig()

	//cfg, err := configF.ParseServerFlags(os.Args[1:])
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

	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		// Фоллбэк, если адрес передан без хоста (например, ":8080") или нестандартно.
		port = strings.TrimPrefix(cfg.Address, ":")
	}
	listenAddr := ":" + port

	fmt.Printf("Server is running on %s\n", cfg.Address)

	if err := http.ListenAndServe(listenAddr, r); err != nil {
		fmt.Fprintf(os.Stderr, "error server connection: %v\n", err)
		os.Exit(1)
	}
}
