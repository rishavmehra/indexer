package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rishavmehra/indexer/internal/models"
)

type Indexer interface {
	Initialize(ctx context.Context, conn *pgx.Conn, targetTable string) error

	GetWebhookConfig(indexerID string) (WebhookConfig, error)

	ProcessPayload(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload) error
}

type TokenIndexer interface {
	Indexer
	ProcessPayloadWithMetadata(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload, heliusAPIKey string) error

	EnrichTokenMetadata(ctx context.Context, pool *pgxpool.Pool, targetTable string, heliusAPIKey string) error

	InitializeWithAPIKey(ctx context.Context, conn *pgx.Conn, targetTable string, heliusAPIKey string) error
}

type BaseIndexer struct {
	ID          string
	Params      json.RawMessage
	initialized bool
}

func NewBaseIndexer(id string, params json.RawMessage) BaseIndexer {
	return BaseIndexer{
		ID:     id,
		Params: params,
	}
}

func (b *BaseIndexer) Initialize(ctx context.Context, conn *pgx.Conn, targetTable string) error {
	if b.initialized {
		return nil
	}
	b.initialized = true
	return nil
}

func formatTableName(name string) string {
	formattedName := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			formattedName += string(c)
		} else {
			formattedName += "_"
		}
	}

	if len(formattedName) == 0 || !((formattedName[0] >= 'a' && formattedName[0] <= 'z') || (formattedName[0] >= 'A' && formattedName[0] <= 'Z')) {
		formattedName = "idx_" + formattedName
	}

	return formattedName
}

func checkTableExists(ctx context.Context, conn *pgx.Conn, tableName string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM pg_tables
			WHERE schemaname = 'public'
			AND tablename = $1
		)
	`, tableName).Scan(&exists)

	return exists, err
}

func executeWithRetry(attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", attempts, err)
}
