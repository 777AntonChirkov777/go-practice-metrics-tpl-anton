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

	l4, err := net.Listen("tcp4", "0.0.0.0:"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listen ipv4: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server is running on :%s\n", port)

	l6, err := net.Listen("tcp6", "[::]:"+port)
	if err == nil {
		// Если IPv6 доступен, запускаем IPv4 в фоне, а IPv6 блокирует main-горутину
		go http.Serve(l4, r)
		if err := http.Serve(l6, r); err != nil {
			fmt.Fprintf(os.Stderr, "error server ipv6: %v\n", err)
		}
	} else {
		// Если IPv6 не поддерживается ОС, просто слушаем IPv4 (блокируя main)
		if err := http.Serve(l4, r); err != nil {
			fmt.Fprintf(os.Stderr, "error server ipv4: %v\n", err)
		}
	}
}
