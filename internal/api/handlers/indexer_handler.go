package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/api/middleware"
	"github.com/rishavmehra/indexer/internal/models"
	"github.com/rishavmehra/indexer/internal/service"
)

// IndexerHandler handles indexer-related requests
type IndexerHandler struct {
	indexerService *service.IndexerService
}

// heliusWebhookMapping maintains a mapping of Helius webhook IDs to indexer IDs
var heliusWebhookMapping = make(map[string]string)

// RegisterWebhookMapping adds a mapping between Helius webhook ID and indexer ID
func RegisterWebhookMapping(heliusWebhookID string, indexerID string) {
	heliusWebhookMapping[heliusWebhookID] = indexerID
	log.Info().
		Str("heliusWebhookID", heliusWebhookID).
		Str("indexerID", indexerID).
		Msg("Registered webhook ID mapping")
}

// GetIndexerIDFromHeliusWebhookID retrieves the indexer ID for a given Helius webhook ID
func GetIndexerIDFromHeliusWebhookID(heliusWebhookID string) (string, bool) {
	indexerID, found := heliusWebhookMapping[heliusWebhookID]
	return indexerID, found
}

// NewIndexerHandler creates a new indexer handler
func NewIndexerHandler(indexerService *service.IndexerService) *IndexerHandler {
	return &IndexerHandler{
		indexerService: indexerService,
	}
}

// RegisterRoutes registers the routes for the indexer handler
func (h *IndexerHandler) RegisterRoutes(router *gin.RouterGroup, mw middleware.MiddlewareConfig) {
	indexers := router.Group("/indexers")
	indexers.Use(mw.Auth)
	{
		indexers.GET("", h.GetIndexers)
		indexers.POST("", h.CreateIndexer)
		indexers.GET("/:id", h.GetIndexerByID)
		indexers.POST("/:id/pause", h.PauseIndexer)
		indexers.POST("/:id/resume", h.ResumeIndexer)
		indexers.DELETE("/:id", h.DeleteIndexer)
		indexers.GET("/:id/logs", h.GetIndexerLogs)
		indexers.GET("/debug/webhook/:webhookId", h.DebugWebhookIndexer)
		indexers.POST("/test-process", h.TestProcessWebhook)
	}

}

// RegisterWebhookRoute registers the webhook route
func (h *IndexerHandler) RegisterWebhookRoute(router *gin.Engine) {
	router.POST("/webhooks", h.HandleWebhook)
}

func (h *IndexerHandler) TestProcessWebhook(c *gin.Context) {
	var req struct {
		WebhookID string                      `json:"webhookId"`
		Data      models.HeliusWebhookPayload `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := h.indexerService.ProcessWebhookPayload(ctx, req.WebhookID, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// Add this to the DebugWebhookIndexer function in internal/api/handlers/indexer_handler.go

// DebugWebhookIndexer provides information about an indexer by its webhook ID
func (h *IndexerHandler) DebugWebhookIndexer(c *gin.Context) {
	webhookID := c.Param("webhookId")

	// Log incoming request for debugging
	log.Info().
		Str("webhookIdParam", webhookID).
		Msg("Debug webhook indexer request")

	indexer, err := h.indexerService.GetIndexerByWebhookIDForDebug(c.Request.Context(), webhookID)

	if err != nil {
		if mappedID, found := GetIndexerIDFromHeliusWebhookID(webhookID); found {
			log.Info().
				Str("heliusWebhookID", webhookID).
				Str("mappedIndexerID", mappedID).
				Msg("Found mapped indexer ID for Helius webhook ID")

			indexer, err = h.indexerService.GetIndexerByWebhookIDForDebug(c.Request.Context(), mappedID)
		}
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   fmt.Sprintf("No indexer found for webhook ID: %s", webhookID),
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, indexer)
}

// GetIndexers returns all indexers for the authenticated user
func (h *IndexerHandler) GetIndexers(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	indexers, err := h.indexerService.GetIndexersByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, indexers)
}

// CreateIndexer creates a new indexer
func (h *IndexerHandler) CreateIndexer(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateIndexerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	indexer, err := h.indexerService.CreateIndexer(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, indexer)
}

// GetIndexerByID returns an indexer by ID
func (h *IndexerHandler) GetIndexerByID(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	indexerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid indexer ID"})
		return
	}

	indexer, err := h.indexerService.GetIndexerByID(c.Request.Context(), userID, indexerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, indexer)
}

// PauseIndexer pauses an indexer
func (h *IndexerHandler) PauseIndexer(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	indexerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid indexer ID"})
		return
	}

	indexer, err := h.indexerService.PauseIndexer(c.Request.Context(), userID, indexerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, indexer)
}

// ResumeIndexer resumes a paused indexer
func (h *IndexerHandler) ResumeIndexer(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	indexerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid indexer ID"})
		return
	}

	indexer, err := h.indexerService.ResumeIndexer(c.Request.Context(), userID, indexerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, indexer)
}

// DeleteIndexer deletes an indexer
func (h *IndexerHandler) DeleteIndexer(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	indexerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid indexer ID"})
		return
	}

	if err := h.indexerService.DeleteIndexer(c.Request.Context(), userID, indexerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetIndexerLogs returns logs for an indexer
func (h *IndexerHandler) GetIndexerLogs(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	indexerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid indexer ID"})
		return
	}

	limit := int32(100)
	offset := int32(0)

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = int32(l)
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	logs, err := h.indexerService.GetIndexingLogs(c.Request.Context(), userID, indexerID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// HandleWebhook processes webhook requests from Helius
func (h *IndexerHandler) HandleWebhook(c *gin.Context) {
	webhookID := c.Query("id")

	if webhookID == "" {
		parts := strings.Split(c.Request.URL.Path, "/")
		if len(parts) > 2 {
			webhookID = parts[len(parts)-1]
		}
	}

	if webhookID == "" {
		log.Warn().Msg("No webhook ID provided in request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing webhook ID"})
		return
	}

	log.Info().
		Str("webhookID", webhookID).
		Msg("Received webhook request")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}
	defer c.Request.Body.Close()

	log.Debug().Str("rawPayload", string(body)).Msg("Received webhook payload")

	if len(body) > 0 && body[0] == '[' {
		var transactions []json.RawMessage
		if err := json.Unmarshal(body, &transactions); err != nil {
			log.Error().Err(err).Msg("Failed to parse transaction array")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid array format"})
			return
		}

		for _, txData := range transactions {
			var tx map[string]interface{}
			if err := json.Unmarshal(txData, &tx); err != nil {
				log.Error().Err(err).Msg("Failed to parse transaction data")
				continue
			}

			slot, _ := tx["slot"].(float64)

			enhancedDetails, _ := json.Marshal(tx)

			payload := models.HeliusWebhookPayload{
				Slot: int64(slot),
				Transaction: models.HeliusTransaction{
					ID:              tx["signature"].(string),
					Signatures:      []string{tx["signature"].(string)},
					EnhancedDetails: enhancedDetails,
				},
			}

			go func(p models.HeliusWebhookPayload) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				if err := h.indexerService.ProcessWebhookPayload(ctx, webhookID, p); err != nil {
					log.Error().Err(err).Str("webhookID", webhookID).Int64("slot", p.Slot).Msg("Failed to process")
				}
			}(payload)
		}
	} else {
		var payload models.HeliusWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Error().Err(err).Msg("Failed to parse webhook payload")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := h.indexerService.ProcessWebhookPayload(ctx, webhookID, payload); err != nil {
				log.Error().Err(err).Str("webhookID", webhookID).Int64("slot", payload.Slot).Msg("Failed to process")
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
