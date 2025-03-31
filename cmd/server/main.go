package main

import (
	"context"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/api"
	"github.com/rishavmehra/indexer/internal/api/handlers"
	"github.com/rishavmehra/indexer/internal/api/middleware"
	"github.com/rishavmehra/indexer/internal/config"
	db "github.com/rishavmehra/indexer/internal/db/generated"
	"github.com/rishavmehra/indexer/internal/indexer"
	"github.com/rishavmehra/indexer/internal/service"
	"github.com/rishavmehra/indexer/pkg/logger"
)

func main() {
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	logger.SetupLogger(cfg.Logger.Level)

	if err := runMigrations(cfg.Database); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migrations")
	}

	pool, err := connectToDatabase(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	queries := db.New(pool)

	heliusClient := indexer.NewHeliusClient(
		cfg.Helius.APIKey,
		cfg.Helius.WebhookSecret,
		cfg.Helius.WebhookBaseURL,
		cfg.Helius.WebhookID,
	)

	authService := service.NewAuthService(cfg.JWT, queries)
	userService := service.NewUserService(queries)
	indexerService := service.NewIndexerService(queries, heliusClient)

	authHandler := handlers.NewAuthHandler(authService)
	userHandler := handlers.NewUserHandler(userService)
	indexerHandler := handlers.NewIndexerHandler(indexerService)

	mw := middleware.MiddlewareConfig{
		Auth: middleware.AuthMiddleware(cfg.JWT),
	}

	server := api.NewServer(cfg.Server)
	api.SetupRoutes(
		server.Router(),
		authHandler,
		userHandler,
		indexerHandler,
		mw,
	)

	if err := server.Start(); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}

// runMigrations runs database migrations
func runMigrations(dbConfig config.DatabaseConfig) error {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.DBName, dbConfig.SSLMode)

	m, err := migrate.New(dbConfig.MigrationURL, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info().Msg("Database migrations completed successfully")
	return nil
}

// connectToDatabase connects to the PostgreSQL database
func connectToDatabase(dbConfig config.DatabaseConfig) (*pgxpool.Pool, error) {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.DBName, dbConfig.SSLMode)

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().Msg("Connected to database")
	return pool, nil
}
