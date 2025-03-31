package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	db "github.com/rishavmehra/indexer/internal/db/generated"
	"github.com/rishavmehra/indexer/internal/indexer"
	"github.com/rishavmehra/indexer/internal/models"
	"github.com/rishavmehra/indexer/pkg/validator"
)

type IndexerService struct {
	store        db.Querier
	heliusClient *indexer.HeliusClient
	indexers     map[uuid.UUID]indexer.Indexer
	heliusAPIKey string
}

func NewIndexerService(store db.Querier, heliusClient *indexer.HeliusClient) *IndexerService {

	var apiKey string
	if heliusClient != nil {
		apiKey = heliusClient.GetAPIKey()
	}

	return &IndexerService{
		store:        store,
		heliusClient: heliusClient,
		indexers:     make(map[uuid.UUID]indexer.Indexer),
		heliusAPIKey: apiKey,
	}
}

func (s *IndexerService) CreateIndexer(ctx context.Context, userID uuid.UUID, req models.CreateIndexerRequest) (*models.IndexerResponse, error) {

	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(userID.String()); err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	var pgCredID pgtype.UUID
	if err := pgCredID.Scan(req.DBCredentialID.String()); err != nil {
		return nil, fmt.Errorf("invalid DB credential ID: %w", err)
	}

	cred, err := s.store.GetDBCredentialByID(ctx, pgCredID)
	if err != nil {
		return nil, errors.New("database credential not found")
	}

	credUserID, err := uuid.Parse(cred.UserID.String())
	if err != nil || credUserID != userID {
		return nil, errors.New("database credential not found")
	}

	if !validator.IsValidTableName(req.TargetTable) {
		return nil, errors.New("invalid target table name")
	}

	if err := validator.ValidateIndexerParams(string(req.IndexerType), req.Params); err != nil {
		return nil, err
	}

	createdIndexer, err := s.store.CreateIndexer(ctx, db.CreateIndexerParams{
		UserID:         pgUserID,
		DbCredentialID: pgCredID,
		IndexerType:    db.IndexerType(req.IndexerType),
		Params:         req.Params,
		TargetTable:    req.TargetTable,
		Status:         db.IndexerStatusPending,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create indexer")
		return nil, errors.New("failed to create indexer")
	}

	var addresses []string
	switch req.IndexerType {
	case models.TokenBorrow, models.TokenPrices:
		var tokenParams struct {
			Tokens []string `json:"tokens"`
		}
		if err := json.Unmarshal(req.Params, &tokenParams); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal token parameters")
		} else {
			addresses = tokenParams.Tokens
		}
	case models.NFTBids, models.NFTPrices:
		var nftParams struct {
			Collection string `json:"collection"`
		}
		if err := json.Unmarshal(req.Params, &nftParams); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal NFT parameters")
		} else {
			if nftParams.Collection != "" {
				addresses = append(addresses, nftParams.Collection)
			}
		}
	}

	if err := s.initializeIndexer(ctx, createdIndexer); err != nil {

		var errText pgtype.Text
		errText.String = err.Error()
		errText.Valid = true

		_, updateErr := s.store.UpdateIndexerStatus(ctx, db.UpdateIndexerStatusParams{
			ID:           createdIndexer.ID,
			Status:       db.IndexerStatusFailed,
			ErrorMessage: errText,
		})
		if updateErr != nil {
			log.Error().Err(updateErr).Msg("Failed to update indexer status")
		}

		return nil, fmt.Errorf("failed to initialize indexer: %w", err)
	}

	if s.heliusClient != nil && len(addresses) > 0 {
		webhookID, err := s.createHeliusWebhook(ctx, createdIndexer, addresses)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create Helius webhook")

			var errText pgtype.Text
			errText.String = fmt.Sprintf("Failed to create webhook: %s", err.Error())
			errText.Valid = true

			_, updateErr := s.store.UpdateIndexerStatus(ctx, db.UpdateIndexerStatusParams{
				ID:           createdIndexer.ID,
				Status:       db.IndexerStatusFailed,
				ErrorMessage: errText,
			})
			if updateErr != nil {
				log.Error().Err(updateErr).Msg("Failed to update indexer status")
			}

			return nil, fmt.Errorf("failed to create Helius webhook: %w", err)
		}

		var webhookText pgtype.Text
		webhookText.String = createdIndexer.ID.String()
		webhookText.Valid = true

		log.Info().
			Str("indexerID", createdIndexer.ID.String()).
			Str("heliusWebhookID", webhookID).
			Msg("Mapping indexer ID to Helius webhook ID")

		createdIndexer, err = s.store.UpdateIndexerWebhookID(ctx, db.UpdateIndexerWebhookIDParams{
			ID:        createdIndexer.ID,
			WebhookID: webhookText,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to update indexer webhook ID")

		}
	}

	details, _ := json.Marshal(map[string]interface{}{
		"targetTable": createdIndexer.TargetTable,
		"webhookID":   createdIndexer.WebhookID.String,
		"addresses":   addresses,
	})

	_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
		IndexerID: createdIndexer.ID,
		EventType: "initialization",
		Message:   "Indexer initialized successfully",
		Details:   details,
	})
	if logErr != nil {
		log.Error().Err(logErr).Msg("Failed to create initialization log entry")

	}

	var emptyText pgtype.Text
	emptyText.Valid = false

	createdIndexer, err = s.store.UpdateIndexerStatus(ctx, db.UpdateIndexerStatusParams{
		ID:           createdIndexer.ID,
		Status:       db.IndexerStatusActive,
		ErrorMessage: emptyText,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to update indexer status")

	}

	var params interface{}
	if err := json.Unmarshal(createdIndexer.Params, &params); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal indexer params")
		params = map[string]interface{}{}
	}

	idUUID, err := uuid.Parse(createdIndexer.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse indexer ID")
		return nil, errors.New("internal server error")
	}

	userIDUUID, err := uuid.Parse(createdIndexer.UserID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		return nil, errors.New("internal server error")
	}

	dbCredIDUUID, err := uuid.Parse(createdIndexer.DbCredentialID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse DB credential ID")
		return nil, errors.New("internal server error")
	}

	var lastIndexedAt *time.Time
	if createdIndexer.LastIndexedAt.Valid {
		t := createdIndexer.LastIndexedAt.Time
		lastIndexedAt = &t
	}

	return &models.IndexerResponse{
		ID:             idUUID,
		UserID:         userIDUUID,
		DBCredentialID: dbCredIDUUID,
		IndexerType:    models.IndexerType(createdIndexer.IndexerType),
		Params:         params,
		TargetTable:    createdIndexer.TargetTable,
		WebhookID:      createdIndexer.WebhookID.String,
		Status:         models.IndexerStatus(createdIndexer.Status),
		LastIndexedAt:  lastIndexedAt,
		ErrorMessage:   createdIndexer.ErrorMessage.String,
		CreatedAt:      createdIndexer.CreatedAt.Time,
		UpdatedAt:      createdIndexer.UpdatedAt.Time,
	}, nil
}

