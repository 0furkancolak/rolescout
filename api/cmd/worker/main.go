package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/infrastructure/database"
)

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	startupCtx, startupCancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	startupCancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Println("RoleScout worker started; waiting for ingestion jobs")
	<-ctx.Done()
	log.Println("RoleScout worker shutting down")
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("Worker stopped: %v", err)
		os.Exit(1)
	}
}
