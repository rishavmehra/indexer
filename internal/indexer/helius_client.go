package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/models"
)

const (
	HeliusAPIBase     = "https://api.helius.xyz/v0"
	MaxAddressesLimit = 25
)

type WebhookConfig struct {
	WebhookURL                     string              `json:"webhookURL"`
	WebhookType                    string              `json:"webhookType"`
	AccountAddresses               []string            `json:"accountAddresses,omitempty"`
	TransactionTypes               []string            `json:"transactionTypes,omitempty"`
	AccountAddressTransactionTypes map[string][]string `json:"accountAddressTransactionTypes,omitempty"`
	Blocks                         string              `json:"blocks,omitempty"`
}

type AddressEntry struct {
	Address   string    `json:"address"`
	IndexerID string    `json:"indexerId"`
	AddedAt   time.Time `json:"addedAt"`
}

type HeliusClient struct {
	apiKey         string
	webhookSecret  string
	webhookBaseURL string
	webhookID      string
	httpClient     *http.Client
	addresses      []AddressEntry
	addressesLock  sync.RWMutex
}

func NewHeliusClient(apiKey, webhookSecret, webhookBaseURL, webhookID string) *HeliusClient {
	return &HeliusClient{
		apiKey:         apiKey,
		webhookSecret:  webhookSecret,
		webhookBaseURL: webhookBaseURL,
		webhookID:      webhookID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		addresses: []AddressEntry{},
	}
}

func (c *HeliusClient) GetDefaultWebhookID() string {
	return c.webhookID
}

func (c *HeliusClient) GetAPIKey() string {
	return c.apiKey
}

func (c *HeliusClient) AddAddresses(ctx context.Context, addresses []string, indexerID string) error {
	if len(addresses) == 0 {
		log.Info().Msg("No addresses to add")
		return nil
	}

	log.Info().
		Strs("addresses", addresses).
		Str("indexerID", indexerID).
		Str("currentWebhookID", c.webhookID).
		Msg("Adding addresses to webhook")

	c.addressesLock.Lock()
	defer c.addressesLock.Unlock()

	if c.webhookID == "" {
		return c.createWebhookWithAddresses(ctx, addresses, indexerID)
	}

	currentConfig, err := c.GetWebhookConfig(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get current webhook configuration")

		if c.webhookID != "" {
			deleteErr := c.DeleteWebhook(ctx, c.webhookID)
			if deleteErr != nil {
				log.Warn().Err(deleteErr).Msg("Failed to delete existing webhook before creating new one")
			}
			c.webhookID = ""
		}

		return c.createWebhookWithAddresses(ctx, addresses, indexerID)
	}

	newAddresses := []string{}
	for _, addr := range addresses {
		isNew := true
		for _, existing := range currentConfig.AccountAddresses {
			if existing == addr {
				isNew = false
				break
			}
		}
		if isNew {
			newAddresses = append(newAddresses, addr)
		}
	}

	if len(newAddresses) == 0 {
		log.Info().Msg("No new addresses to add to webhook")
		return nil
	}

	log.Info().
		Strs("newAddresses", newAddresses).
		Int("currentAddressCount", len(currentConfig.AccountAddresses)).
		Msg("Found new addresses to add")

	now := time.Now()
	for _, addr := range newAddresses {
		c.addresses = append(c.addresses, AddressEntry{
			Address:   addr,
			IndexerID: indexerID,
			AddedAt:   now,
		})
	}

	if len(currentConfig.AccountAddresses)+len(newAddresses) > MaxAddressesLimit {
		log.Info().
			Int("currentCount", len(currentConfig.AccountAddresses)).
			Int("newCount", len(newAddresses)).
			Int("limit", MaxAddressesLimit).
			Msg("Address limit will be exceeded, removing oldest addresses")

		return c.updateWebhookAddresses(ctx)
	} else {
		allAddresses := append(currentConfig.AccountAddresses, newAddresses...)
		log.Info().
			Int("totalAddressCount", len(allAddresses)).
			Msg("Updating webhook with all addresses")

		return c.updateWebhookWithAddresses(ctx, allAddresses)
	}
}

func (c *HeliusClient) GetAddresses() []AddressEntry {
	c.addressesLock.RLock()
	defer c.addressesLock.RUnlock()

	result := make([]AddressEntry, len(c.addresses))
	copy(result, c.addresses)
	return result
}

func (c *HeliusClient) RemoveIndexerAddresses(ctx context.Context, indexerID string) error {
	c.addressesLock.Lock()
	defer c.addressesLock.Unlock()

	var remainingAddresses []AddressEntry
	addressesToRemove := []string{}

	for _, entry := range c.addresses {
		if entry.IndexerID == indexerID {
			addressesToRemove = append(addressesToRemove, entry.Address)
		} else {
			remainingAddresses = append(remainingAddresses, entry)
		}
	}

	if len(addressesToRemove) == 0 {
		return nil
	}

	c.addresses = remainingAddresses

	currentConfig, err := c.GetWebhookConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current webhook configuration: %w", err)
	}

	var updatedAddresses []string
	for _, addr := range currentConfig.AccountAddresses {
		shouldKeep := true
		for _, removeAddr := range addressesToRemove {
			if addr == removeAddr {
				shouldKeep = false
				break
			}
		}
		if shouldKeep {
			updatedAddresses = append(updatedAddresses, addr)
		}
	}

	return c.updateWebhookWithAddresses(ctx, updatedAddresses)
}