func (s *IndexerService) GetDefaultWebhookID() string {
	if s.heliusClient != nil {
		return s.heliusClient.GetDefaultWebhookID()
	}
	return ""
}

func (s *IndexerService) GetIndexersByUserID(ctx context.Context, userID uuid.UUID) ([]models.IndexerResponse, error) {

	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(userID.String()); err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	indexerList, err := s.store.GetIndexersByUserID(ctx, pgUserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexers")
		return nil, errors.New("failed to retrieve indexers")
	}

	response := make([]models.IndexerResponse, len(indexerList))
	for i, idx := range indexerList {
		var params interface{}
		if err := json.Unmarshal(idx.Params, &params); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal indexer params")
			params = map[string]interface{}{}
		}

		idUUID, err := uuid.Parse(idx.ID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse indexer ID")
			continue
		}

		userIDUUID, err := uuid.Parse(idx.UserID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse user ID")
			continue
		}

		dbCredIDUUID, err := uuid.Parse(idx.DbCredentialID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse DB credential ID")
			continue
		}

		var lastIndexedAt *time.Time
		if idx.LastIndexedAt.Valid {
			t := idx.LastIndexedAt.Time
			lastIndexedAt = &t
		}

		response[i] = models.IndexerResponse{
			ID:             idUUID,
			UserID:         userIDUUID,
			DBCredentialID: dbCredIDUUID,
			IndexerType:    models.IndexerType(idx.IndexerType),
			Params:         params,
			TargetTable:    idx.TargetTable,
			WebhookID:      idx.WebhookID.String,
			Status:         models.IndexerStatus(idx.Status),
			LastIndexedAt:  lastIndexedAt,
			ErrorMessage:   idx.ErrorMessage.String,
			CreatedAt:      idx.CreatedAt.Time,
			UpdatedAt:      idx.UpdatedAt.Time,
		}
	}

	return response, nil
}

