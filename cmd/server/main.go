package main

import (
	"net/http"
	handlers "practice/internal/handler"

	"github.com/labstack/gommon/log"
)

func main() {
	log.Info("server start")

	mux := http.NewServeMux()
	mux.HandleFunc(`/update/`, handlers.UpdateHandler)

	log.Info("server occupied the port 8080")
	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
