package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/config"
)

type MiddlewareConfig struct {
	Auth gin.HandlerFunc
}

func NewMiddlewareConfig(jwtConfig config.JWTConfig) MiddlewareConfig {
	return MiddlewareConfig{
		Auth: AuthMiddleware(jwtConfig),
	}
}

// AuthMiddleware is a middleware to verify JWT tokens
func AuthMiddleware(cfg config.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
			return
		}

		tokenString := parts[1]
		claims := &jwt.RegisteredClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(cfg.Secret), nil
		})

		if err != nil {
			log.Error().Err(err).Str("token", tokenString).Msg("Failed to parse token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			log.Error().Err(err).Str("subject", claims.Subject).Msg("Invalid user ID in token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token subject"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// GetUserID extracts the user ID from the context
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, errors.New("user ID not found in context")
	}

	id, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("user ID is not a valid UUID")
	}

	return id, nil
}