func (s *IndexerService) GetIndexerByID(ctx context.Context, userID uuid.UUID, indexerID uuid.UUID) (*models.IndexerResponse, error) {

	var pgIndexerID pgtype.UUID
	if err := pgIndexerID.Scan(indexerID.String()); err != nil {
		return nil, fmt.Errorf("invalid indexer ID: %w", err)
	}

	foundIndexer, err := s.store.GetIndexerByID(ctx, pgIndexerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexer")
		return nil, errors.New("indexer not found")
	}

	userIDFromDB, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil || userIDFromDB != userID {
		return nil, errors.New("indexer not found")
	}

	var params interface{}
	if err := json.Unmarshal(foundIndexer.Params, &params); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal indexer params")
		params = map[string]interface{}{}
	}

	idUUID, err := uuid.Parse(foundIndexer.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse indexer ID")
		return nil, errors.New("internal server error")
	}

	userIDUUID, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		return nil, errors.New("internal server error")
	}

	dbCredIDUUID, err := uuid.Parse(foundIndexer.DbCredentialID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse DB credential ID")
		return nil, errors.New("internal server error")
	}

	var lastIndexedAt *time.Time
	if foundIndexer.LastIndexedAt.Valid {
		t := foundIndexer.LastIndexedAt.Time
		lastIndexedAt = &t
	}

	return &models.IndexerResponse{
		ID:             idUUID,
		UserID:         userIDUUID,
		DBCredentialID: dbCredIDUUID,
		IndexerType:    models.IndexerType(foundIndexer.IndexerType),
		Params:         params,
		TargetTable:    foundIndexer.TargetTable,
		WebhookID:      foundIndexer.WebhookID.String,
		Status:         models.IndexerStatus(foundIndexer.Status),
		LastIndexedAt:  lastIndexedAt,
		ErrorMessage:   foundIndexer.ErrorMessage.String,
		CreatedAt:      foundIndexer.CreatedAt.Time,
		UpdatedAt:      foundIndexer.UpdatedAt.Time,
	}, nil
}

func (s *IndexerService) PauseIndexer(ctx context.Context, userID uuid.UUID, indexerID uuid.UUID) (*models.IndexerResponse, error) {

	var pgIndexerID pgtype.UUID
	if err := pgIndexerID.Scan(indexerID.String()); err != nil {
		return nil, fmt.Errorf("invalid indexer ID: %w", err)
	}

	foundIndexer, err := s.store.GetIndexerByID(ctx, pgIndexerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexer")
		return nil, errors.New("indexer not found")
	}

	userIDFromDB, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil || userIDFromDB != userID {
		return nil, errors.New("indexer not found")
	}

	if foundIndexer.Status == db.IndexerStatusPaused || foundIndexer.Status == db.IndexerStatusFailed {
		return nil, fmt.Errorf("indexer is already %s", foundIndexer.Status)
	}

	var emptyText pgtype.Text
	emptyText.Valid = false

	foundIndexer, err = s.store.UpdateIndexerStatus(ctx, db.UpdateIndexerStatusParams{
		ID:           pgIndexerID,
		Status:       db.IndexerStatusPaused,
		ErrorMessage: emptyText,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to update indexer status")
		return nil, errors.New("failed to pause indexer")
	}

	var params interface{}
	if err := json.Unmarshal(foundIndexer.Params, &params); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal indexer params")
		params = map[string]interface{}{}
	}

	idUUID, err := uuid.Parse(foundIndexer.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse indexer ID")
		return nil, errors.New("internal server error")
	}

	userIDUUID, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		return nil, errors.New("internal server error")
	}

	dbCredIDUUID, err := uuid.Parse(foundIndexer.DbCredentialID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse DB credential ID")
		return nil, errors.New("internal server error")
	}

	var lastIndexedAt *time.Time
	if foundIndexer.LastIndexedAt.Valid {
		t := foundIndexer.LastIndexedAt.Time
		lastIndexedAt = &t
	}

	return &models.IndexerResponse{
		ID:             idUUID,
		UserID:         userIDUUID,
		DBCredentialID: dbCredIDUUID,
		IndexerType:    models.IndexerType(foundIndexer.IndexerType),
		Params:         params,
		TargetTable:    foundIndexer.TargetTable,
		WebhookID:      foundIndexer.WebhookID.String,
		Status:         models.IndexerStatus(foundIndexer.Status),
		LastIndexedAt:  lastIndexedAt,
		ErrorMessage:   foundIndexer.ErrorMessage.String,
		CreatedAt:      foundIndexer.CreatedAt.Time,
		UpdatedAt:      foundIndexer.UpdatedAt.Time,
	}, nil
}

