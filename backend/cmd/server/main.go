package main

import (
	"log"
	"net/http"
	"os"

	"ai-bot-chain/backend/internal/app"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present (supports running from repo root or from ./backend).
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	server, err := app.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
