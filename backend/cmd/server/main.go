package main

import (
	"log"
	"net/http"

	"bws-checkin/backend/internal/app"
	"bws-checkin/backend/internal/config"
)

func main() {
	cfg := config.Load()
	handler, cleanup, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler))
}
