package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/config"
	db "github.com/rishavmehra/indexer/internal/db/generated"
	"github.com/rishavmehra/indexer/internal/models"
	"github.com/rishavmehra/indexer/pkg/crypto"
	"github.com/rishavmehra/indexer/pkg/validator"
)

type AuthService struct {
	cfg   config.JWTConfig
	store db.Querier
}

func NewAuthService(cfg config.JWTConfig, store db.Querier) *AuthService {
	return &AuthService{
		cfg:   cfg,
		store: store,
	}
}

func (s *AuthService) SignUp(ctx context.Context, req models.SignupRequest) (*models.UserResponse, error) {

	if !validator.IsValidEmail(req.Email) {
		return nil, errors.New("invalid email format")
	}

	if !validator.IsValidPassword(req.Password) {
		return nil, errors.New("password must be at least 8 characters and contain letters and numbers")
	}

	existingUser, err := s.store.GetUserByEmail(ctx, req.Email)
	if err == nil {

		var emptyUUID pgtype.UUID
		if existingUser.ID != emptyUUID {
			return nil, errors.New("user with this email already exists")
		}
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return nil, errors.New("failed to process password")
	}

	user, err := s.store.CreateUser(ctx, db.CreateUserParams{
		Email:        req.Email,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		return nil, errors.New("failed to create user")
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

func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.TokenResponse, error) {

	user, err := s.store.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	match, err := crypto.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !match {
		return nil, errors.New("invalid email or password")
	}

	id, err := uuid.Parse(user.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse UUID")
		return nil, errors.New("authentication failed")
	}

	expiresAt := time.Now().Add(s.cfg.ExpiresIn)
	token, err := s.generateToken(id, expiresAt)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		return nil, errors.New("authentication failed")
	}

	return &models.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.UserResponse, error) {

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

func (s *AuthService) generateToken(userID uuid.UUID, expiresAt time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