func (s *IndexerService) ResumeIndexer(ctx context.Context, userID uuid.UUID, indexerID uuid.UUID) (*models.IndexerResponse, error) {

	var pgIndexerID pgtype.UUID
	if err := pgIndexerID.Scan(indexerID.String()); err != nil {
		return nil, fmt.Errorf("invalid indexer ID: %w", err)
	}

	foundIndexer, err := s.store.GetIndexerByID(ctx, pgIndexerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexer")
		return nil, errors.New("indexer not found")
	}

	userIDFromDB, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil || userIDFromDB != userID {
		return nil, errors.New("indexer not found")
	}

	if foundIndexer.Status == db.IndexerStatusActive {
		return nil, errors.New("indexer is already active")
	}

	var emptyText pgtype.Text
	emptyText.Valid = false

	foundIndexer, err = s.store.UpdateIndexerStatus(ctx, db.UpdateIndexerStatusParams{
		ID:           pgIndexerID,
		Status:       db.IndexerStatusActive,
		ErrorMessage: emptyText,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to update indexer status")
		return nil, errors.New("failed to resume indexer")
	}

	var params interface{}
	if err := json.Unmarshal(foundIndexer.Params, &params); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal indexer params")
		params = map[string]interface{}{}
	}

	idUUID, err := uuid.Parse(foundIndexer.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse indexer ID")
		return nil, errors.New("internal server error")
	}

	userIDUUID, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		return nil, errors.New("internal server error")
	}

	dbCredIDUUID, err := uuid.Parse(foundIndexer.DbCredentialID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse DB credential ID")
		return nil, errors.New("internal server error")
	}

	var lastIndexedAt *time.Time
	if foundIndexer.LastIndexedAt.Valid {
		t := foundIndexer.LastIndexedAt.Time
		lastIndexedAt = &t
	}

	return &models.IndexerResponse{
		ID:             idUUID,
		UserID:         userIDUUID,
		DBCredentialID: dbCredIDUUID,
		IndexerType:    models.IndexerType(foundIndexer.IndexerType),
		Params:         params,
		TargetTable:    foundIndexer.TargetTable,
		WebhookID:      foundIndexer.WebhookID.String,
		Status:         models.IndexerStatus(foundIndexer.Status),
		LastIndexedAt:  lastIndexedAt,
		ErrorMessage:   foundIndexer.ErrorMessage.String,
		CreatedAt:      foundIndexer.CreatedAt.Time,
		UpdatedAt:      foundIndexer.UpdatedAt.Time,
	}, nil
}