func (c *HeliusClient) GetWebhookBaseURL() string {
	return c.webhookBaseURL
}

func (c *HeliusClient) GetWebhookSecret() string {
	return c.webhookSecret
}

func (c *HeliusClient) CreateWebhook(ctx context.Context, config WebhookConfig) (*models.HeliusWebhookResponse, error) {
	if config.WebhookURL == "" {
		if c.webhookBaseURL == "" {
			return nil, fmt.Errorf("webhook URL is required")
		}

		if c.webhookBaseURL[len(c.webhookBaseURL)-1] == '/' {
			config.WebhookURL = fmt.Sprintf("%swebhooks?key=%s", c.webhookBaseURL, c.webhookSecret)
		} else {
			config.WebhookURL = fmt.Sprintf("%s/webhooks?key=%s", c.webhookBaseURL, c.webhookSecret)
		}
	}

	requestBody, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook config: %w", err)
	}

	log.Debug().
		Str("url", fmt.Sprintf("%s/webhooks?api-key=%s", HeliusAPIBase, c.apiKey)).
		Str("webhookURL", config.WebhookURL).
		Interface("config", config).
		Msg("Creating Helius webhook")

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/webhooks?api-key=%s", HeliusAPIBase, c.apiKey),
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create webhook: %s (status code: %d)", string(body), resp.StatusCode)
	}

	var response models.HeliusWebhookResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (c *HeliusClient) GetWebhookConfig(ctx context.Context) (*WebhookConfig, error) {
	if c.webhookID == "" {
		return nil, fmt.Errorf("no webhook ID configured")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/webhooks/%s?api-key=%s", HeliusAPIBase, c.webhookID, c.apiKey),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Warn().
			Str("webhookID", c.webhookID).
			Int("statusCode", resp.StatusCode).
			Str("responseBody", string(body)).
			Msg("Failed to get webhook configuration")

		// Return an empty config that we can use as a fallback
		return &WebhookConfig{
			WebhookType:      "enhanced",
			AccountAddresses: []string{},
			TransactionTypes: []string{"ANY"},
		}, nil
	}

	log.Debug().
		Str("webhookID", c.webhookID).
		Str("responseBody", string(body)).
		Msg("Got webhook configuration from Helius")

	var config WebhookConfig
	if err := json.Unmarshal(body, &config); err != nil {
		log.Error().Err(err).Str("responseBody", string(body)).Msg("Failed to unmarshal webhook config")

		var rawResponse map[string]interface{}
		if jsonErr := json.Unmarshal(body, &rawResponse); jsonErr == nil {
			fallbackConfig := &WebhookConfig{
				WebhookType:      "enhanced",
				TransactionTypes: []string{"ANY"},
			}

			if accountAddresses, ok := rawResponse["accountAddresses"].([]interface{}); ok {
				addresses := make([]string, 0, len(accountAddresses))
				for _, addr := range accountAddresses {
					if addrStr, ok := addr.(string); ok {
						addresses = append(addresses, addrStr)
					}
				}
				fallbackConfig.AccountAddresses = addresses
			}

			if webhookURL, ok := rawResponse["webhookURL"].(string); ok {
				fallbackConfig.WebhookURL = webhookURL
			}

			return fallbackConfig, nil
		}

		return nil, fmt.Errorf("failed to parse webhook configuration: %w", err)
	}

	return &config, nil
}

