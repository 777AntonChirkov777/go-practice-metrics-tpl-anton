package main

import (
	"net/http"
	"practice/internal/handler"

	"github.com/labstack/gommon/log"
)

func main() {
	log.Info("server start")

	mux := http.NewServeMux()
	mux.HandleFunc(`/update/`, handler.UpdateHandler)

	log.Info("server occupied the port 8080")
	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