func (s *IndexerService) DeleteIndexer(ctx context.Context, userID uuid.UUID, indexerID uuid.UUID) error {

	var pgIndexerID pgtype.UUID
	if err := pgIndexerID.Scan(indexerID.String()); err != nil {
		return fmt.Errorf("invalid indexer ID: %w", err)
	}

	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(userID.String()); err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	foundIndexer, err := s.store.GetIndexerByID(ctx, pgIndexerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexer")
		return errors.New("indexer not found")
	}

	userIDFromDB, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil || userIDFromDB != userID {
		return errors.New("indexer not found")
	}

	if s.heliusClient != nil && foundIndexer.WebhookID.Valid && foundIndexer.WebhookID.String != "" {

		indexerID := foundIndexer.ID.String()

		var heliusWebhookID string
		for helID, idxID := range indexer.GetAllWebhookMappings() {
			if idxID == indexerID {
				heliusWebhookID = helID
				break
			}
		}

		if heliusWebhookID != "" {
			log.Info().
				Str("indexerID", indexerID).
				Str("heliusWebhookID", heliusWebhookID).
				Msg("Deleting Helius webhook for indexer")

			if err := s.heliusClient.DeleteWebhook(ctx, heliusWebhookID); err != nil {
				log.Error().
					Err(err).
					Str("indexerID", indexerID).
					Str("heliusWebhookID", heliusWebhookID).
					Msg("Failed to delete Helius webhook")

			} else {
				log.Info().
					Str("indexerID", indexerID).
					Str("heliusWebhookID", heliusWebhookID).
					Msg("Successfully deleted Helius webhook")
			}
		} else {
			log.Warn().
				Str("indexerID", indexerID).
				Msg("Could not find Helius webhook ID for indexer")
		}
	}

	err = s.store.DeleteIndexer(ctx, db.DeleteIndexerParams{
		ID:     pgIndexerID,
		UserID: pgUserID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete indexer")
		return errors.New("failed to delete indexer")
	}

	idUUID, err := uuid.Parse(foundIndexer.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse indexer ID")

	} else {

		delete(s.indexers, idUUID)
	}

	return nil
}

func (s *IndexerService) GetIndexingLogs(ctx context.Context, userID uuid.UUID, indexerID uuid.UUID, limit int32, offset int32) ([]models.IndexingLogResponse, error) {

	var pgIndexerID pgtype.UUID
	if err := pgIndexerID.Scan(indexerID.String()); err != nil {
		return nil, fmt.Errorf("invalid indexer ID: %w", err)
	}

	foundIndexer, err := s.store.GetIndexerByID(ctx, pgIndexerID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexer")
		return nil, errors.New("indexer not found")
	}

	userIDFromDB, err := uuid.Parse(foundIndexer.UserID.String())
	if err != nil || userIDFromDB != userID {
		return nil, errors.New("indexer not found")
	}

	logs, err := s.store.GetIndexingLogsByIndexerID(ctx, db.GetIndexingLogsByIndexerIDParams{
		IndexerID: pgIndexerID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexing logs")
		return nil, errors.New("failed to retrieve indexing logs")
	}

	response := make([]models.IndexingLogResponse, len(logs))
	for i, l := range logs {
		var details interface{}
		if l.Details != nil {
			if err := json.Unmarshal(l.Details, &details); err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal log details")
				details = map[string]interface{}{}
			}
		}

		idUUID, err := uuid.Parse(l.ID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse log ID")
			continue
		}

		indexerIDUUID, err := uuid.Parse(l.IndexerID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse indexer ID in log")
			continue
		}

		response[i] = models.IndexingLogResponse{
			ID:        idUUID,
			IndexerID: indexerIDUUID,
			EventType: l.EventType,
			Message:   l.Message,
			Details:   details,
			CreatedAt: l.CreatedAt.Time,
		}
	}

	return response, nil
}

func (s *IndexerService) ProcessWebhookPayload(ctx context.Context, webhookID string, payload models.HeliusWebhookPayload) error {

	var pgWebhookID pgtype.Text
	pgWebhookID.String = webhookID
	pgWebhookID.Valid = true

	log.Info().
		Str("webhookID", webhookID).
		Int64("slot", payload.Slot).
		Msg("Processing webhook payload")

	var foundIndexer db.Indexer
	var err error

	for attempt := 1; attempt <= 3; attempt++ {
		foundIndexer, err = s.store.GetIndexerByWebhookID(ctx, pgWebhookID)
		if err == nil {
			break
		}

		log.Warn().
			Err(err).
			Str("webhookID", webhookID).
			Int("attempt", attempt).
			Msg("Failed to get indexer by webhook ID, retrying...")

		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("indexer not found for webhook ID %s: %w", webhookID, err)
	}

	if foundIndexer.Status != db.IndexerStatusActive {
		return fmt.Errorf("indexer is not active (status: %s)", foundIndexer.Status)
	}

	var cred db.DbCredential

	for attempt := 1; attempt <= 3; attempt++ {
		cred, err = s.store.GetDBCredentialByID(ctx, foundIndexer.DbCredentialID)
		if err == nil {
			break
		}

		log.Warn().
			Err(err).
			Str("credentialID", foundIndexer.DbCredentialID.String()).
			Int("attempt", attempt).
			Msg("Failed to get database credential, retrying...")

		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("database credential not found: %w", err)
	}

	idxImpl, err := s.getOrCreateIndexerImpl(ctx, foundIndexer)
	if err != nil {
		return fmt.Errorf("failed to create indexer implementation: %w", err)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cred.DbHost, cred.DbPort, cred.DbUser, cred.DbPassword, cred.DbName, cred.DbSslMode)

	log.Debug().
		Str("host", cred.DbHost).
		Int32("port", cred.DbPort).
		Str("database", cred.DbName).
		Str("user", cred.DbUser).
		Msg("Connecting to user database")

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if tokenIndexer, ok := idxImpl.(indexer.TokenIndexer); ok && s.heliusAPIKey != "" {

		if err := tokenIndexer.EnrichTokenMetadata(ctx, pool, foundIndexer.TargetTable, s.heliusAPIKey); err != nil {
			log.Warn().Err(err).Msg("Failed to enrich token metadata")

		}

		if err := tokenIndexer.ProcessPayloadWithMetadata(ctx, pool, foundIndexer.TargetTable, payload, s.heliusAPIKey); err != nil {

			log.Error().Err(err).
				Str("indexerID", foundIndexer.ID.String()).
				Str("webhookID", webhookID).
				Msg("Failed to process webhook payload")

			details, _ := json.Marshal(map[string]interface{}{
				"error": err.Error(),
				"slot":  payload.Slot,
			})

			_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
				IndexerID: foundIndexer.ID,
				EventType: "error",
				Message:   fmt.Sprintf("Failed to process payload: %s", err.Error()),
				Details:   details,
			})
			if logErr != nil {
				log.Error().Err(logErr).Msg("Failed to create error log entry")
			}

			return err
		}
	} else {

		if err := idxImpl.ProcessPayload(ctx, pool, foundIndexer.TargetTable, payload); err != nil {

			log.Error().Err(err).
				Str("indexerID", foundIndexer.ID.String()).
				Str("webhookID", webhookID).
				Msg("Failed to process webhook payload")

			details, _ := json.Marshal(map[string]interface{}{
				"error": err.Error(),
				"slot":  payload.Slot,
			})

			_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
				IndexerID: foundIndexer.ID,
				EventType: "error",
				Message:   fmt.Sprintf("Failed to process payload: %s", err.Error()),
				Details:   details,
			})
			if logErr != nil {
				log.Error().Err(logErr).Msg("Failed to create error log entry")
			}

			return err
		}
	}

	_, err = s.store.UpdateLastIndexedTime(ctx, foundIndexer.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update last indexed time")

	}

	details, _ := json.Marshal(map[string]interface{}{
		"slot": payload.Slot,
	})

	_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
		IndexerID: foundIndexer.ID,
		EventType: "success",
		Message:   "Successfully processed webhook payload",
		Details:   details,
	})
	if logErr != nil {
		log.Error().Err(logErr).Msg("Failed to create success log entry")

	}

	log.Info().
		Str("webhookID", webhookID).
		Int64("slot", payload.Slot).
		Msg("Successfully processed webhook payload")

	return nil
}

