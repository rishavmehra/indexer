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

	var targetPool *pgxpool.Pool
	if len(logs) > 0 {
		var pgCredID pgtype.UUID
		if err := pgCredID.Scan(foundIndexer.DbCredentialID.String()); err != nil {
			log.Error().Err(err).Msg("Failed to parse DB credential ID")
		} else {
			cred, err := s.store.GetDBCredentialByID(ctx, pgCredID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get DB credential")
			} else {
				dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
					cred.DbHost, cred.DbPort, cred.DbUser, cred.DbPassword, cred.DbName, cred.DbSslMode)

				poolConfig, connErr := pgxpool.ParseConfig(dsn)
				if connErr != nil {
					log.Error().Err(connErr).Msg("Failed to parse database config")
				} else {
					poolConfig.MaxConns = 5
					poolConfig.MinConns = 1
					targetPool, connErr = pgxpool.NewWithConfig(ctx, poolConfig)
					if connErr != nil {
						log.Error().Err(connErr).Msg("Failed to connect to target database")
					} else {
						defer targetPool.Close()
					}
				}
			}
		}
	}

	for i, l := range logs {
		var details interface{}
		if l.Details != nil {
			if err := json.Unmarshal(l.Details, &details); err != nil {
				log.Error().Err(err).Msg("Failed to unmarshal log details")
				details = map[string]interface{}{}
			}
		} else {
			details = map[string]interface{}{}
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

		// Enhance details with target DB data for success and token_data events
		if (l.EventType == "success" || l.EventType == "token_data") && targetPool != nil {
			// If the log doesn't already have detailed token data and this is a token price indexer
			if foundIndexer.IndexerType == db.IndexerTypeTokenPrices {
				detailsMap, ok := details.(map[string]interface{})
				if ok && detailsMap["token_data"] == nil && detailsMap["tokens"] == nil {
					// For token price indexers, if we don't already have token data, try to fetch it
					enhancedDetails, err := enhanceTokenPriceDetails(ctx, targetPool, foundIndexer.TargetTable, details)
					if err != nil {
						log.Warn().Err(err).Msg("Failed to enhance token price details")
					} else if enhancedDetails != nil {
						details = enhancedDetails
					}
				}
			} else {
				// For other indexer types, use the original enhancement method
				enhancedDetails, err := enhanceGenericLogDetails(ctx, targetPool, foundIndexer.TargetTable, details, foundIndexer.IndexerType)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to enhance log details with target data")
				} else if enhancedDetails != nil {
					details = enhancedDetails
				}
			}
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

func enhanceTokenPriceDetails(ctx context.Context, pool *pgxpool.Pool, targetTable string, details interface{}) (interface{}, error) {
	detailsMap, ok := details.(map[string]interface{})
	if !ok {
		return details, fmt.Errorf("details is not a map")
	}

	var slot int64
	if slotVal, ok := detailsMap["slot"].(float64); ok {
		slot = int64(slotVal)
	}

	targetTable = formatTableName(targetTable)

	queryStr := fmt.Sprintf(`
		SELECT 
			token_address, 
			COALESCE(token_name, '') as token_name, 
			COALESCE(token_symbol, '') as token_symbol, 
			platform, 
			price_usd, 
			price_sol, 
			volume_24h,
			market_cap,
			liquidity,
			price_change_24h,
			total_supply,
			transaction_id,
			updated_at, 
			slot
		FROM %s
	`, targetTable)

	var rows pgx.Rows
	var err error
	if slot > 0 {
		queryStr += " WHERE slot <= $1 ORDER BY updated_at DESC LIMIT 5"
		rows, err = pool.Query(ctx, queryStr, slot)
	} else {
		queryStr += " ORDER BY updated_at DESC LIMIT 5"
		rows, err = pool.Query(ctx, queryStr)
	}

	if err != nil {
		return details, fmt.Errorf("failed to query token price data: %w", err)
	}
	defer rows.Close()

	var tokenData []map[string]interface{}
	for rows.Next() {
		var (
			tokenAddress   string
			tokenName      string
			tokenSymbol    string
			platform       string
			priceUSD       float64
			priceSOL       pgtype.Float8
			volume24h      pgtype.Float8
			marketCap      pgtype.Float8
			liquidity      pgtype.Float8
			priceChange24h pgtype.Float8
			totalSupply    pgtype.Float8
			transactionID  pgtype.Text
			updatedAt      time.Time
			rowSlot        int64
		)

		if err := rows.Scan(
			&tokenAddress,
			&tokenName,
			&tokenSymbol,
			&platform,
			&priceUSD,
			&priceSOL,
			&volume24h,
			&marketCap,
			&liquidity,
			&priceChange24h,
			&totalSupply,
			&transactionID,
			&updatedAt,
			&rowSlot,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan token row")
			continue
		}

		token := map[string]interface{}{
			"token_address": tokenAddress,
			"token_name":    tokenName,
			"token_symbol":  tokenSymbol,
			"platform":      platform,
			"price_usd":     priceUSD,
			"updated_at":    updatedAt.Format(time.RFC3339),
			"slot":          rowSlot,
		}

		if priceSOL.Valid {
			token["price_sol"] = priceSOL.Float64
		}
		if volume24h.Valid {
			token["volume_24h"] = volume24h.Float64
		}
		if marketCap.Valid {
			token["market_cap"] = marketCap.Float64
		}
		if liquidity.Valid {
			token["liquidity"] = liquidity.Float64
		}
		if priceChange24h.Valid {
			token["price_change_24h"] = priceChange24h.Float64
		}
		if totalSupply.Valid {
			token["total_supply"] = totalSupply.Float64
		}
		if transactionID.Valid {
			token["transaction_id"] = transactionID.String
		}

		tokenData = append(tokenData, token)
	}

	if len(tokenData) > 0 {
		detailsMap["tokens"] = tokenData
	}

	return detailsMap, nil
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

func (s *IndexerService) enhanceLogDetailsWithTargetData(ctx context.Context, pool *pgxpool.Pool, targetTable string, details interface{}, indexerType db.IndexerType) (interface{}, error) {
	detailsMap, ok := details.(map[string]interface{})
	if !ok {
		return details, fmt.Errorf("details is not a map")
	}

	slotFloat, slotOk := detailsMap["slot"].(float64)

	// Format the target table name to ensure it's valid
	targetTable = formatTableName(targetTable)

	// Query the target table for data based on indexer type
	var rows pgx.Rows
	var err error

	// Different query based on indexer type
	if indexerType == db.IndexerTypeTokenPrices {
		// Token Price specific query
		if slotOk {
			slot := int64(slotFloat)
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					token_address, token_name, token_symbol, platform,
					price_usd, price_sol, volume_24h, market_cap, liquidity,
					price_change_24h, total_supply, transaction_id, updated_at, slot
				FROM %s
				WHERE slot <= $1
				ORDER BY updated_at DESC
				LIMIT 5
			`, targetTable), slot)
		} else {
			// If no slot, just get recent entries
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					token_address, token_name, token_symbol, platform,
					price_usd, price_sol, volume_24h, market_cap, liquidity,
					price_change_24h, total_supply, transaction_id, updated_at, slot
				FROM %s
				ORDER BY updated_at DESC
				LIMIT 5
			`, targetTable))
		}

		if err != nil {
			return details, fmt.Errorf("failed to query token price table: %w", err)
		}
		defer rows.Close()

		var tokenData []map[string]interface{}
		for rows.Next() {
			var (
				tokenAddress   string
				tokenName      pgtype.Text
				tokenSymbol    pgtype.Text
				platform       string
				priceUSD       float64
				priceSOL       pgtype.Float8
				volume24h      pgtype.Float8
				marketCap      pgtype.Float8
				liquidity      pgtype.Float8
				priceChange24h pgtype.Float8
				totalSupply    pgtype.Float8
				transactionID  pgtype.Text
				updatedAt      time.Time
				slot           int64
			)

			if err := rows.Scan(
				&tokenAddress, &tokenName, &tokenSymbol, &platform,
				&priceUSD, &priceSOL, &volume24h, &marketCap, &liquidity,
				&priceChange24h, &totalSupply, &transactionID, &updatedAt, &slot,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan row from token price table")
				continue
			}

			rowData := map[string]interface{}{
				"token_address": tokenAddress,
				"platform":      platform,
				"price_usd":     priceUSD,
				"updated_at":    updatedAt.Format(time.RFC3339),
				"slot":          slot,
			}

			// Add optional fields if they have values
			if tokenName.Valid {
				rowData["token_name"] = tokenName.String
			}
			if tokenSymbol.Valid {
				rowData["token_symbol"] = tokenSymbol.String
			}
			if priceSOL.Valid {
				rowData["price_sol"] = priceSOL.Float64
			}
			if volume24h.Valid {
				rowData["volume_24h"] = volume24h.Float64
			}
			if marketCap.Valid {
				rowData["market_cap"] = marketCap.Float64
			}
			if liquidity.Valid {
				rowData["liquidity"] = liquidity.Float64
			}
			if priceChange24h.Valid {
				rowData["price_change_24h"] = priceChange24h.Float64
			}
			if totalSupply.Valid {
				rowData["total_supply"] = totalSupply.Float64
			}
			if transactionID.Valid {
				rowData["transaction_id"] = transactionID.String
			}

			tokenData = append(tokenData, rowData)
		}

		if len(tokenData) > 0 {
			detailsMap["tokens"] = tokenData
		}

	} else if indexerType == db.IndexerTypeNftPrices {
		// Original NFT Price query logic
		if slotOk {
			slot := int64(slotFloat)
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					id, signature, slot, block_time, 
					nft_mint, COALESCE(nft_name, '') as nft_name, marketplace, 
					price, currency, usd_value, 
					seller, buyer, status, 
					created_at, updated_at
				FROM %s
				WHERE slot = $1
				ORDER BY created_at DESC
				LIMIT 5
			`, targetTable), slot)
		} else {
			// If no slot, just get recent entries
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					id, signature, slot, block_time, 
					nft_mint, COALESCE(nft_name, '') as nft_name, marketplace, 
					price, currency, usd_value, 
					seller, buyer, status, 
					created_at, updated_at
				FROM %s
				ORDER BY created_at DESC
				LIMIT 5
			`, targetTable))
		}

		if err != nil {
			return details, fmt.Errorf("failed to query target table: %w", err)
		}
		defer rows.Close()

		var targetData []map[string]interface{}
		for rows.Next() {
			var (
				id          string
				signature   string
				slot        int64
				blockTime   time.Time
				nftMint     string
				nftName     string
				marketplace string
				price       float64
				currency    string
				usdValue    pgtype.Float8
				seller      string
				buyer       pgtype.Text
				status      string
				createdAt   time.Time
				updatedAt   time.Time
			)

			if err := rows.Scan(
				&id, &signature, &slot, &blockTime,
				&nftMint, &nftName, &marketplace,
				&price, &currency, &usdValue,
				&seller, &buyer, &status,
				&createdAt, &updatedAt,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan row from target table")
				continue
			}

			rowData := map[string]interface{}{
				"id":          id,
				"signature":   signature,
				"slot":        slot,
				"block_time":  blockTime.Format(time.RFC3339),
				"nft_mint":    nftMint,
				"nft_name":    nftName,
				"marketplace": marketplace,
				"price":       price,
				"currency":    currency,
				"seller":      seller,
				"status":      status,
				"created_at":  createdAt.Format(time.RFC3339),
				"updated_at":  updatedAt.Format(time.RFC3339),
			}

			if usdValue.Valid {
				rowData["usd_value"] = usdValue.Float64
			} else {
				rowData["usd_value"] = nil
			}

			if buyer.Valid {
				rowData["buyer"] = buyer.String
			} else {
				rowData["buyer"] = nil
			}

			targetData = append(targetData, rowData)
		}

		if len(targetData) > 0 {
			detailsMap["transactions"] = targetData
		}
	} else if indexerType == db.IndexerTypeTokenBorrow {
		// For token borrow indexers
		if slotOk {
			slot := int64(slotFloat)
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					token_address, platform, available_amount, borrow_rate, 
					supply_rate, utilization_rate, total_borrowed, total_supplied, 
					updated_at, slot
				FROM %s
				WHERE slot <= $1
				ORDER BY updated_at DESC
				LIMIT 5
			`, targetTable), slot)
		} else {
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					token_address, platform, available_amount, borrow_rate, 
					supply_rate, utilization_rate, total_borrowed, total_supplied, 
					updated_at, slot
				FROM %s
				ORDER BY updated_at DESC
				LIMIT 5
			`, targetTable))
		}

		if err != nil {
			return details, fmt.Errorf("failed to query token borrow table: %w", err)
		}
		defer rows.Close()

		var borrowData []map[string]interface{}
		for rows.Next() {
			var (
				tokenAddress    string
				platform        string
				availableAmount pgtype.Float8
				borrowRate      pgtype.Float8
				supplyRate      pgtype.Float8
				utilizationRate pgtype.Float8
				totalBorrowed   pgtype.Float8
				totalSupplied   pgtype.Float8
				updatedAt       time.Time
				slot            int64
			)

			if err := rows.Scan(
				&tokenAddress, &platform, &availableAmount, &borrowRate,
				&supplyRate, &utilizationRate, &totalBorrowed, &totalSupplied,
				&updatedAt, &slot,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan row from token borrow table")
				continue
			}

			rowData := map[string]interface{}{
				"token_address": tokenAddress,
				"platform":      platform,
				"updated_at":    updatedAt.Format(time.RFC3339),
				"slot":          slot,
			}

			// Add optional fields
			if availableAmount.Valid {
				rowData["available_amount"] = availableAmount.Float64
			}
			if borrowRate.Valid {
				rowData["borrow_rate"] = borrowRate.Float64
			}
			if supplyRate.Valid {
				rowData["supply_rate"] = supplyRate.Float64
			}
			if utilizationRate.Valid {
				rowData["utilization_rate"] = utilizationRate.Float64
			}
			if totalBorrowed.Valid {
				rowData["total_borrowed"] = totalBorrowed.Float64
			}
			if totalSupplied.Valid {
				rowData["total_supplied"] = totalSupplied.Float64
			}

			borrowData = append(borrowData, rowData)
		}

		if len(borrowData) > 0 {
			detailsMap["borrow_data"] = borrowData
		}
	} else if indexerType == db.IndexerTypeNftBids {
		// For NFT bids indexers
		if slotOk {
			slot := int64(slotFloat)
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					id, signature, slot, block_time, 
					nft_mint, auction_house, marketplace, 
					bidder, bid_amount, bid_currency, bid_usd_value, expiry,
					created_at
				FROM %s
				WHERE slot = $1
				ORDER BY created_at DESC
				LIMIT 5
			`, targetTable), slot)
		} else {
			rows, err = pool.Query(ctx, fmt.Sprintf(`
				SELECT 
					id, signature, slot, block_time, 
					nft_mint, auction_house, marketplace, 
					bidder, bid_amount, bid_currency, bid_usd_value, expiry,
					created_at
				FROM %s
				ORDER BY created_at DESC
				LIMIT 5
			`, targetTable))
		}

		if err != nil {
			return details, fmt.Errorf("failed to query NFT bids table: %w", err)
		}
		defer rows.Close()

		var bidsData []map[string]interface{}
		for rows.Next() {
			var (
				id           int
				signature    string
				slot         int64
				blockTime    time.Time
				nftMint      string
				auctionHouse pgtype.Text
				marketplace  string
				bidder       string
				bidAmount    float64
				bidCurrency  string
				bidUsdValue  pgtype.Float8
				expiry       pgtype.Timestamptz
				createdAt    time.Time
			)

			if err := rows.Scan(
				&id, &signature, &slot, &blockTime,
				&nftMint, &auctionHouse, &marketplace,
				&bidder, &bidAmount, &bidCurrency, &bidUsdValue, &expiry,
				&createdAt,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan row from NFT bids table")
				continue
			}

			rowData := map[string]interface{}{
				"signature":    signature,
				"slot":         slot,
				"block_time":   blockTime.Format(time.RFC3339),
				"nft_mint":     nftMint,
				"marketplace":  marketplace,
				"bidder":       bidder,
				"bid_amount":   bidAmount,
				"bid_currency": bidCurrency,
				"created_at":   createdAt.Format(time.RFC3339),
			}

			if auctionHouse.Valid {
				rowData["auction_house"] = auctionHouse.String
			}
			if bidUsdValue.Valid {
				rowData["bid_usd_value"] = bidUsdValue.Float64
			}
			if expiry.Valid {
				rowData["expiry"] = expiry.Time.Format(time.RFC3339)
			}

			bidsData = append(bidsData, rowData)
		}

		if len(bidsData) > 0 {
			detailsMap["bids"] = bidsData
		}
	}

	return detailsMap, nil
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

	// Create enhanced log details
	logData := map[string]interface{}{
		"slot": payload.Slot,
	}

	// Add transaction details if available
	if payload.Transaction.ID != "" {
		logData["transaction_id"] = payload.Transaction.ID
	}

	// Add signatures if available
	if len(payload.Transaction.Signatures) > 0 {
		logData["signatures"] = payload.Transaction.Signatures
	}

	// Create the standard success log
	details, _ := json.Marshal(logData)

	_, logErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
		IndexerID: foundIndexer.ID,
		EventType: "success",
		Message:   "Successfully processed webhook payload",
		Details:   details,
	})
	if logErr != nil {
		log.Error().Err(logErr).Msg("Failed to create success log entry")
	}

	// For token price indexers, create an additional detailed token data log
	if foundIndexer.IndexerType == db.IndexerTypeTokenPrices {
		// Get token data from the database
		tokenData := fetchCurrentTokenData(ctx, pool, foundIndexer.TargetTable)

		if len(tokenData) > 0 {
			tokenDetails, _ := json.Marshal(map[string]interface{}{
				"slot":       payload.Slot,
				"token_data": tokenData,
			})

			_, tokenLogErr := s.store.CreateIndexingLog(ctx, db.CreateIndexingLogParams{
				IndexerID: foundIndexer.ID,
				EventType: "token_data",
				Message:   "Current token price data",
				Details:   tokenDetails,
			})

			if tokenLogErr != nil {
				log.Error().Err(tokenLogErr).Msg("Failed to create token data log entry")
			}
		}
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

func fetchCurrentTokenData(ctx context.Context, pool *pgxpool.Pool, targetTable string) []map[string]interface{} {
	var tokenData []map[string]interface{}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire connection for token data")
		return tokenData
	}
	defer conn.Release()

	// Format table name to ensure it's valid
	targetTable = formatTableName(targetTable)

	// Query the latest token data
	rows, err := conn.Query(ctx, fmt.Sprintf(`
		SELECT 
			token_address, 
			COALESCE(token_name, '') as token_name, 
			COALESCE(token_symbol, '') as token_symbol, 
			platform, 
			price_usd, 
			price_sol, 
			volume_24h,
			market_cap,
			liquidity,
			price_change_24h,
			total_supply,
			transaction_id,
			updated_at, 
			slot
		FROM %s
		ORDER BY updated_at DESC
		LIMIT 5
	`, targetTable))

	if err != nil {
		log.Error().Err(err).Str("table", targetTable).Msg("Failed to query token data")
		return tokenData
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tokenAddress   string
			tokenName      string
			tokenSymbol    string
			platform       string
			priceUSD       float64
			priceSOL       pgtype.Float8
			volume24h      pgtype.Float8
			marketCap      pgtype.Float8
			liquidity      pgtype.Float8
			priceChange24h pgtype.Float8
			totalSupply    pgtype.Float8
			transactionID  pgtype.Text
			updatedAt      time.Time
			slot           int64
		)

		if err := rows.Scan(
			&tokenAddress,
			&tokenName,
			&tokenSymbol,
			&platform,
			&priceUSD,
			&priceSOL,
			&volume24h,
			&marketCap,
			&liquidity,
			&priceChange24h,
			&totalSupply,
			&transactionID,
			&updatedAt,
			&slot,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan token row")
			continue
		}

		// Build token data map with all available fields
		token := map[string]interface{}{
			"token_address": tokenAddress,
			"token_name":    tokenName,
			"token_symbol":  tokenSymbol,
			"platform":      platform,
			"price_usd":     priceUSD,
			"updated_at":    updatedAt.Format(time.RFC3339),
			"slot":          slot,
		}

		// Add optional fields if they have values
		if priceSOL.Valid {
			token["price_sol"] = priceSOL.Float64
		}
		if volume24h.Valid {
			token["volume_24h"] = volume24h.Float64
		}
		if marketCap.Valid {
			token["market_cap"] = marketCap.Float64
		}
		if liquidity.Valid {
			token["liquidity"] = liquidity.Float64
		}
		if priceChange24h.Valid {
			token["price_change_24h"] = priceChange24h.Float64
		}
		if totalSupply.Valid {
			token["total_supply"] = totalSupply.Float64
		}
		if transactionID.Valid {
			token["transaction_id"] = transactionID.String
		}

		tokenData = append(tokenData, token)
	}

	return tokenData
}

func enhanceGenericLogDetails(ctx context.Context, pool *pgxpool.Pool, targetTable string, details interface{}, indexerType db.IndexerType) (interface{}, error) {
	detailsMap, ok := details.(map[string]interface{})
	if !ok {
		return details, fmt.Errorf("details is not a map")
	}

	slotFloat, slotOk := detailsMap["slot"].(float64)

	// Format the target table name to ensure it's valid
	targetTable = formatTableName(targetTable)

	// Query the target table for data
	var rows pgx.Rows
	var err error

	if slotOk {
		slot := int64(slotFloat)
		rows, err = pool.Query(ctx, fmt.Sprintf(`
			SELECT 
				id, signature, slot, block_time, 
				nft_mint, COALESCE(nft_name, '') as nft_name, marketplace, 
				price, currency, usd_value, 
				seller, buyer, status, 
				created_at, updated_at
			FROM %s
			WHERE slot = $1
			ORDER BY created_at DESC
			LIMIT 5
		`, targetTable), slot)
	} else {
		// If no slot, just get recent entries
		rows, err = pool.Query(ctx, fmt.Sprintf(`
			SELECT 
				id, signature, slot, block_time, 
				nft_mint, COALESCE(nft_name, '') as nft_name, marketplace, 
				price, currency, usd_value, 
				seller, buyer, status, 
				created_at, updated_at
			FROM %s
			ORDER BY created_at DESC
			LIMIT 5
		`, targetTable))
	}

	if err != nil {
		return details, fmt.Errorf("failed to query target table: %w", err)
	}
	defer rows.Close()

	var targetData []map[string]interface{}
	for rows.Next() {
		var (
			id          string
			signature   string
			slot        int64
			blockTime   time.Time
			nftMint     string
			nftName     string
			marketplace string
			price       float64
			currency    string
			usdValue    pgtype.Float8
			seller      string
			buyer       pgtype.Text
			status      string
			createdAt   time.Time
			updatedAt   time.Time
		)

		if err := rows.Scan(
			&id, &signature, &slot, &blockTime,
			&nftMint, &nftName, &marketplace,
			&price, &currency, &usdValue,
			&seller, &buyer, &status,
			&createdAt, &updatedAt,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan row from target table")
			continue
		}

		rowData := map[string]interface{}{
			"id":          id,
			"signature":   signature,
			"slot":        slot,
			"block_time":  blockTime.Format(time.RFC3339),
			"nft_mint":    nftMint,
			"nft_name":    nftName,
			"marketplace": marketplace,
			"price":       price,
			"currency":    currency,
			"seller":      seller,
			"status":      status,
			"created_at":  createdAt.Format(time.RFC3339),
			"updated_at":  updatedAt.Format(time.RFC3339),
		}

		if usdValue.Valid {
			rowData["usd_value"] = usdValue.Float64
		} else {
			rowData["usd_value"] = nil
		}

		if buyer.Valid {
			rowData["buyer"] = buyer.String
		} else {
			rowData["buyer"] = nil
		}

		targetData = append(targetData, rowData)
	}

	if len(targetData) > 0 {
		detailsMap["transactions"] = targetData
	}

	return detailsMap, nil
}
