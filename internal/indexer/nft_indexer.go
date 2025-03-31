package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rishavmehra/indexer/internal/models"
)

type NFTBidIndexer struct {
	BaseIndexer
	Collection   string
	Marketplaces []string
}

func NewNFTBidIndexer(id string, params json.RawMessage) (Indexer, error) {
	base := NewBaseIndexer(id, params)

	var nftParams models.NFTBidParams
	if err := json.Unmarshal(params, &nftParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NFT bid parameters: %w", err)
	}

	if nftParams.Collection == "" {
		return nil, fmt.Errorf("collection address is required")
	}

	return &NFTBidIndexer{
		BaseIndexer:  base,
		Collection:   nftParams.Collection,
		Marketplaces: nftParams.Marketplaces,
	}, nil
}

func (i *NFTBidIndexer) Initialize(ctx context.Context, conn *pgx.Conn, targetTable string) error {
	if err := i.BaseIndexer.Initialize(ctx, conn, targetTable); err != nil {
		return err
	}

	targetTable = formatTableName(targetTable)

	exists, err := checkTableExists(ctx, conn, targetTable)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !exists {
		_, err = conn.Exec(ctx, fmt.Sprintf(`
			CREATE TABLE %s (
				id SERIAL PRIMARY KEY,
				mint_address TEXT NOT NULL,
				auction_house TEXT NOT NULL,
				marketplace TEXT NOT NULL,
				bidder TEXT NOT NULL,
				bid_amount NUMERIC NOT NULL,
				bid_currency TEXT NOT NULL,
				bid_usd_value NUMERIC,
				expiry TIMESTAMP WITH TIME ZONE,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
				slot BIGINT NOT NULL,
				transaction_id TEXT,
				UNIQUE(mint_address, bidder, auction_house)
			)
		`, targetTable))
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		_, err = conn.Exec(ctx, fmt.Sprintf(`
			CREATE INDEX %s_mint_address_idx ON %s(mint_address);
			CREATE INDEX %s_marketplace_idx ON %s(marketplace);
			CREATE INDEX %s_bidder_idx ON %s(bidder);
			CREATE INDEX %s_created_at_idx ON %s(created_at);
			CREATE INDEX %s_slot_idx ON %s(slot);
		`,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
		))
		if err != nil {
			return fmt.Errorf("failed to create indices: %w", err)
		}
	}

	return nil
}

func (i *NFTBidIndexer) GetWebhookConfig(indexerID string) (WebhookConfig, error) {
	config := WebhookConfig{
		WebhookType:      "enhanced",
		AccountAddresses: []string{i.Collection},
		TransactionTypes: []string{"ANY"},
	}

	return config, nil
}