func (s *IndexerService) initializeIndexer(ctx context.Context, dbIndexer db.Indexer) error {

	cred, err := s.store.GetDBCredentialByID(ctx, dbIndexer.DbCredentialID)
	if err != nil {
		return fmt.Errorf("database credential not found: %w", err)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cred.DbHost, cred.DbPort, cred.DbUser, cred.DbPassword, cred.DbName, cred.DbSslMode)

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	idxImpl, err := s.getOrCreateIndexerImpl(ctx, dbIndexer)
	if err != nil {
		return fmt.Errorf("failed to create indexer implementation: %w", err)
	}

	if tokenIndexer, ok := idxImpl.(indexer.TokenIndexer); ok && s.heliusAPIKey != "" {

		if err := tokenIndexer.InitializeWithAPIKey(ctx, conn, dbIndexer.TargetTable, s.heliusAPIKey); err != nil {
			return fmt.Errorf("failed to initialize token indexer: %w", err)
		}
	} else {

		if err := idxImpl.Initialize(ctx, conn, dbIndexer.TargetTable); err != nil {
			return fmt.Errorf("failed to initialize indexer: %w", err)
		}
	}

	details, _ := json.Marshal(map[string]interface{}{
		"targetTable": dbIndexer.TargetTable,
	})

	_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
		IndexerID: dbIndexer.ID,
		EventType: "initialization",
		Message:   "Indexer initialized successfully",
		Details:   details,
	})
	if logErr != nil {
		log.Error().Err(logErr).Msg("Failed to create initialization log entry")

	}

	return nil
}

