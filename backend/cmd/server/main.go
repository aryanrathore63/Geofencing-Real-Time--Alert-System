package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dewanshpatle/geofencing-alert-system/backend/internal/app"
)

func main() {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatalf("PORT must be an integer between 1 and 65535")
	}
	databasePath := strings.TrimSpace(os.Getenv("DATABASE_PATH"))
	if databasePath == "" {
		databasePath = "./data/geofencing.db"
	}
	store, err := app.OpenStore(databasePath)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer store.Close()

	origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	if strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")) == "" {
		origins = []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:3000"}
	}
	hub := app.NewHub(origins)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app.NewAPI(store, hub, origins),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	log.Printf("geofencing API listening on 0.0.0.0:%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve API: %v", err)
	}
}