func (i *NFTBidIndexer) ProcessPayload(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload) error {
	if len(payload.Transaction.Signatures) == 0 {
		return nil
	}

	targetTable = formatTableName(targetTable)

	var enhancedDetails map[string]interface{}
	if err := json.Unmarshal(payload.Transaction.EnhancedDetails, &enhancedDetails); err != nil {
		return fmt.Errorf("failed to unmarshal enhanced details: %w", err)
	}

	events, ok := enhancedDetails["events"].([]interface{})
	if !ok || len(events) == 0 {
		return nil
	}

	for _, eventRaw := range events {
		event, ok := eventRaw.(map[string]interface{})
		if !ok {
			continue
		}

		eventType, ok := event["type"].(string)
		if !ok {
			continue
		}

		if eventType != "NFT_BID" && eventType != "NFT_BID_CANCELLED" {
			continue
		}

		bidInfo, ok := event["data"].(map[string]interface{})
		if !ok {
			continue
		}

		mintAddress, _ := bidInfo["mint"].(string)
		auctionHouse, _ := bidInfo["auctionHouse"].(string)
		marketplace, _ := bidInfo["marketplace"].(string)
		bidder, _ := bidInfo["bidder"].(string)

		if len(i.Marketplaces) > 0 {
			found := false
			for _, m := range i.Marketplaces {
				if strings.EqualFold(marketplace, m) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		var bidAmount float64
		if amount, ok := bidInfo["amount"].(float64); ok {
			bidAmount = amount
		} else if amount, ok := bidInfo["amount"].(string); ok {
			fmt.Sscanf(amount, "%f", &bidAmount)
		}

		bidCurrency, _ := bidInfo["currency"].(string)
		if bidCurrency == "" {
			bidCurrency = "SOL"
		}

		var bidUSDValue float64
		if usdValue, ok := bidInfo["usdValue"].(float64); ok {
			bidUSDValue = usdValue
		}

		var expiry *time.Time
		if expiryStr, ok := bidInfo["expiry"].(string); ok && expiryStr != "" {
			t, err := time.Parse(time.RFC3339, expiryStr)
			if err == nil {
				expiry = &t
			}
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		if eventType == "NFT_BID" {
			_, err = tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s (
					mint_address, auction_house, marketplace, bidder, 
					bid_amount, bid_currency, bid_usd_value, expiry, slot, transaction_id
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
				) ON CONFLICT (mint_address, bidder, auction_house) 
				DO UPDATE SET 
					bid_amount = EXCLUDED.bid_amount,
					bid_usd_value = EXCLUDED.bid_usd_value,
					expiry = EXCLUDED.expiry,
					created_at = NOW(),
					slot = EXCLUDED.slot,
					transaction_id = EXCLUDED.transaction_id
			`, targetTable),
				mintAddress, auctionHouse, marketplace, bidder,
				bidAmount, bidCurrency, bidUSDValue, expiry, payload.Slot, payload.Transaction.ID,
			)
		} else if eventType == "NFT_BID_CANCELLED" {
			_, err = tx.Exec(ctx, fmt.Sprintf(`
				DELETE FROM %s 
				WHERE mint_address = $1 AND bidder = $2 AND auction_house = $3
			`, targetTable),
				mintAddress, bidder, auctionHouse,
			)
		}

		if err != nil {
			return fmt.Errorf("failed to process bid event: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return nil
}

type NFTPriceIndexer struct {
	BaseIndexer
	Collection   string
	Marketplaces []string
}

func NewNFTPriceIndexer(id string, params json.RawMessage) (Indexer, error) {
	base := NewBaseIndexer(id, params)

	var nftParams models.NFTPriceParams
	if err := json.Unmarshal(params, &nftParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NFT price parameters: %w", err)
	}

	if nftParams.Collection == "" {
		return nil, fmt.Errorf("collection address is required")
	}

	return &NFTPriceIndexer{
		BaseIndexer:  base,
		Collection:   nftParams.Collection,
		Marketplaces: nftParams.Marketplaces,
	}, nil
}

func (i *NFTPriceIndexer) Initialize(ctx context.Context, conn *pgx.Conn, targetTable string) error {
	if err := i.BaseIndexer.Initialize(ctx, conn, targetTable); err != nil {
		return err
	}

	targetTable = formatTableName(targetTable)

	exists, err := checkTableExists(ctx, conn, targetTable)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !exists {
		_, err = conn.Exec(ctx, fmt.Sprintf(`
			CREATE TABLE %s (
				id SERIAL PRIMARY KEY,
				mint_address TEXT NOT NULL,
				marketplace TEXT NOT NULL,
				price NUMERIC NOT NULL,
				currency TEXT NOT NULL,
				usd_value NUMERIC,
				seller TEXT NOT NULL,
				buyer TEXT,
				status TEXT NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
				slot BIGINT NOT NULL,
				transaction_id TEXT,
				UNIQUE(mint_address, marketplace, seller)
			)
		`, targetTable))
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		_, err = conn.Exec(ctx, fmt.Sprintf(`
			CREATE INDEX %s_mint_address_idx ON %s(mint_address);
			CREATE INDEX %s_marketplace_idx ON %s(marketplace);
			CREATE INDEX %s_seller_idx ON %s(seller);
			CREATE INDEX %s_status_idx ON %s(status);
			CREATE INDEX %s_created_at_idx ON %s(created_at);
			CREATE INDEX %s_slot_idx ON %s(slot);
		`,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
		))
		if err != nil {
			return fmt.Errorf("failed to create indices: %w", err)
		}
	}

	return nil
}

func (i *NFTPriceIndexer) GetWebhookConfig(indexerID string) (WebhookConfig, error) {
	config := WebhookConfig{
		WebhookType:      "enhanced",
		AccountAddresses: []string{i.Collection},
		TransactionTypes: []string{"ANY"},
	}

	return config, nil
}

func (i *NFTPriceIndexer) ProcessPayload(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload) error {
	if len(payload.Transaction.Signatures) == 0 {
		return nil
	}

	targetTable = formatTableName(targetTable)

	var enhancedDetails map[string]interface{}
	if err := json.Unmarshal(payload.Transaction.EnhancedDetails, &enhancedDetails); err != nil {
		return fmt.Errorf("failed to unmarshal enhanced details: %w", err)
	}

	events, ok := enhancedDetails["events"].([]interface{})
	if !ok || len(events) == 0 {
		return nil
	}

	for _, eventRaw := range events {
		event, ok := eventRaw.(map[string]interface{})
		if !ok {
			continue
		}

		eventType, ok := event["type"].(string)
		if !ok {
			continue
		}

		if eventType != "NFT_LISTING" && eventType != "NFT_SALE" && eventType != "NFT_CANCEL_LISTING" {
			continue
		}

		eventData, ok := event["data"].(map[string]interface{})
		if !ok {
			continue
		}

		mintAddress, _ := eventData["mint"].(string)
		marketplace, _ := eventData["marketplace"].(string)
		seller, _ := eventData["seller"].(string)

		if len(i.Marketplaces) > 0 {
			found := false
			for _, m := range i.Marketplaces {
				if strings.EqualFold(marketplace, m) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		var price float64
		if p, ok := eventData["amount"].(float64); ok {
			price = p
		} else if p, ok := eventData["amount"].(string); ok {
			fmt.Sscanf(p, "%f", &price)
		}

		currency, _ := eventData["currency"].(string)
		if currency == "" {
			currency = "SOL"
		}

		var usdValue float64
		if v, ok := eventData["usdValue"].(float64); ok {
			usdValue = v
		}

		var buyer string
		if b, ok := eventData["buyer"].(string); ok {
			buyer = b
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		switch eventType {
		case "NFT_LISTING":
			_, err = tx.Exec(ctx, fmt.Sprintf(`
                INSERT INTO %s (
                    mint_address, marketplace, price, currency, usd_value,
                    seller, status, slot, transaction_id
                ) VALUES (
                    $1, $2, $3, $4, $5, $6, $7, $8, $9
                ) ON CONFLICT (mint_address, marketplace, seller) 
                DO UPDATE SET 
                    price = EXCLUDED.price,
                    usd_value = EXCLUDED.usd_value,
                    status = EXCLUDED.status,
                    updated_at = NOW(),
                    slot = EXCLUDED.slot,
                    transaction_id = EXCLUDED.transaction_id
            `, targetTable),
				mintAddress, marketplace, price, currency, usdValue,
				seller, "listed", payload.Slot, payload.Transaction.ID,
			)

		case "NFT_SALE":
			result, err := tx.Exec(ctx, fmt.Sprintf(`
                UPDATE %s SET
                    status = $1,
                    buyer = $2,
                    updated_at = NOW(),
                    slot = $3,
                    transaction_id = $4
                WHERE mint_address = $5 AND marketplace = $6 AND seller = $7
                AND status = 'listed'
            `, targetTable),
				"sold", buyer, payload.Slot, payload.Transaction.ID,
				mintAddress, marketplace, seller,
			)

			if err == nil {
				rowsAffected := result.RowsAffected()
				if rowsAffected == 0 {
					_, err = tx.Exec(ctx, fmt.Sprintf(`
                        INSERT INTO %s (
                            mint_address, marketplace, price, currency, usd_value,
                            seller, buyer, status, slot, transaction_id
                        ) VALUES (
                            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
                        ) ON CONFLICT (mint_address, marketplace, seller) 
                        DO UPDATE SET 
                            price = EXCLUDED.price,
                            usd_value = EXCLUDED.usd_value,
                            buyer = EXCLUDED.buyer,
                            status = EXCLUDED.status,
                            updated_at = NOW(),
                            slot = EXCLUDED.slot,
                            transaction_id = EXCLUDED.transaction_id
                    `, targetTable),
						mintAddress, marketplace, price, currency, usdValue,
						seller, buyer, "sold", payload.Slot, payload.Transaction.ID,
					)
				}
			}

		case "NFT_CANCEL_LISTING":
			_, err = tx.Exec(ctx, fmt.Sprintf(`
                UPDATE %s SET
                    status = $1,
                    updated_at = NOW(),
                    slot = $2,
                    transaction_id = $3
                WHERE mint_address = $4 AND marketplace = $5 AND seller = $6
                AND status = 'listed'
            `, targetTable),
				"cancelled", payload.Slot, payload.Transaction.ID,
				mintAddress, marketplace, seller,
			)
		}

		if err != nil {
			return fmt.Errorf("failed to process NFT price event: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return nil
}
