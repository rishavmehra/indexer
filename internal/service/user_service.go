package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	db "github.com/rishavmehra/indexer/internal/db/generated"
	"github.com/rishavmehra/indexer/internal/models"
	"github.com/rishavmehra/indexer/pkg/validator"
)

type UserService struct {
	store db.Querier
}

func NewUserService(store db.Querier) *UserService {
	return &UserService{
		store: store,
	}
}

func (s *UserService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.UserResponse, error) {

	var pgID pgtype.UUID
	if err := pgID.Scan(userID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert UUID")
		return nil, errors.New("invalid user ID")
	}

	user, err := s.store.GetUserByID(ctx, pgID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user")
		return nil, errors.New("user not found")
	}

	id, err := uuid.Parse(user.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse UUID")
		return nil, errors.New("internal server error")
	}

	return &models.UserResponse{
		ID:        id,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Time,
	}, nil
}

func (s *UserService) CreateDBCredential(ctx context.Context, userID uuid.UUID, req models.DBCredentialRequest) (*models.DBCredentialResponse, error) {

	if err := validator.ValidateDBCredentials(req.Host, req.Port, req.Name, req.User, req.Password); err != nil {
		return nil, err
	}

	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(userID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert UUID")
		return nil, errors.New("invalid user ID")
	}

	sslMode := req.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	cred, err := s.store.CreateDBCredential(ctx, db.CreateDBCredentialParams{
		UserID:     pgUserID,
		DbHost:     req.Host,
		DbPort:     int32(req.Port),
		DbName:     req.Name,
		DbUser:     req.User,
		DbPassword: req.Password,
		DbSslMode:  sslMode,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create DB credential")
		return nil, errors.New("failed to create database credential")
	}

	id, err := uuid.Parse(cred.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse credential ID")
		return nil, errors.New("internal server error")
	}

	userIDParsed, err := uuid.Parse(cred.UserID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		return nil, errors.New("internal server error")
	}

	return &models.DBCredentialResponse{
		ID:        id,
		UserID:    userIDParsed,
		Host:      cred.DbHost,
		Port:      int(cred.DbPort),
		Name:      cred.DbName,
		User:      cred.DbUser,
		SSLMode:   cred.DbSslMode,
		CreatedAt: cred.CreatedAt.Time,
		UpdatedAt: cred.UpdatedAt.Time,
	}, nil
}

func (s *UserService) UpdateDBCredential(ctx context.Context, userID uuid.UUID, credID uuid.UUID, req models.DBCredentialRequest) (*models.DBCredentialResponse, error) {

	if err := validator.ValidateDBCredentials(req.Host, req.Port, req.Name, req.User, req.Password); err != nil {
		return nil, err
	}

	var pgCredID pgtype.UUID
	if err := pgCredID.Scan(credID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert credential ID")
		return nil, errors.New("invalid credential ID")
	}

	currentCred, err := s.store.GetDBCredentialByID(ctx, pgCredID)
	if err != nil {
		return nil, errors.New("database credential not found")
	}

	userIDString, err := uuid.Parse(currentCred.UserID.String())
	if err != nil || userIDString != userID {
		return nil, errors.New("database credential not found")
	}

	sslMode := req.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	updatedCred, err := s.store.UpdateDBCredential(ctx, db.UpdateDBCredentialParams{
		ID:         pgCredID,
		DbHost:     req.Host,
		DbPort:     int32(req.Port),
		DbName:     req.Name,
		DbUser:     req.User,
		DbPassword: req.Password,
		DbSslMode:  sslMode,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to update DB credential")
		return nil, errors.New("failed to update database credential")
	}

	id, err := uuid.Parse(updatedCred.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse credential ID")
		return nil, errors.New("internal server error")
	}

	userIDParsed, err := uuid.Parse(updatedCred.UserID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse user ID")
		return nil, errors.New("internal server error")
	}

	return &models.DBCredentialResponse{
		ID:        id,
		UserID:    userIDParsed,
		Host:      updatedCred.DbHost,
		Port:      int(updatedCred.DbPort),
		Name:      updatedCred.DbName,
		User:      updatedCred.DbUser,
		SSLMode:   updatedCred.DbSslMode,
		CreatedAt: updatedCred.CreatedAt.Time,
		UpdatedAt: updatedCred.UpdatedAt.Time,
	}, nil
}

func (s *UserService) GetDBCredentials(ctx context.Context, userID uuid.UUID) ([]models.DBCredentialResponse, error) {

	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(userID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert user ID")
		return nil, errors.New("invalid user ID")
	}

	creds, err := s.store.GetDBCredentialsByUserID(ctx, pgUserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get DB credentials")
		return nil, errors.New("failed to retrieve database credentials")
	}

	response := make([]models.DBCredentialResponse, len(creds))
	for i, cred := range creds {

		id, err := uuid.Parse(cred.ID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse credential ID")
			continue
		}

		userIDParsed, err := uuid.Parse(cred.UserID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse user ID")
			continue
		}

		response[i] = models.DBCredentialResponse{
			ID:        id,
			UserID:    userIDParsed,
			Host:      cred.DbHost,
			Port:      int(cred.DbPort),
			Name:      cred.DbName,
			User:      cred.DbUser,
			SSLMode:   cred.DbSslMode,
			CreatedAt: cred.CreatedAt.Time,
			UpdatedAt: cred.UpdatedAt.Time,
		}
	}

	return response, nil
}

func (s *UserService) GetDBCredentialByID(ctx context.Context, userID uuid.UUID, credID uuid.UUID) (*models.DBCredentialResponse, error) {

	var pgCredID pgtype.UUID
	if err := pgCredID.Scan(credID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert credential ID")
		return nil, errors.New("invalid credential ID")
	}

	cred, err := s.store.GetDBCredentialByID(ctx, pgCredID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get DB credential")
		return nil, errors.New("database credential not found")
	}

	credUserID, err := uuid.Parse(cred.UserID.String())
	if err != nil || credUserID != userID {
		return nil, errors.New("database credential not found")
	}

	id, err := uuid.Parse(cred.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse credential ID")
		return nil, errors.New("internal server error")
	}

	return &models.DBCredentialResponse{
		ID:        id,
		UserID:    credUserID,
		Host:      cred.DbHost,
		Port:      int(cred.DbPort),
		Name:      cred.DbName,
		User:      cred.DbUser,
		SSLMode:   cred.DbSslMode,
		CreatedAt: cred.CreatedAt.Time,
		UpdatedAt: cred.UpdatedAt.Time,
	}, nil
}

func (s *UserService) DeleteDBCredential(ctx context.Context, userID uuid.UUID, credID uuid.UUID) error {

	var pgCredID pgtype.UUID
	if err := pgCredID.Scan(credID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert credential ID")
		return errors.New("invalid credential ID")
	}

	var pgUserID pgtype.UUID
	if err := pgUserID.Scan(userID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to convert user ID")
		return errors.New("invalid user ID")
	}

	cred, err := s.store.GetDBCredentialByID(ctx, pgCredID)
	if err != nil {
		return errors.New("database credential not found")
	}

	credUserID, err := uuid.Parse(cred.UserID.String())
	if err != nil || credUserID != userID {
		return errors.New("database credential not found")
	}

	indexers, err := s.store.GetIndexersByUserID(ctx, pgUserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get indexers")
		return errors.New("failed to verify credential usage")
	}

	for _, indexer := range indexers {
		indexerCredID, err := uuid.Parse(indexer.DbCredentialID.String())
		if err == nil && indexerCredID == credID {
			return errors.New("credential is in use by one or more indexers")
		}
	}

	err = s.store.DeleteDBCredential(ctx, db.DeleteDBCredentialParams{
		ID:     pgCredID,
		UserID: pgUserID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete DB credential")
		return errors.New("failed to delete database credential")
	}

	return nil
}

func (s *UserService) TestDBConnection(ctx context.Context, req models.DBCredentialRequest) error {
	if err := validator.ValidateDBCredentials(req.Host, req.Port, req.Name, req.User, req.Password); err != nil {
		return err
	}

	sslMode := req.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		req.Host, req.Port, req.User, req.Password, req.Name, sslMode)

	// Set up connection config
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close(ctx)

	// Test the connection with a simple query
	var result int
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute test query")
		return fmt.Errorf("query test failed: %w", err)
	}

	if result != 1 {
		return errors.New("unexpected result from test query")
	}

	return nil
}
