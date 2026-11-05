package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"warehouse-service/internal/auth"
	"warehouse-service/internal/config"
	"warehouse-service/internal/database"
	"warehouse-service/internal/server"
)

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Open(context.Background(), cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	authService := auth.NewService(db, cfg.JWTSecret)
	api := server.New(cfg, db, authService)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- api.Start()
	}()
	log.Printf("API listening on :%d (%s)", cfg.Port, cfg.AppEnv)

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := api.App.ShutdownWithContext(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
