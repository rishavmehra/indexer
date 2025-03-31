package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/api/middleware"
	"github.com/rishavmehra/indexer/internal/models"
	"github.com/rishavmehra/indexer/internal/service"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userService *service.UserService
	authService *service.AuthService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// RegisterRoutes registers the routes for the user handler
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup, mw middleware.MiddlewareConfig) {
	users := router.Group("/users")
	users.Use(mw.Auth)
	{
		users.GET("/me", h.GetCurrentUser)
		dbCreds := users.Group("/db-credentials")
		{
			dbCreds.GET("", h.GetDBCredentials)
			dbCreds.POST("", h.CreateDBCredential)
			dbCreds.POST("/test", h.TestDBConnection)
			dbCreds.GET("/:id", h.GetDBCredentialByID)
			dbCreds.PUT("/:id", h.UpdateDBCredential)
			dbCreds.DELETE("/:id", h.DeleteDBCredential)
		}
	}
}

// GetCurrentUser returns the current authenticated user
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetDBCredentials returns all database credentials for the user
func (h *UserHandler) GetDBCredentials(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	credentials, err := h.userService.GetDBCredentials(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, credentials)
}

// CreateDBCredential creates a new database credential
func (h *UserHandler) CreateDBCredential(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.DBCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	credential, err := h.userService.CreateDBCredential(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, credential)
}

// GetDBCredentialByID returns a database credential by ID
func (h *UserHandler) GetDBCredentialByID(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	credID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	credential, err := h.userService.GetDBCredentialByID(c.Request.Context(), userID, credID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, credential)
}

// UpdateDBCredential updates a database credential
func (h *UserHandler) UpdateDBCredential(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	credID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	var req models.DBCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	credential, err := h.userService.UpdateDBCredential(c.Request.Context(), userID, credID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, credential)
}

// DeleteDBCredential deletes a database credential
func (h *UserHandler) DeleteDBCredential(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	credID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid credential ID"})
		return
	}

	if err := h.userService.DeleteDBCredential(c.Request.Context(), userID, credID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) TestDBConnection(c *gin.Context) {
	var req models.DBCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Add logging to help debug
	log.Info().
		Str("host", req.Host).
		Int("port", req.Port).
		Str("dbname", req.Name).
		Str("user", req.User).
		Str("sslMode", req.SSLMode).
		Msg("Testing database connection")

	// Add timeout for the connection test
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.userService.TestDBConnection(ctx, req); err != nil {
		log.Error().Err(err).Msg("Database connection test failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "success": false})
		return
	}

	log.Info().Msg("Database connection test successful")
	c.JSON(http.StatusOK, gin.H{"message": "Connection successful", "success": true})
}