func (s *IndexerService) createHeliusWebhook(ctx context.Context, dbIndexer db.Indexer, addresses []string) (string, error) {
	log.Info().
		Str("indexerID", dbIndexer.ID.String()).
		Strs("addresses", addresses).
		Msg("Creating dedicated Helius webhook for indexer")

	webhookURL := ""
	if s.heliusClient != nil && s.heliusClient.GetWebhookBaseURL() != "" {
		baseURL := s.heliusClient.GetWebhookBaseURL()

		baseURL = strings.TrimSuffix(baseURL, "/")

		webhookURL = fmt.Sprintf("%s/webhooks?id=%s&key=%s",
			baseURL,
			dbIndexer.ID.String(),
			s.heliusClient.GetWebhookSecret())
	} else {
		return "", fmt.Errorf("webhook base URL is not configured")
	}

	config := indexer.WebhookConfig{
		WebhookURL:       webhookURL,
		WebhookType:      "enhanced",
		AccountAddresses: addresses,
		TransactionTypes: []string{"ANY"},
	}

	webhook, err := s.heliusClient.CreateWebhook(ctx, config)
	if err != nil {
		return "", fmt.Errorf("failed to create Helius webhook: %w", err)
	}

	indexer.RegisterWebhookMapping(webhook.WebhookID, dbIndexer.ID.String())

	details, _ := json.Marshal(map[string]interface{}{
		"heliusWebhookID": webhook.WebhookID,
		"indexerID":       dbIndexer.ID.String(),
		"endpoint":        webhook.Endpoint,
		"addresses":       addresses,
	})

	_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
		IndexerID: dbIndexer.ID,
		EventType: "webhook_creation",
		Message:   "Created dedicated Helius webhook for indexer",
		Details:   details,
	})
	if logErr != nil {
		log.Error().Err(logErr).Msg("Failed to create webhook creation log entry")

	}

	log.Info().
		Str("indexerID", dbIndexer.ID.String()).
		Str("webhookID", webhook.WebhookID).
		Str("endpoint", webhook.Endpoint).
		Msg("Successfully created dedicated webhook")

	return webhook.WebhookID, nil
}

func (s *IndexerService) GetIndexerByWebhookIDForDebug(ctx context.Context, webhookID string) (interface{}, error) {
	var pgWebhookID pgtype.Text
	pgWebhookID.String = webhookID
	pgWebhookID.Valid = true

	foundIndexer, err := s.store.GetIndexerByWebhookID(ctx, pgWebhookID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"found":       true,
		"id":          foundIndexer.ID.String,
		"status":      string(foundIndexer.Status),
		"webhookId":   foundIndexer.WebhookID.String,
		"targetTable": foundIndexer.TargetTable,
	}, nil
}

func (s *IndexerService) getOrCreateIndexerImpl(ctx context.Context, dbIndexer db.Indexer) (indexer.Indexer, error) {

	idUUID, err := uuid.Parse(dbIndexer.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse indexer ID: %w", err)
	}

	if idx, ok := s.indexers[idUUID]; ok {
		return idx, nil
	}

	var idxImpl indexer.Indexer

	switch dbIndexer.IndexerType {
	case db.IndexerTypeNftBids:
		idxImpl, err = indexer.NewNFTBidIndexer(dbIndexer.ID.String(), dbIndexer.Params)
	case db.IndexerTypeNftPrices:
		idxImpl, err = indexer.NewNFTPriceIndexer(dbIndexer.ID.String(), dbIndexer.Params)
	case db.IndexerTypeTokenBorrow:
		idxImpl, err = indexer.NewTokenBorrowIndexer(dbIndexer.ID.String(), dbIndexer.Params)
	case db.IndexerTypeTokenPrices:
		idxImpl, err = indexer.NewTokenPriceIndexer(dbIndexer.ID.String(), dbIndexer.Params)
	default:
		return nil, fmt.Errorf("unsupported indexer type: %s", dbIndexer.IndexerType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create indexer implementation: %w", err)
	}

	s.indexers[idUUID] = idxImpl

	return idxImpl, nil
}
