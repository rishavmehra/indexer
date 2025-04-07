package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/models"
)

type WebhookHandler struct {
	heliusClient *HeliusClient
	processor    WebhookProcessor
}

type WebhookProcessor interface {
	ProcessWebhookPayload(ctx context.Context, webhookID string, payload models.HeliusWebhookPayload) error
}

var heliusWebhookMapping = make(map[string]string)

func RegisterWebhookMapping(heliusWebhookID string, indexerID string) {
	heliusWebhookMapping[heliusWebhookID] = indexerID
	log.Info().
		Str("heliusWebhookID", heliusWebhookID).
		Str("indexerID", indexerID).
		Msg("Registered webhook ID mapping")
}

func GetIndexerIDFromHeliusWebhookID(heliusWebhookID string) (string, bool) {
	indexerID, found := heliusWebhookMapping[heliusWebhookID]
	return indexerID, found
}

func NewWebhookHandler(heliusClient *HeliusClient, processor WebhookProcessor) *WebhookHandler {
	return &WebhookHandler{
		heliusClient: heliusClient,
		processor:    processor,
	}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	log.Info().Str("payload", string(bodyBytes)).Msg("Raw webhook payload")

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	webhookID := r.URL.Query().Get("id")
	if webhookID == "" {

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 2 {
			webhookID = parts[len(parts)-1]
		}
	}

	if webhookID == "" {
		log.Error().Msg("Missing webhook ID in request")
		http.Error(w, "Missing webhook ID", http.StatusBadRequest)
		return
	}

	webhookSecret := r.URL.Query().Get("key")
	if webhookSecret == "" {

		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			webhookSecret = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	log.Debug().Str("rawPayload", string(body)).Msg("Received webhook payload")

	if h.heliusClient != nil && webhookSecret != "" {
		if !h.heliusClient.VerifyWebhookSignature(webhookSecret, string(body)) {
			log.Error().Msg("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload models.HeliusWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Error().Err(err).Msg("Failed to parse webhook payload")
		http.Error(w, "Invalid payload format", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("webhookID", webhookID).
		Int64("slot", payload.Slot).
		Int("accountCount", len(payload.AccountData)).
		Msg("Received webhook")

	go func() {
		ctx := context.Background()
		if err := h.processor.ProcessWebhookPayload(ctx, webhookID, payload); err != nil {
			log.Error().Err(err).
				Str("webhookID", webhookID).
				Int64("slot", payload.Slot).
				Msg("Failed to process webhook payload")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func GetAllWebhookMappings() map[string]string {
	result := make(map[string]string)
	for k, v := range heliusWebhookMapping {
		result[k] = v
	}
	return result
}
