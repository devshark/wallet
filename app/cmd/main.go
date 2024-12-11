package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devshark/wallet/app/internal/migration"
	"github.com/devshark/wallet/app/internal/repository"
	"github.com/devshark/wallet/app/rest"
	"github.com/devshark/wallet/pkg/env"
)

const (
	shutdownTimeout = 5 * time.Second
	readTimeout     = 5 * time.Second
	writeTimeout    = 10 * time.Second
)

func main() {
	ctx := context.Background()

	config := NewConfig()
	logger := log.Default()

	connStr := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=disable", config.postgres.User, config.postgres.Password, config.postgres.Host, config.postgres.Port, config.postgres.Database)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	migrator := migration.NewMigratorWithLogger(db, logger, "migrations")
	if err = migrator.Up(ctx); err != nil {
		logger.Fatalf("Failed to migrate database: %v", err)
	}

	logger.Println("Database migrated successfully")

	repo := repository.NewPostgresRepository(db)

	router := rest.NewRouter(repo, logger)

	server := rest.NewHttpServer(router, config.port, readTimeout, writeTimeout)

	stop := make(chan os.Signal, 1)
	signal.Notify(
		stop,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	go func() {
		logger.Printf("listening on port %d", config.port)

		if err := server.ListenAndServe(); !errors.Is(err, nil) && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("http server failed to start: %v", err)
		}

		logger.Println("http server stopped")
	}()

	logger.Print("the app is running")

	<-stop

	log.Print("Shutting down...")
	// if Shutdown takes too long, cancel the context
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Shutdown", err)
	}

	log.Print("Gracefully stopped.")
}

type DbConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type Config struct {
	port     int64
	postgres DbConfig
}

func NewConfig() *Config {
	return &Config{
		port: env.GetEnvInt64("PORT", 8080),
		postgres: DbConfig{
			Host:     env.RequireEnv("POSTGRES_HOST"),
			Port:     env.RequireEnv("POSTGRES_PORT"),
			User:     env.RequireEnv("POSTGRES_USER"),
			Password: env.RequireEnv("POSTGRES_PASSWORD"),
			Database: env.RequireEnv("POSTGRES_DATABASE"),
		},
	}
}
