package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type TokenMetadataCache struct {
	cache map[string]TokenMetadata
	mu    sync.RWMutex
}

type TokenMetadata struct {
	Symbol    string
	Name      string
	Decimals  int
	FetchedAt time.Time
}

func NewTokenMetadataCache() *TokenMetadataCache {
	return &TokenMetadataCache{
		cache: make(map[string]TokenMetadata),
	}
}

func (c *TokenMetadataCache) Get(tokenAddress string) (TokenMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	metadata, found := c.cache[strings.ToLower(tokenAddress)]

	if found && time.Since(metadata.FetchedAt) > 24*time.Hour {
		return metadata, false
	}

	return metadata, found
}

func (c *TokenMetadataCache) Set(tokenAddress string, metadata TokenMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	metadata.FetchedAt = time.Now()
	c.cache[strings.ToLower(tokenAddress)] = metadata
}

var metadataCache = NewTokenMetadataCache()

type TokenMetadataFetcher struct {
	heliusAPIKey string
	httpClient   *http.Client
}

func NewTokenMetadataFetcher(heliusAPIKey string) *TokenMetadataFetcher {
	return &TokenMetadataFetcher{
		heliusAPIKey: heliusAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (f *TokenMetadataFetcher) FetchTokenMetadata(ctx context.Context, tokenAddress string) (TokenMetadata, error) {

	if metadata, found := metadataCache.Get(tokenAddress); found {
		return metadata, nil
	}

	log.Info().Str("token", tokenAddress).Msg("Fetching token metadata from Helius DAS API")

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "metadata-request",
		"method":  "getAsset",
		"params": map[string]interface{}{
			"id": tokenAddress,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("failed to marshal getAsset request: %w", err)
	}

	requestURL := fmt.Sprintf("https://mainnet.helius-rpc.com/?api-key=%s", f.heliusAPIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug().Str("response", string(body)).Msg("Raw DAS API response")

	var response struct {
		Result struct {
			Content struct {
				Metadata struct {
					Name   string `json:"name"`
					Symbol string `json:"symbol"`
				} `json:"metadata"`
			} `json:"content"`
			TokenInfo struct {
				Decimals int `json:"decimals"`
			} `json:"token_info"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return TokenMetadata{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if response.Error != nil {
		return TokenMetadata{}, fmt.Errorf("RPC error: %s (code %d)", response.Error.Message, response.Error.Code)
	}

	metadata := TokenMetadata{
		Symbol:   response.Result.Content.Metadata.Symbol,
		Name:     response.Result.Content.Metadata.Name,
		Decimals: response.Result.TokenInfo.Decimals,
	}

	metadataCache.Set(tokenAddress, metadata)

	log.Info().
		Str("token", tokenAddress).
		Str("name", metadata.Name).
		Str("symbol", metadata.Symbol).
		Int("decimals", metadata.Decimals).
		Msg("Successfully fetched token metadata")

	return metadata, nil
}

func (f *TokenMetadataFetcher) FetchMultipleTokenMetadata(ctx context.Context, tokenAddresses []string) map[string]TokenMetadata {
	results := make(map[string]TokenMetadata)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, addr := range tokenAddresses {

		if metadata, found := metadataCache.Get(addr); found {
			mu.Lock()
			results[addr] = metadata
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(tokenAddr string) {
			defer wg.Done()

			metadata, err := f.FetchTokenMetadata(ctx, tokenAddr)
			if err != nil {
				log.Warn().Err(err).Str("token", tokenAddr).Msg("Failed to fetch token metadata from Helius DAS API")
				return
			}

			mu.Lock()
			results[tokenAddr] = metadata
			mu.Unlock()
		}(addr)
	}

	wg.Wait()
	return results
}
