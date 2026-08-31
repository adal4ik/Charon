package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adal4ik/Charon/internal/httpapi"
	"github.com/adal4ik/Charon/internal/repository"
	"github.com/adal4ik/Charon/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHTTPAddress = ":8080"
	startupTimeout     = 10 * time.Second
	shutdownTimeout    = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("ledger stopped: %v", err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	httpAddress := os.Getenv("HTTP_ADDR")
	if httpAddress == "" {
		httpAddress = defaultHTTPAddress
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), startupTimeout)
	defer startupCancel()

	pool, err := pgxpool.New(startupCtx, databaseURL)
	if err != nil {
		return fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(startupCtx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	accountRepository := repository.NewPostgresRepository(pool)
	accountService := service.New(accountRepository)
	handler := httpapi.NewHandler(accountService)
	router := httpapi.NewRouter(handler)

	server := &http.Server{
		Addr:              httpAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on %s", httpAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run HTTP server: %w", err)
		}
		return nil
	case <-signalCtx.Done():
		log.Printf("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		shutdownErr := fmt.Errorf("shutdown HTTP server: %w", err)

		if closeErr := server.Close(); closeErr != nil {
			shutdownErr = errors.Join(
				shutdownErr,
				fmt.Errorf("force close HTTP server: %w", closeErr),
			)
		}

		serveErr := <-serverErrors
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			shutdownErr = errors.Join(
				shutdownErr,
				fmt.Errorf("run HTTP server: %w", serveErr),
			)
		}

		return shutdownErr
	}

	serveErr := <-serverErrors
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("run HTTP server: %w", serveErr)
	}

	return nil
}