func (c *HeliusClient) updateWebhookWithAddresses(ctx context.Context, addresses []string) error {
	if c.webhookID == "" {
		return fmt.Errorf("no webhook ID configured")
	}

	currentConfig, err := c.GetWebhookConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current webhook configuration: %w", err)
	}

	webhookURL := currentConfig.WebhookURL
	if webhookURL == "" {
		if c.webhookBaseURL == "" {
			return fmt.Errorf("webhook base URL is required for update")
		}

		if c.webhookBaseURL[len(c.webhookBaseURL)-1] == '/' {
			webhookURL = fmt.Sprintf("%swebhooks?key=%s", c.webhookBaseURL, c.webhookSecret)
		} else {
			webhookURL = fmt.Sprintf("%s/webhooks?key=%s", c.webhookBaseURL, c.webhookSecret)
		}
	}

	config := WebhookConfig{
		WebhookURL:                     webhookURL,
		WebhookType:                    "enhanced",
		AccountAddresses:               addresses,
		TransactionTypes:               []string{"ANY"},
		AccountAddressTransactionTypes: currentConfig.AccountAddressTransactionTypes,
		Blocks:                         currentConfig.Blocks,
	}

	configJSON, _ := json.Marshal(config)
	log.Debug().
		Str("webhookID", c.webhookID).
		RawJSON("config", configJSON).
		Msg("Updating webhook with configuration")

	requestBody, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook config: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf("%s/webhooks/%s?api-key=%s", HeliusAPIBase, c.webhookID, c.apiKey),
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Str("webhookID", c.webhookID).
			Int("statusCode", resp.StatusCode).
			Str("responseBody", string(body)).
			Str("requestBody", string(requestBody)).
			Msg("Failed to update webhook")

		// If we failed to update, try to recreate the webhook
		return c.recreateWebhookWithAddresses(ctx, addresses)
	}

	log.Info().
		Str("webhookID", c.webhookID).
		Int("addressCount", len(addresses)).
		Msg("Successfully updated webhook addresses")

	return nil
}
func (c *HeliusClient) recreateWebhookWithAddresses(ctx context.Context, addresses []string) error {
	log.Info().Msg("Attempting to recreate webhook after update failure")

	// Try to delete the existing webhook
	oldWebhookID := c.webhookID
	err := c.DeleteWebhook(ctx, oldWebhookID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to delete old webhook, continuing anyway")
	}

	// Create a new webhook config
	config := WebhookConfig{
		WebhookType:      "enhanced",
		AccountAddresses: addresses,
		TransactionTypes: []string{"ANY"},
	}

	// Create the new webhook
	resp, err := c.CreateWebhook(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to recreate webhook: %w", err)
	}

	log.Info().
		Str("oldWebhookID", oldWebhookID).
		Str("newWebhookID", resp.WebhookID).
		Int("addressCount", len(addresses)).
		Msg("Successfully recreated webhook")

	return nil
}

func (c *HeliusClient) createWebhookWithAddresses(ctx context.Context, addresses []string, indexerID string) error {
	if c.webhookBaseURL == "" {
		return fmt.Errorf("webhook base URL is required to create a webhook")
	}

	var webhookURL string
	if c.webhookBaseURL[len(c.webhookBaseURL)-1] == '/' {
		webhookURL = fmt.Sprintf("%swebhooks?key=%s", c.webhookBaseURL, c.webhookSecret)
	} else {
		webhookURL = fmt.Sprintf("%s/webhooks?key=%s", c.webhookBaseURL, c.webhookSecret)
	}

	config := WebhookConfig{
		WebhookURL:       webhookURL,
		WebhookType:      "enhanced",
		AccountAddresses: addresses,
		TransactionTypes: []string{"ANY"},
	}

	configJSON, _ := json.Marshal(config)
	log.Info().
		RawJSON("config", configJSON).
		Msg("Creating new webhook with configuration")

	response, err := c.CreateWebhook(ctx, config)
	if err != nil {
		log.Error().
			Err(err).
			Strs("addresses", addresses).
			Msg("Failed to create webhook")
		return err
	}

	now := time.Now()
	c.addressesLock.Lock()
	for _, addr := range addresses {
		c.addresses = append(c.addresses, AddressEntry{
			Address:   addr,
			IndexerID: indexerID,
			AddedAt:   now,
		})
	}
	c.addressesLock.Unlock()

	log.Info().
		Str("webhookID", response.WebhookID).
		Str("webhookURL", webhookURL).
		Int("addressCount", len(addresses)).
		Strs("addresses", addresses).
		Msg("Successfully created new webhook with addresses")

	return nil
}

func (c *HeliusClient) updateWebhookAddresses(ctx context.Context) error {
	addresses := c.GetAddresses()

	if len(addresses) > MaxAddressesLimit {
		for i := 0; i < len(addresses); i++ {
			for j := i + 1; j < len(addresses); j++ {
				if addresses[i].AddedAt.After(addresses[j].AddedAt) {
					addresses[i], addresses[j] = addresses[j], addresses[i]
				}
			}
		}

		addresses = addresses[len(addresses)-MaxAddressesLimit:]

		c.addressesLock.Lock()
		c.addresses = addresses
		c.addressesLock.Unlock()
	}

	addressList := make([]string, len(addresses))
	for i, entry := range addresses {
		addressList[i] = entry.Address
	}

	return c.updateWebhookWithAddresses(ctx, addressList)
}

func (c *HeliusClient) DeleteWebhook(ctx context.Context, webhookID string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("%s/webhooks/%s?api-key=%s", HeliusAPIBase, webhookID, c.apiKey),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete webhook: %s (status code: %d)", string(body), resp.StatusCode)
	}

	return nil
}

func (c *HeliusClient) VerifyWebhookSignature(header, payload string) bool {
	if c.webhookSecret == "" {
		log.Warn().Msg("No webhook secret configured, skipping signature verification")
		return true
	}

	return header == c.webhookSecret
}
