package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type IndexerType string

const (
	NFTBids     IndexerType = "nft_bids"
	NFTPrices   IndexerType = "nft_prices"
	TokenBorrow IndexerType = "token_borrow"
	TokenPrices IndexerType = "token_prices"
)

type IndexerStatus string

const (
	Pending   IndexerStatus = "pending"
	Active    IndexerStatus = "active"
	Paused    IndexerStatus = "paused"
	Failed    IndexerStatus = "failed"
	Completed IndexerStatus = "completed"
)

type NFTBidParams struct {
	Collection   string   `json:"collection"`
	Marketplaces []string `json:"marketplaces,omitempty"`
}

type NFTPriceParams struct {
	Collection   string   `json:"collection"`
	Marketplaces []string `json:"marketplaces,omitempty"`
}

type TokenBorrowParams struct {
	Tokens    []string `json:"tokens"`
	Platforms []string `json:"platforms,omitempty"`
}

type TokenPriceParams struct {
	Tokens    []string `json:"tokens"`
	Platforms []string `json:"platforms,omitempty"`
}

type CreateIndexerRequest struct {
	DBCredentialID uuid.UUID       `json:"dbCredentialId" binding:"required"`
	IndexerType    IndexerType     `json:"indexerType" binding:"required"`
	TargetTable    string          `json:"targetTable" binding:"required"`
	Params         json.RawMessage `json:"params" binding:"required"`
	WebhookID      string          `json:"webhookId,omitempty"`
}

type IndexerResponse struct {
	ID             uuid.UUID     `json:"id"`
	UserID         uuid.UUID     `json:"userId"`
	DBCredentialID uuid.UUID     `json:"dbCredentialId"`
	IndexerType    IndexerType   `json:"indexerType"`
	Params         interface{}   `json:"params"`
	TargetTable    string        `json:"targetTable"`
	WebhookID      string        `json:"webhookId"`
	Status         IndexerStatus `json:"status"`
	LastIndexedAt  *time.Time    `json:"lastIndexedAt"`
	ErrorMessage   string        `json:"errorMessage"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

type IndexingLogResponse struct {
	ID        uuid.UUID   `json:"id"`
	IndexerID uuid.UUID   `json:"indexerId"`
	EventType string      `json:"eventType"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details"`
	CreatedAt time.Time   `json:"createdAt"`
}

type HeliusWebhookResponse struct {
	WebhookID string `json:"webhookID"`
	Endpoint  string `json:"webhookURL"`
}

type HeliusWebhookPayload struct {
	AccountData []HeliusAccountData `json:"accountData"`
	Slot        int64               `json:"slot"`
	Transaction HeliusTransaction   `json:"transaction,omitempty"`
}

type HeliusAccountData struct {
	Account     string          `json:"account"`
	Data        json.RawMessage `json:"data"`
	Executable  bool            `json:"executable"`
	Lamports    int64           `json:"lamports"`
	Owner       string          `json:"owner"`
	RentEpoch   int64           `json:"rentEpoch"`
	ProgramData json.RawMessage `json:"programData,omitempty"`
}

type HeliusTransaction struct {
	ID              string          `json:"id"`
	Signatures      []string        `json:"signatures"`
	FeePayerID      string          `json:"feePayerId"`
	Instructions    json.RawMessage `json:"instructions"`
	Events          json.RawMessage `json:"events"`
	Type            string          `json:"type"`
	StatusMessage   string          `json:"statusMessage"`
	EnhancedDetails json.RawMessage `json:"enhancedDetails,omitempty"`
}
