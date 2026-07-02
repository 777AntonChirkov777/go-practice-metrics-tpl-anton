package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"practice/internal/config"
	handlers "practice/internal/handler"
	"practice/internal/storage"
	"strings"

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

	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		// Фоллбэк на случай, если адрес передан нестандартно
		port = strings.TrimPrefix(cfg.Address, ":")
	}

	fmt.Printf("Server is running on port %s\n", port)

	go func() {
		if err := http.ListenAndServe("127.0.0.1:"+port, r); err != nil {
			fmt.Fprintf(os.Stderr, "error ipv4 server: %v\n", err)
		}
	}()

	if err := http.ListenAndServe("[::1]:"+port, r); err != nil {
		fmt.Printf("IPv6 not available (%v), falling back to IPv4\n", err)
		if err := http.ListenAndServe("127.0.0.1:"+port, r); err != nil {
			fmt.Fprintf(os.Stderr, "error server connection: %v\n", err)
			os.Exit(1)
		}
	}
}
