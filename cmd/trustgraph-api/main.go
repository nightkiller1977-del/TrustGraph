package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/api"
	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

func main() {
	// Parse configuration from environment
	var cfg config.Config
	if err := envconfig.Process("TRUSTGRAPH", &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("TrustGraph API starting",
		zap.String("version", "0.1.0"),
		zap.String("environment", cfg.Environment),
		zap.String("port", cfg.Port),
	)

	// Initialize database connection pool
	db, err := store.NewPostgres(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	logger.Info("Database connected successfully")

	// Run migrations
	if err := store.RunMigrations(db, "migrations"); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	logger.Info("Migrations completed successfully")

	// Initialize API server
	router := api.NewRouter(db, logger, &cfg)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down gracefully", zap.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}
	}()

	logger.Info("Starting HTTP server", zap.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("HTTP server error", zap.Error(err))
	}

	logger.Info("Server stopped")
}
