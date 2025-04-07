package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

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
				signature TEXT UNIQUE NOT NULL,
				slot BIGINT NOT NULL,
				block_time TIMESTAMP WITH TIME ZONE NOT NULL,
				nft_mint TEXT NOT NULL,
				auction_house TEXT,
				marketplace TEXT NOT NULL,
				bidder TEXT NOT NULL,
				bid_amount NUMERIC NOT NULL,
				bid_currency TEXT NOT NULL DEFAULT 'SOL',
				bid_usd_value NUMERIC,
				expiry TIMESTAMP WITH TIME ZONE,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
			)
		`, targetTable))
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		_, err = conn.Exec(ctx, fmt.Sprintf(`
			CREATE INDEX %s_nft_mint_idx ON %s(nft_mint);
			CREATE INDEX %s_marketplace_idx ON %s(marketplace);
			CREATE INDEX %s_bidder_idx ON %s(bidder);
			CREATE INDEX %s_block_time_idx ON %s(block_time);
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

		log.Info().Str("table", targetTable).Msg("Successfully created NFT bids table")
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

	signature := payload.Transaction.ID
	if signature == "" && len(payload.Transaction.Signatures) > 0 {
		signature = payload.Transaction.Signatures[0]
	}

	targetTable = formatTableName(targetTable)

	log.Info().
		Str("collection", i.Collection).
		Str("targetTable", targetTable).
		Str("signature", signature).
		Int64("slot", payload.Slot).
		Msg("Processing NFT bid payload")

	var enhancedDetails map[string]interface{}
	if err := json.Unmarshal(payload.Transaction.EnhancedDetails, &enhancedDetails); err != nil {
		log.Error().Err(err).Str("signature", signature).Msg("Failed to unmarshal enhanced details")
		return fmt.Errorf("failed to unmarshal enhanced details: %w", err)
	}

	// Dump full transaction data for debugging (remove in production)
	fullJson, _ := json.Marshal(enhancedDetails)
	log.Debug().RawJSON("transaction", fullJson).Msg("Full transaction data")

	// Check if we have a description that can help us identify the transaction
	if description, ok := enhancedDetails["description"].(string); ok && description != "" {
		descLower := strings.ToLower(description)
		if strings.Contains(descLower, "bid") {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("Detected possible NFT bid from description")
		}
	}

	// Check for direct event type first (no events array)
	if eventType, ok := enhancedDetails["type"].(string); ok {
		log.Info().
			Str("signature", signature).
			Str("eventType", eventType).
			Msg("Found direct event type")

		if eventType == "NFT_BID" {
			log.Info().
				Str("signature", signature).
				Str("type", eventType).
				Msg("🔷 Found direct NFT bid event")

			return i.processBidEvent(ctx, pool, targetTable, enhancedDetails, payload.Slot, signature)
		} else if eventType == "NFT_BID_CANCELLED" {
			log.Info().
				Str("signature", signature).
				Str("type", eventType).
				Msg("❌ Found direct NFT bid cancellation event")

			return i.processBidCancellation(ctx, pool, targetTable, enhancedDetails, payload.Slot, signature)
		}
	}

	// Check for events array
	events, ok := enhancedDetails["events"].([]interface{})
	if ok && len(events) > 0 {
		log.Info().
			Str("signature", signature).
			Int("eventCount", len(events)).
			Msg("Found events array in transaction")

		foundBidEvent := false

		for idx, eventRaw := range events {
			event, ok := eventRaw.(map[string]interface{})
			if !ok {
				continue
			}

			eventType, ok := event["type"].(string)
			if !ok {
				continue
			}

			log.Info().
				Str("signature", signature).
				Int("eventIndex", idx).
				Str("eventType", eventType).
				Msg("Processing event from array")

			if eventType == "NFT_BID" {
				log.Info().
					Str("signature", signature).
					Int("eventIndex", idx).
					Str("eventType", eventType).
					Msg("🔷 Found NFT bid event in array")

				foundBidEvent = true
				if err := i.processBidEvent(ctx, pool, targetTable, event, payload.Slot, signature); err != nil {
					log.Error().Err(err).Str("signature", signature).Msg("Failed to process NFT bid event")
					return err
				}
			} else if eventType == "NFT_BID_CANCELLED" {
				log.Info().
					Str("signature", signature).
					Int("eventIndex", idx).
					Str("eventType", eventType).
					Msg("❌ Found NFT bid cancellation event in array")

				foundBidEvent = true
				if err := i.processBidCancellation(ctx, pool, targetTable, event, payload.Slot, signature); err != nil {
					log.Error().Err(err).Str("signature", signature).Msg("Failed to process NFT bid cancellation")
					return err
				}
			}
		}

		if foundBidEvent {
			return nil
		}
	}

	// Try to detect bid from description as a last resort
	if description, ok := enhancedDetails["description"].(string); ok && description != "" {
		descLower := strings.ToLower(description)
		if strings.Contains(descLower, "bid") && strings.Contains(descLower, "for") {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("🔍 Attempting to parse NFT bid from description")

			// Create synthetic bid event
			bidEvent := map[string]interface{}{
				"type":        "NFT_BID",
				"description": description,
				// Try to extract other fields from description here if needed
			}

			if err := i.processBidEvent(ctx, pool, targetTable, bidEvent, payload.Slot, signature); err != nil {
				log.Warn().
					Err(err).
					Str("signature", signature).
					Str("description", description).
					Msg("Failed to process bid from description")
			}
		}
	}

	// If we reach here and couldn't identify any NFT bid events
	log.Debug().
		Str("signature", signature).
		Msg("No NFT bid events found in transaction")

	return nil
}

func (i *NFTBidIndexer) processBidEvent(ctx context.Context, pool *pgxpool.Pool, targetTable string, eventData map[string]interface{}, slot int64, signature string) error {
	var bidData map[string]interface{}

	if data, ok := eventData["data"].(map[string]interface{}); ok {
		bidData = data
	} else {
		// Try other possible locations based on Helius format
		bidData = eventData
	}

	// Enhanced debugging of the raw event data
	rawJson, _ := json.Marshal(bidData)
	log.Debug().RawJSON("bidData", rawJson).Msg("Raw NFT bid data")

	var mintAddress, auctionHouse, marketplace, bidder, nftName string
	var bidAmount, bidUSDValue float64
	var currency string = "SOL"
	var expiryTime *time.Time

	// Extract mint address
	if mint, ok := bidData["mint"].(string); ok && mint != "" {
		mintAddress = mint
	} else if nft, ok := bidData["nft"].(map[string]interface{}); ok {
		if mint, ok := nft["mint"].(string); ok {
			mintAddress = mint
		}
		if name, ok := nft["name"].(string); ok {
			nftName = name
		}
	}

	// Try to extract NFT name from metadata
	if nftName == "" {
		if metadata, ok := bidData["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok && name != "" {
				nftName = name
			}
		}
	}

	// If we still don't have an NFT name, create one from mint address
	if nftName == "" && mintAddress != "" {
		parts := strings.Split(mintAddress, "")
		if len(parts) > 5 {
			nftName = fmt.Sprintf("NFT %s...%s", strings.Join(parts[0:3], ""), strings.Join(parts[len(parts)-3:], ""))
		}
	}

	// Only process if it matches our configured collection
	if i.Collection != "" && !strings.EqualFold(mintAddress, i.Collection) {
		collectionMatched := false

		// Check if this NFT is part of our collection - for logging purposes only
		if collectionData, ok := bidData["collection"].(map[string]interface{}); ok {
			if collectionAddr, ok := collectionData["address"].(string); ok && strings.EqualFold(collectionAddr, i.Collection) {
				collectionMatched = true
			}
		}

		// Log the collection mismatch but DO NOT filter out (removed return nil)
		if !collectionMatched {
			log.Info().
				Str("foundMint", mintAddress).
				Str("configuredCollection", i.Collection).
				Msg("Processing NFT from different collection than configured")
			// Continue processing - do not return nil
		}
	}

	// Extract auction house
	if ah, ok := bidData["auctionHouse"].(string); ok {
		auctionHouse = ah
	}

	// Extract marketplace
	if mp, ok := bidData["marketplace"].(string); ok {
		marketplace = mp
	}

	// Try different marketplace fields
	if marketplace == "" {
		if source, ok := eventData["source"].(string); ok && source != "" {
			marketplace = source
		}
	}

	// Filter by marketplace if configured
	if len(i.Marketplaces) > 0 && marketplace != "" {
		marketplaceMatched := false
		for _, m := range i.Marketplaces {
			if strings.EqualFold(marketplace, m) {
				marketplaceMatched = true
				break
			}
		}

		if !marketplaceMatched {
			log.Debug().
				Str("foundMarketplace", marketplace).
				Strs("configuredMarketplaces", i.Marketplaces).
				Msg("Skipping NFT bid - marketplace not in configured list")
			return nil
		}
	}

	// Extract bidder
	if b, ok := bidData["bidder"].(string); ok {
		bidder = b
	}

	// Extract bid amount - could be in different formats
	if amount, ok := bidData["amount"].(float64); ok {
		bidAmount = amount
	} else if amount, ok := bidData["amount"].(string); ok {
		parsedAmount, err := strconv.ParseFloat(amount, 64)
		if err == nil {
			bidAmount = parsedAmount
		}
	} else if price, ok := bidData["price"].(float64); ok {
		bidAmount = price
	}

	// Try to parse from description if needed
	if (bidder == "" || bidAmount <= 0) && eventData != nil {
		if description, ok := eventData["description"].(string); ok && description != "" {
			log.Info().
				Str("description", description).
				Msg("Attempting to extract bid info from description")

			descLower := strings.ToLower(description)
			if strings.Contains(descLower, "bid") && strings.Contains(descLower, "for") {
				// Extract bidder if needed
				if bidder == "" {
					parts := strings.Split(description, " ")
					if len(parts) > 0 {
						bidder = parts[0] // Usually first word is bidder
					}
				}

				// Extract amount if needed
				if bidAmount <= 0 {
					parts := strings.Split(description, " ")
					for i, part := range parts {
						if part == "for" && i+1 < len(parts) {
							parsedAmount, err := strconv.ParseFloat(parts[i+1], 64)
							if err == nil {
								bidAmount = parsedAmount
								if i+2 < len(parts) && strings.ToLower(parts[i+2]) == "sol" {
									currency = "SOL"
								}
							}
						}
					}
				}
			}
		}
	}

	// Extract USD value
	if usdValue, ok := bidData["usdValue"].(float64); ok {
		bidUSDValue = usdValue
	}

	// Extract currency
	if curr, ok := bidData["currency"].(string); ok && curr != "" {
		currency = curr
	}

	// Extract expiry if available
	if expiry, ok := bidData["expiry"].(string); ok && expiry != "" {
		parsedTime, err := time.Parse(time.RFC3339, expiry)
		if err == nil {
			expiryTime = &parsedTime
		}
	}

	// If we're missing essential data, log and skip
	if mintAddress == "" || bidder == "" || bidAmount <= 0 {
		log.Warn().
			Str("mint", mintAddress).
			Str("bidder", bidder).
			Float64("amount", bidAmount).
			Msg("Skipping NFT bid - missing essential data")
		return nil
	}

	// If marketplace is empty, use a default value
	if marketplace == "" {
		marketplace = "UNKNOWN"
	}

	// Use current time if no blockTime available
	blockTime := time.Now().UTC()

	// Format a nice price string with USD value if available
	priceStr := fmt.Sprintf("%.4f %s", bidAmount, currency)
	if bidUSDValue > 0 {
		priceStr += fmt.Sprintf(" ($%.2f)", bidUSDValue)
	}

	// Log the NFT bid with all key details
	log.Info().
		Str("event_type", "NFT_BID").
		Str("signature", signature).
		Int64("slot", slot).
		Str("nft_mint", mintAddress).
		Str("nft_name", nftName).
		Str("marketplace", marketplace).
		Str("bidder", bidder).
		Float64("bid_amount", bidAmount).
		Str("currency", currency).
		Float64("usd_value", bidUSDValue).
		Msg("💸 NFT BID PLACED")

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert or update the bid
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			signature, slot, block_time, nft_mint, auction_house, marketplace, 
			bidder, bid_amount, bid_currency, bid_usd_value, expiry
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) ON CONFLICT (signature) 
		DO UPDATE SET 
			nft_mint = EXCLUDED.nft_mint,
			auction_house = EXCLUDED.auction_house,
			marketplace = EXCLUDED.marketplace,
			bidder = EXCLUDED.bidder,
			bid_amount = EXCLUDED.bid_amount,
			bid_currency = EXCLUDED.bid_currency,
			bid_usd_value = EXCLUDED.bid_usd_value,
			expiry = EXCLUDED.expiry,
			slot = EXCLUDED.slot,
			block_time = EXCLUDED.block_time
	`, targetTable),
		signature, slot, blockTime, mintAddress, auctionHouse, marketplace,
		bidder, bidAmount, currency, bidUSDValue, expiryTime)

	if err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("mint", mintAddress).
			Str("bidder", bidder).
			Float64("amount", bidAmount).
			Msg("Error inserting NFT bid")
		return fmt.Errorf("failed to insert NFT bid: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Confirm database operation success
	log.Info().
		Str("signature", signature).
		Str("nft", nftName).
		Str("mint", mintAddress).
		Str("marketplace", marketplace).
		Str("bidder", bidder).
		Str("price", priceStr).
		Msg("💾 Stored NFT bid in DB")

	return nil
}

func (i *NFTBidIndexer) processBidCancellation(ctx context.Context, pool *pgxpool.Pool, targetTable string, eventData map[string]interface{}, slot int64, signature string) error {
	var bidData map[string]interface{}

	if data, ok := eventData["data"].(map[string]interface{}); ok {
		bidData = data
	} else {
		bidData = eventData
	}

	var mintAddress, bidder, auctionHouse string

	// Extract data similarly to the bid event
	if mint, ok := bidData["mint"].(string); ok {
		mintAddress = mint
	}

	if b, ok := bidData["bidder"].(string); ok {
		bidder = b
	}

	if ah, ok := bidData["auctionHouse"].(string); ok {
		auctionHouse = ah
	}

	// Only handle if we have minimum required info
	if mintAddress == "" || bidder == "" {
		log.Warn().
			Str("mint", mintAddress).
			Str("bidder", bidder).
			Msg("Skipping NFT bid cancellation - missing essential data")
		return nil
	}

	// Remove the bid from our database
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var deleteQuery string
	var queryParams []interface{}

	if auctionHouse != "" {
		deleteQuery = fmt.Sprintf(`
			DELETE FROM %s 
			WHERE nft_mint = $1 AND bidder = $2 AND auction_house = $3
		`, targetTable)
		queryParams = []interface{}{mintAddress, bidder, auctionHouse}
	} else {
		deleteQuery = fmt.Sprintf(`
			DELETE FROM %s 
			WHERE nft_mint = $1 AND bidder = $2
		`, targetTable)
		queryParams = []interface{}{mintAddress, bidder}
	}

	result, err := tx.Exec(ctx, deleteQuery, queryParams...)
	if err != nil {
		log.Error().
			Err(err).
			Str("mint", mintAddress).
			Str("bidder", bidder).
			Msg("Error deleting NFT bid")
		return fmt.Errorf("failed to delete NFT bid: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	rowsAffected := result.RowsAffected()
	log.Info().
		Str("signature", signature).
		Str("mint", mintAddress).
		Str("bidder", bidder).
		Int64("rowsAffected", rowsAffected).
		Msg("Successfully processed NFT bid cancellation")

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
		// Log the exact SQL query for debugging
		createTableSQL := fmt.Sprintf(`
            CREATE TABLE %s (
                id SERIAL PRIMARY KEY,
                signature TEXT UNIQUE NOT NULL,
                slot BIGINT NOT NULL,
                block_time TIMESTAMP WITH TIME ZONE NOT NULL,
                nft_mint TEXT NOT NULL,
                nft_name TEXT,
                marketplace TEXT NOT NULL,
                price NUMERIC NOT NULL,
                currency TEXT NOT NULL DEFAULT 'SOL',
                usd_value NUMERIC,
                seller TEXT NOT NULL,
                buyer TEXT,
                status TEXT NOT NULL,
                created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
            )
        `, targetTable)

		log.Debug().Str("sql", createTableSQL).Msg("Creating table with SQL")

		_, err = conn.Exec(ctx, createTableSQL)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		// Create indices with explicit names to avoid conflicts
		indexesSQL := fmt.Sprintf(`
            CREATE INDEX IF NOT EXISTS %s_nft_mint_idx ON %s(nft_mint);
            CREATE INDEX IF NOT EXISTS %s_marketplace_idx ON %s(marketplace);
            CREATE INDEX IF NOT EXISTS %s_seller_idx ON %s(seller);
            CREATE INDEX IF NOT EXISTS %s_buyer_idx ON %s(buyer);
            CREATE INDEX IF NOT EXISTS %s_status_idx ON %s(status);
            CREATE INDEX IF NOT EXISTS %s_block_time_idx ON %s(block_time);
            CREATE INDEX IF NOT EXISTS %s_slot_idx ON %s(slot);
        `,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
		)

		log.Debug().Str("sql", indexesSQL).Msg("Creating indices with SQL")

		_, err = conn.Exec(ctx, indexesSQL)
		if err != nil {
			return fmt.Errorf("failed to create indices: %w", err)
		}

		// Verify table creation
		var tableCount int
		err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public' AND tablename = $1", targetTable).Scan(&tableCount)
		if err != nil {
			return fmt.Errorf("failed to verify table creation: %w", err)
		}

		if tableCount != 1 {
			return fmt.Errorf("table creation verification failed, expected 1 table, found %d", tableCount)
		}

		log.Info().
			Str("table", targetTable).
			Msg("Successfully created NFT prices table and verified its existence")
	} else {
		log.Info().
			Str("table", targetTable).
			Msg("NFT prices table already exists, skipping creation")
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

	i.validateDatabaseSetup(ctx, pool, targetTable)

	if len(payload.Transaction.Signatures) == 0 {
		return nil
	}

	signature := payload.Transaction.ID
	if signature == "" && len(payload.Transaction.Signatures) > 0 {
		signature = payload.Transaction.Signatures[0]
	}

	targetTable = formatTableName(targetTable)

	log.Info().
		Str("collection", i.Collection).
		Str("targetTable", targetTable).
		Str("signature", signature).
		Int64("slot", payload.Slot).
		Msg("Processing NFT price/listing payload")

	if err := i.validateDatabaseConnection(ctx, pool, targetTable); err != nil {
		log.Error().Err(err).Str("targetTable", targetTable).Msg("Failed to validate database connection")
		return fmt.Errorf("database validation failed: %w", err)
	}

	var enhancedDetails map[string]interface{}
	if err := json.Unmarshal(payload.Transaction.EnhancedDetails, &enhancedDetails); err != nil {
		log.Error().Err(err).Str("signature", signature).Msg("Failed to unmarshal enhanced details")
		return fmt.Errorf("failed to unmarshal enhanced details: %w", err)
	}

	// Log the raw transaction type for debugging
	if txType, ok := enhancedDetails["type"].(string); ok {
		log.Debug().
			Str("signature", signature).
			Str("transactionType", txType).
			Msg("Transaction type from Helius")
	}

	// Check if we have a description that can help us classify the event
	if description, ok := enhancedDetails["description"].(string); ok && description != "" {
		log.Debug().
			Str("signature", signature).
			Str("description", description).
			Msg("Transaction description from Helius")

		descLower := strings.ToLower(description)
		if strings.Contains(descLower, "listed") && strings.Contains(descLower, "for") {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("📝 Detected NFT listing from description")
		} else if strings.Contains(descLower, "purchased") || strings.Contains(descLower, "bought") ||
			(strings.Contains(descLower, "sold") && strings.Contains(descLower, "for")) {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("📝 Detected NFT sale from description")
		} else if strings.Contains(descLower, "cancel") || strings.Contains(descLower, "delist") {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("📝 Detected NFT listing cancellation from description")
		}
	}

	// First check if we have a direct event without the events array
	if eventType, ok := enhancedDetails["type"].(string); ok {
		if eventType == "NFT_LISTING" {
			log.Info().Str("type", eventType).Msg("Found direct NFT listing event")
			return i.processListingEvent(ctx, pool, targetTable, enhancedDetails, payload.Slot, signature)
		} else if eventType == "NFT_SALE" {
			log.Info().Str("type", eventType).Msg("Found direct NFT sale event")
			return i.processSaleEvent(ctx, pool, targetTable, enhancedDetails, payload.Slot, signature)
		} else if eventType == "NFT_CANCEL_LISTING" {
			log.Info().Str("type", eventType).Msg("Found direct NFT listing cancellation event")
			return i.processCancelListingEvent(ctx, pool, targetTable, enhancedDetails, payload.Slot, signature)
		}
	}

	// Check for events array
	events, ok := enhancedDetails["events"].([]interface{})
	if ok && len(events) > 0 {
		log.Info().
			Str("signature", signature).
			Int("eventCount", len(events)).
			Msg("Found events array in transaction")

		// Process all events in the array
		for idx, eventRaw := range events {
			event, ok := eventRaw.(map[string]interface{})
			if !ok {
				continue
			}

			eventType, ok := event["type"].(string)
			if !ok {
				continue
			}

			log.Info().
				Str("signature", signature).
				Int("eventIndex", idx).
				Str("eventType", eventType).
				Msg("Processing event from array")

			if eventType == "NFT_LISTING" {
				log.Info().
					Str("signature", signature).
					Int("eventIndex", idx).
					Str("eventType", eventType).
					Msg("📋 Found NFT listing event in array")

				if err := i.processListingEvent(ctx, pool, targetTable, event, payload.Slot, signature); err != nil {
					log.Error().Err(err).Str("signature", signature).Msg("Failed to process NFT listing")
					return err
				}
			} else if eventType == "NFT_SALE" {
				log.Info().
					Str("signature", signature).
					Int("eventIndex", idx).
					Str("eventType", eventType).
					Msg("💲 Found NFT sale event in array")

				if err := i.processSaleEvent(ctx, pool, targetTable, event, payload.Slot, signature); err != nil {
					log.Error().Err(err).Str("signature", signature).Msg("Failed to process NFT sale")
					return err
				}
			} else if eventType == "NFT_CANCEL_LISTING" {
				log.Info().
					Str("signature", signature).
					Int("eventIndex", idx).
					Str("eventType", eventType).
					Msg("🚫 Found NFT cancel listing event in array")

				if err := i.processCancelListingEvent(ctx, pool, targetTable, event, payload.Slot, signature); err != nil {
					log.Error().Err(err).Str("signature", signature).Msg("Failed to process NFT listing cancellation")
					return err
				}
			}
		}
		return nil
	}

	// If we couldn't find events array, try to parse from description
	if description, ok := enhancedDetails["description"].(string); ok && description != "" {
		descLower := strings.ToLower(description)

		if strings.Contains(descLower, "listed") && strings.Contains(descLower, "for") && strings.Contains(descLower, "sol") {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("📋 Parsing NFT listing from description")

			return i.processListingFromDescription(ctx, pool, targetTable, description, enhancedDetails, payload.Slot, signature)
		} else if (strings.Contains(descLower, "bought") || strings.Contains(descLower, "purchased") ||
			(strings.Contains(descLower, "sold") && strings.Contains(descLower, "for"))) &&
			strings.Contains(descLower, "sol") {
			log.Info().
				Str("signature", signature).
				Str("description", description).
				Msg("💲 Detected NFT sale from description")

			// Create a synthetic sale event from the description
			saleEvent := map[string]interface{}{
				"type":        "NFT_SALE",
				"description": description,
			}
			return i.processSaleEvent(ctx, pool, targetTable, saleEvent, payload.Slot, signature)
		}
	}

	// If we reach here, we couldn't identify any NFT events in this transaction
	log.Debug().
		Str("signature", signature).
		Msg("No NFT events found in transaction")

	return nil
}

func (i *NFTPriceIndexer) processListingEvent(ctx context.Context, pool *pgxpool.Pool, targetTable string, eventData map[string]interface{}, slot int64, signature string) error {
	var listingData map[string]interface{}

	if data, ok := eventData["data"].(map[string]interface{}); ok {
		listingData = data
	} else {
		listingData = eventData
	}

	var mintAddress, marketplace, seller, nftName string
	var price, usdValue float64
	var currency string = "SOL"

	// Enhanced debugging of the raw event data
	rawJson, _ := json.Marshal(listingData)
	log.Debug().RawJSON("listingData", rawJson).Msg("Raw NFT listing data")

	// Extract mint address
	if mint, ok := listingData["mint"].(string); ok && mint != "" {
		mintAddress = mint
	} else if nft, ok := listingData["nft"].(map[string]interface{}); ok {
		if mint, ok := nft["mint"].(string); ok {
			mintAddress = mint
		}
		if name, ok := nft["name"].(string); ok {
			nftName = name
		}
	}

	// Try to extract NFT name from metadata if available
	if nftName == "" {
		if metadata, ok := listingData["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok && name != "" {
				nftName = name
			}
		}
	}

	// If we still don't have an NFT name, try the NFT's metadata
	if nftName == "" && mintAddress != "" {
		parts := strings.Split(mintAddress, "")
		if len(parts) > 5 {
			nftName = fmt.Sprintf("NFT %s...%s", strings.Join(parts[0:3], ""), strings.Join(parts[len(parts)-3:], ""))
		}
	}

	// Only process if it matches our configured collection
	if i.Collection != "" && !strings.EqualFold(mintAddress, i.Collection) {
		collectionMatched := false

		// Check if this NFT is part of our collection - for logging purposes only
		if collectionData, ok := listingData["collection"].(map[string]interface{}); ok {
			if collectionAddr, ok := collectionData["address"].(string); ok && strings.EqualFold(collectionAddr, i.Collection) {
				collectionMatched = true
			}
		}

		// Log the collection mismatch but DO NOT filter out (removed return nil)
		if !collectionMatched {
			log.Info().
				Str("foundMint", mintAddress).
				Str("configuredCollection", i.Collection).
				Msg("Processing NFT from different collection than configured")
			// Continue processing - do not return nil
		}
	}

	// Extract marketplace
	if mp, ok := listingData["marketplace"].(string); ok {
		marketplace = mp
	}

	// Filter by marketplace if configured
	if len(i.Marketplaces) > 0 && marketplace != "" {
		marketplaceMatched := false
		for _, m := range i.Marketplaces {
			if strings.EqualFold(marketplace, m) {
				marketplaceMatched = true
				break
			}
		}

		if !marketplaceMatched {
			log.Debug().
				Str("foundMarketplace", marketplace).
				Strs("configuredMarketplaces", i.Marketplaces).
				Msg("Skipping NFT listing - marketplace not in configured list")
			return nil
		}
	}

	// Extract seller
	if s, ok := listingData["seller"].(string); ok {
		seller = s
	}

	// Extract price - could be in different formats
	if amount, ok := listingData["amount"].(float64); ok {
		price = amount
	} else if amount, ok := listingData["amount"].(string); ok {
		parsedAmount, err := strconv.ParseFloat(amount, 64)
		if err == nil {
			price = parsedAmount
		}
	} else if p, ok := listingData["price"].(float64); ok {
		price = p
	}

	// Extract USD value
	if usd, ok := listingData["usdValue"].(float64); ok {
		usdValue = usd
	}

	// Extract currency
	if curr, ok := listingData["currency"].(string); ok && curr != "" {
		currency = curr
	}

	// If we're missing essential data, try to parse from description
	if mintAddress == "" || seller == "" || price <= 0 {
		if description, ok := eventData["description"].(string); ok && description != "" {
			log.Info().
				Str("description", description).
				Msg("Attempting to extract listing data from description")
			return i.processListingFromDescription(ctx, pool, targetTable, description, eventData, slot, signature)
		}

		log.Warn().
			Str("mint", mintAddress).
			Str("seller", seller).
			Float64("price", price).
			Msg("Skipping NFT listing - missing essential data")
		return nil
	}

	// If marketplace is empty, use a default value
	if marketplace == "" {
		marketplace = "UNKNOWN"
	}

	// Use current time if no blockTime available
	blockTime := time.Now().UTC()

	// Log the NFT listing with all key details
	log.Info().
		Str("event_type", "NFT_LISTING").
		Str("signature", signature).
		Int64("slot", slot).
		Str("nft_mint", mintAddress).
		Str("nft_name", nftName).
		Str("marketplace", marketplace).
		Str("seller", seller).
		Float64("price", price).
		Str("currency", currency).
		Float64("usd_value", usdValue).
		Msg("✨ NFT LISTED")

	// DIAGNOSTIC: Check table existence before attempting insertion
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to acquire connection for table verification")
		return fmt.Errorf("failed to acquire connection: %w", err)
	}

	// Check if table exists before proceeding
	var tableExists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM pg_tables 
			WHERE schemaname = 'public' 
			AND tablename = $1
		)
	`, targetTable).Scan(&tableExists)

	conn.Release()

	if err != nil {
		log.Error().Err(err).Str("table", targetTable).Msg("⚠️ Failed to check if table exists")
		return fmt.Errorf("failed to check table existence: %w", err)
	}

	if !tableExists {
		log.Error().Str("table", targetTable).Msg("⚠️ Target table does not exist!")
		return fmt.Errorf("target table %s does not exist", targetTable)
	}

	log.Info().Str("table", targetTable).Msg("✅ Target table exists, proceeding with insertion")

	// Create a transaction with extended timeout
	dbCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// MODIFIED: Use simpler direct connection approach first to diagnose issues
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (
			signature, slot, block_time, nft_mint, nft_name, marketplace, 
			price, currency, usd_value, seller, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) ON CONFLICT (signature) 
		DO UPDATE SET 
			nft_mint = EXCLUDED.nft_mint,
			nft_name = CASE WHEN EXCLUDED.nft_name IS NOT NULL AND EXCLUDED.nft_name != '' THEN EXCLUDED.nft_name ELSE nft_name END,
			marketplace = EXCLUDED.marketplace,
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			usd_value = EXCLUDED.usd_value,
			seller = EXCLUDED.seller,
			status = EXCLUDED.status,
			slot = EXCLUDED.slot,
			block_time = EXCLUDED.block_time,
			updated_at = NOW()
	`, targetTable)

	log.Info().
		Str("sql", insertSQL).
		Str("signature", signature).
		Str("mint", mintAddress).
		Str("table", targetTable).
		Msg("🔍 Executing SQL insert")

	// Try with direct connection first to get more detailed error info
	directConn, err := pool.Acquire(dbCtx)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to acquire direct connection")
		return fmt.Errorf("failed to acquire direct connection: %w", err)
	}
	defer directConn.Release()

	// Execute the query directly first
	_, err = directConn.Exec(dbCtx, insertSQL,
		signature, slot, blockTime, mintAddress, nftName, marketplace,
		price, currency, usdValue, seller, "listed")

	if err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("table", targetTable).
			Str("mint", mintAddress).
			Float64("price", price).
			Msg("❌ Direct insert failed")

		// Check for specific error types
		if pgErr, ok := err.(*pgconn.PgError); ok {
			log.Error().
				Str("pgErrorCode", pgErr.Code).
				Str("pgErrorMessage", pgErr.Message).
				Str("pgErrorDetail", pgErr.Detail).
				Msg("⚠️ PostgreSQL error details")
		}

		return fmt.Errorf("failed to insert NFT listing: %w", err)
	}

	// If direct execution worked, try with transaction
	tx, err := pool.Begin(dbCtx)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to begin transaction (but direct insert succeeded)")
		// Don't return error here since direct insert worked
		return nil
	}
	defer tx.Rollback(dbCtx)

	_, err = tx.Exec(dbCtx, insertSQL,
		signature, slot, blockTime, mintAddress, nftName, marketplace,
		price, currency, usdValue, seller, "listed")

	if err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("mint", mintAddress).
			Str("seller", seller).
			Float64("price", price).
			Msg("❌ Error inserting NFT listing in transaction")
		// Don't return error since direct insert worked
		return nil
	}

	if err := tx.Commit(dbCtx); err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Msg("❌ Failed to commit transaction (but direct insert succeeded)")
		// Don't return error since direct insert worked
		return nil
	}

	// Confirm database operation success
	priceStr := fmt.Sprintf("%.4f %s ($%.2f)", price, currency, usdValue)
	log.Info().
		Str("signature", signature).
		Str("nft", nftName).
		Str("mint", mintAddress).
		Str("marketplace", marketplace).
		Str("seller", seller).
		Str("price", priceStr).
		Msg("💾 Stored NFT listing in DB")

	return nil
}

func (i *NFTPriceIndexer) processListingFromDescription(ctx context.Context, pool *pgxpool.Pool, targetTable string, description string, eventData map[string]interface{}, slot int64, signature string) error {
	// Example format: "4wv6eShMW5ReztgeYd3kQHqbm4joY8QUpnS3SkdDhwyX listed Ivy #268 for 11.24999 SOL on MAGIC_EDEN."

	parts := strings.Split(description, " ")
	if len(parts) < 6 {
		log.Warn().Str("description", description).Msg("Insufficient parts in description for NFT listing")
		return nil
	}

	var seller, nftName, marketplace string
	var price float64
	var currency string = "SOL"

	// First part is usually the seller
	seller = parts[0]

	// Find the "for" to extract price
	for i := 0; i < len(parts); i++ {
		if parts[i] == "for" && i+1 < len(parts) {
			priceStr := parts[i+1]
			parsedPrice, err := strconv.ParseFloat(priceStr, 64)
			if err == nil {
				price = parsedPrice
			}
		}
	}

	// Find the "on" to extract marketplace
	for i := 0; i < len(parts); i++ {
		if parts[i] == "on" && i+1 < len(parts) {
			marketplace = strings.TrimSuffix(parts[i+1], ".")
		}
	}

	// Try to extract NFT name - usually between "listed" and "for"
	var nftNameParts []string
	listedIndex := -1
	forIndex := -1

	for i, part := range parts {
		if part == "listed" {
			listedIndex = i
		} else if part == "for" {
			forIndex = i
			break
		}
	}

	if listedIndex != -1 && forIndex != -1 && listedIndex+1 < forIndex {
		nftNameParts = parts[listedIndex+1 : forIndex]
		nftName = strings.Join(nftNameParts, " ")
	}

	// If we couldn't extract essential information, log and skip
	if seller == "" || price <= 0 {
		log.Warn().
			Str("description", description).
			Str("seller", seller).
			Float64("price", price).
			Msg("Could not extract essential listing data from description")
		return nil
	}

	// Check if marketplace is in our configured list if we have one
	if len(i.Marketplaces) > 0 && marketplace != "" {
		marketplaceMatched := false
		for _, m := range i.Marketplaces {
			if strings.EqualFold(marketplace, m) {
				marketplaceMatched = true
				break
			}
		}

		if !marketplaceMatched {
			log.Debug().
				Str("foundMarketplace", marketplace).
				Strs("configuredMarketplaces", i.Marketplaces).
				Msg("Skipping NFT listing - marketplace not in configured list")
			return nil
		}
	}

	// If marketplace is empty, use a default value
	if marketplace == "" {
		marketplace = "UNKNOWN"
	}

	// Use current time if no blockTime available
	blockTime := time.Now().UTC()

	// Try to extract mint address from the event data
	var mintAddress string
	if instructions, ok := eventData["instructions"].([]interface{}); ok {
		for _, instruction := range instructions {
			if instructionData, ok := instruction.(map[string]interface{}); ok {
				if accounts, ok := instructionData["accounts"].([]interface{}); ok {
					// Look for an account address that might be an NFT mint
					// This is a heuristic and may need adjustment
					for _, account := range accounts {
						if accountAddr, ok := account.(string); ok && len(accountAddr) >= 32 {
							mintAddress = accountAddr
							break
						}
					}
				}
			}
		}
	}

	// If we couldn't extract a mint address, treat this as informational only
	if mintAddress == "" {
		// 1. First try to use collection address
		mintAddress = i.Collection
		log.Info().
			Str("signature", signature).
			Str("description", description).
			Str("seller", seller).
			Float64("price", price).
			Str("marketplace", marketplace).
			Str("collectionAsAddress", mintAddress).
			Msg("Using collection as mint address for NFT listing")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert the listing
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			signature, slot, block_time, nft_mint, nft_name, marketplace, 
			price, currency, seller, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) ON CONFLICT (signature) 
		DO UPDATE SET 
			nft_mint = EXCLUDED.nft_mint,
			nft_name = CASE WHEN EXCLUDED.nft_name IS NOT NULL AND EXCLUDED.nft_name != '' THEN EXCLUDED.nft_name ELSE %s.nft_name END,
			marketplace = EXCLUDED.marketplace,
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			seller = EXCLUDED.seller,
			status = EXCLUDED.status,
			slot = EXCLUDED.slot,
			block_time = EXCLUDED.block_time,
			updated_at = NOW()
	`, targetTable, targetTable),
		signature, slot, blockTime, mintAddress, nftName, marketplace,
		price, currency, seller, "listed")

	if err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("description", description).
			Msg("Error inserting NFT listing from description")
		return fmt.Errorf("failed to insert NFT listing from description: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("signature", signature).
		Str("mint", mintAddress).
		Str("nftName", nftName).
		Str("marketplace", marketplace).
		Str("seller", seller).
		Float64("price", price).
		Msg("Successfully stored NFT listing from description")

	return nil
}

func (i *NFTPriceIndexer) processSaleEvent(ctx context.Context, pool *pgxpool.Pool, targetTable string, eventData map[string]interface{}, slot int64, signature string) error {
	var saleData map[string]interface{}

	if data, ok := eventData["data"].(map[string]interface{}); ok {
		saleData = data
	} else {
		saleData = eventData
	}

	// Enhanced debugging of the raw event data
	rawJson, _ := json.Marshal(saleData)
	log.Debug().RawJSON("saleData", rawJson).Msg("Raw NFT sale data")

	var mintAddress, marketplace, seller, buyer, nftName string
	var price, usdValue float64
	var currency string = "SOL"

	// Extract mint address
	if mint, ok := saleData["mint"].(string); ok && mint != "" {
		mintAddress = mint
	} else if nft, ok := saleData["nft"].(map[string]interface{}); ok {
		if mint, ok := nft["mint"].(string); ok {
			mintAddress = mint
		}
		if name, ok := nft["name"].(string); ok {
			nftName = name
		}
	}

	// Try to extract NFT name from metadata if available
	if nftName == "" {
		if metadata, ok := saleData["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok && name != "" {
				nftName = name
			}
		}
	}

	// If we still don't have an NFT name, try the NFT's metadata
	if nftName == "" && mintAddress != "" {
		// This is intentionally kept simple - you could expand with actual metadata fetching
		parts := strings.Split(mintAddress, "")
		if len(parts) > 5 {
			nftName = fmt.Sprintf("NFT %s...%s", parts[0:3], parts[len(parts)-3:])
		}
	}

	// Only process if it matches our configured collection
	if i.Collection != "" && !strings.EqualFold(mintAddress, i.Collection) {
		collectionMatched := false

		// Check if this NFT is part of our collection - for logging purposes only
		if collectionData, ok := saleData["collection"].(map[string]interface{}); ok {
			if collectionAddr, ok := collectionData["address"].(string); ok && strings.EqualFold(collectionAddr, i.Collection) {
				collectionMatched = true
			}
		}

		// Log the collection mismatch but DO NOT filter out (removed return nil)
		if !collectionMatched {
			log.Info().
				Str("foundMint", mintAddress).
				Str("configuredCollection", i.Collection).
				Msg("Processing NFT from different collection than configured")
			// Continue processing - do not return nil
		}
	}

	// Extract marketplace
	if mp, ok := saleData["marketplace"].(string); ok {
		marketplace = mp
	}

	// Check marketplace field in different formats
	if marketplace == "" {
		if source, ok := eventData["source"].(string); ok && source != "" {
			marketplace = source
		}
	}

	// Filter by marketplace if configured
	if len(i.Marketplaces) > 0 && marketplace != "" {
		marketplaceMatched := false
		for _, m := range i.Marketplaces {
			if strings.EqualFold(marketplace, m) {
				marketplaceMatched = true
				break
			}
		}

		if !marketplaceMatched {
			log.Debug().
				Str("foundMarketplace", marketplace).
				Strs("configuredMarketplaces", i.Marketplaces).
				Msg("Skipping NFT sale - marketplace not in configured list")
			return nil
		}
	}

	// Extract seller
	if s, ok := saleData["seller"].(string); ok {
		seller = s
	}

	// Extract buyer
	if b, ok := saleData["buyer"].(string); ok {
		buyer = b
	}

	// Extract price - could be in different formats
	if amount, ok := saleData["amount"].(float64); ok {
		price = amount
	} else if amount, ok := saleData["amount"].(string); ok {
		parsedAmount, err := strconv.ParseFloat(amount, 64)
		if err == nil {
			price = parsedAmount
		}
	} else if p, ok := saleData["price"].(float64); ok {
		price = p
	}

	// Try to extract from description if we don't have price
	if price <= 0 {
		if description, ok := eventData["description"].(string); ok && description != "" {
			// Example: "abc123...xyz sold NFT to buyer456 for 10.5 SOL on MARKETPLACE"
			parts := strings.Split(description, " ")
			for i, part := range parts {
				if part == "for" && i+1 < len(parts) {
					parsedPrice, err := strconv.ParseFloat(parts[i+1], 64)
					if err == nil {
						price = parsedPrice
						if i+2 < len(parts) && parts[i+2] == "SOL" {
							currency = "SOL"
						}
					}
				}
			}
		}
	}

	// Extract USD value
	if usd, ok := saleData["usdValue"].(float64); ok {
		usdValue = usd
	}

	// Extract currency
	if curr, ok := saleData["currency"].(string); ok && curr != "" {
		currency = curr
	}

	// If we're missing essential data, log and skip
	if mintAddress == "" || seller == "" || buyer == "" || price <= 0 {
		// Additional attempt to extract from description
		if description, ok := eventData["description"].(string); ok && description != "" {
			log.Info().Str("description", description).Msg("Attempting to extract sale from description")

			// Try to find seller and buyer in description
			descLower := strings.ToLower(description)
			if strings.Contains(descLower, "sold") && strings.Contains(descLower, "for") {
				parts := strings.Split(description, " ")
				if len(parts) > 3 {
					// First part is usually seller
					if seller == "" {
						seller = parts[0]
					}

					// Look for buyer after "sold" and before "for"
					soldIndex := -1
					forIndex := -1
					for i, part := range parts {
						if strings.ToLower(part) == "sold" {
							soldIndex = i
						} else if part == "for" {
							forIndex = i
							break
						}
					}

					if soldIndex != -1 && forIndex != -1 && soldIndex+2 < forIndex {
						potentialBuyer := parts[soldIndex+2]
						if buyer == "" {
							buyer = potentialBuyer
						}
					}
				}
			}
		}

		// If still missing data, skip
		if mintAddress == "" || seller == "" || buyer == "" || price <= 0 {
			log.Warn().
				Str("mint", mintAddress).
				Str("seller", seller).
				Str("buyer", buyer).
				Float64("price", price).
				Msg("Skipping NFT sale - missing essential data")
			return nil
		}
	}

	// If marketplace is empty, use a default value
	if marketplace == "" {
		marketplace = "UNKNOWN"
	}

	// Use current time if no blockTime available
	blockTime := time.Now().UTC()

	// Log the NFT sale with all key details
	log.Info().
		Str("event_type", "NFT_SALE").
		Str("signature", signature).
		Int64("slot", slot).
		Str("nft_mint", mintAddress).
		Str("nft_name", nftName).
		Str("marketplace", marketplace).
		Str("seller", seller).
		Str("buyer", buyer).
		Float64("price", price).
		Str("currency", currency).
		Float64("usd_value", usdValue).
		Msg("🎉 NFT SOLD")

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// First try to update an existing listing for this mint/seller
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET
			status = 'sold',
			buyer = $1,
			updated_at = NOW(),
			slot = $2,
			block_time = $3,
			signature = $4
		WHERE nft_mint = $5 AND seller = $6 AND status = 'listed'
		AND marketplace = $7
	`, targetTable),
		buyer, slot, blockTime, signature,
		mintAddress, seller, marketplace)

	if err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("mint", mintAddress).
			Str("seller", seller).
			Str("buyer", buyer).
			Msg("Error updating existing NFT listing to sold")
		// Continue to insert as a direct sale
	}

	rowsAffected := int64(0)
	if err == nil {
		rowsAffected = result.RowsAffected()
	}

	// If we didn't update an existing listing, insert as a direct sale
	if rowsAffected == 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (
				signature, slot, block_time, nft_mint, nft_name, marketplace, 
				price, currency, usd_value, seller, buyer, status
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
			) ON CONFLICT (signature) 
			DO UPDATE SET 
				nft_mint = EXCLUDED.nft_mint,
				nft_name = CASE WHEN EXCLUDED.nft_name IS NOT NULL AND EXCLUDED.nft_name != '' THEN EXCLUDED.nft_name ELSE %s.nft_name END,
				marketplace = EXCLUDED.marketplace,
				price = EXCLUDED.price,
				currency = EXCLUDED.currency,
				usd_value = EXCLUDED.usd_value,
				seller = EXCLUDED.seller,
				buyer = EXCLUDED.buyer,
				status = EXCLUDED.status,
				slot = EXCLUDED.slot,
				block_time = EXCLUDED.block_time,
				updated_at = NOW()
		`, targetTable, targetTable),
			signature, slot, blockTime, mintAddress, nftName, marketplace,
			price, currency, usdValue, seller, buyer, "sold")

		if err != nil {
			log.Error().
				Err(err).
				Str("signature", signature).
				Str("mint", mintAddress).
				Str("seller", seller).
				Str("buyer", buyer).
				Float64("price", price).
				Msg("Error inserting NFT sale")
			return fmt.Errorf("failed to insert NFT sale: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Format a clean price string with USD value if available
	priceStr := fmt.Sprintf("%.4f %s", price, currency)
	if usdValue > 0 {
		priceStr += fmt.Sprintf(" ($%.2f)", usdValue)
	}

	if rowsAffected > 0 {
		log.Info().
			Str("signature", signature).
			Str("nft", nftName).
			Str("mint", mintAddress).
			Str("marketplace", marketplace).
			Str("seller", seller).
			Str("buyer", buyer).
			Str("price", priceStr).
			Msg("💰 Updated existing NFT listing to SOLD")
	} else {
		log.Info().
			Str("signature", signature).
			Str("nft", nftName).
			Str("mint", mintAddress).
			Str("marketplace", marketplace).
			Str("seller", seller).
			Str("buyer", buyer).
			Str("price", priceStr).
			Msg("💰 Stored new NFT sale in DB")
	}

	return nil
}

func (i *NFTPriceIndexer) processCancelListingEvent(ctx context.Context, pool *pgxpool.Pool, targetTable string, eventData map[string]interface{}, slot int64, signature string) error {
	var cancelData map[string]interface{}

	if data, ok := eventData["data"].(map[string]interface{}); ok {
		cancelData = data
	} else {
		cancelData = eventData
	}

	var mintAddress, marketplace, seller string

	// Extract mint address
	if mint, ok := cancelData["mint"].(string); ok {
		mintAddress = mint
	} else if nft, ok := cancelData["nft"].(map[string]interface{}); ok {
		if mint, ok := nft["mint"].(string); ok {
			mintAddress = mint
		}
	}

	// Only process if it matches our configured collection
	if i.Collection != "" && !strings.EqualFold(mintAddress, i.Collection) {
		collectionMatched := false

		// Check if this NFT is part of our collection - for logging purposes only
		if collectionData, ok := cancelData["collection"].(map[string]interface{}); ok {
			if collectionAddr, ok := collectionData["address"].(string); ok && strings.EqualFold(collectionAddr, i.Collection) {
				collectionMatched = true
			}
		}

		// Log the collection mismatch but DO NOT filter out (removed return nil)
		if !collectionMatched {
			log.Info().
				Str("foundMint", mintAddress).
				Str("configuredCollection", i.Collection).
				Msg("Processing NFT from different collection than configured")
			// Continue processing - do not return nil
		}
	}

	// Extract marketplace
	if mp, ok := cancelData["marketplace"].(string); ok {
		marketplace = mp
	}

	// Filter by marketplace if configured
	if len(i.Marketplaces) > 0 && marketplace != "" {
		marketplaceMatched := false
		for _, m := range i.Marketplaces {
			if strings.EqualFold(marketplace, m) {
				marketplaceMatched = true
				break
			}
		}

		if !marketplaceMatched {
			log.Debug().
				Str("foundMarketplace", marketplace).
				Strs("configuredMarketplaces", i.Marketplaces).
				Msg("Skipping NFT listing cancellation - marketplace not in configured list")
			return nil
		}
	}

	// Extract seller
	if s, ok := cancelData["seller"].(string); ok {
		seller = s
	}

	// If we're missing essential data, log and skip
	if mintAddress == "" || seller == "" {
		log.Warn().
			Str("mint", mintAddress).
			Str("seller", seller).
			Msg("Skipping NFT listing cancellation - missing essential data")
		return nil
	}

	// If marketplace is empty, use a default value
	if marketplace == "" {
		marketplace = "UNKNOWN"
	}

	// Use current time if no blockTime available
	blockTime := time.Now().UTC()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update existing listing to cancelled status
	result, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET
			status = 'cancelled',
			updated_at = NOW(),
			slot = $1,
			block_time = $2,
			signature = $3
		WHERE nft_mint = $4 AND seller = $5 AND status = 'listed'
		AND (marketplace = $6 OR $6 = 'UNKNOWN')
	`, targetTable),
		slot, blockTime, signature,
		mintAddress, seller, marketplace)

	if err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("mint", mintAddress).
			Str("seller", seller).
			Msg("Error updating NFT listing to cancelled status")
		return fmt.Errorf("failed to update NFT listing to cancelled: %w", err)
	}

	rowsAffected := result.RowsAffected()

	// If we didn't find an existing listing, add an informational record
	if rowsAffected == 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (
				signature, slot, block_time, nft_mint, marketplace, 
				price, currency, seller, status
			) VALUES (
				$1, $2, $3, $4, $5, 0, $6, $7, $8
			) ON CONFLICT (signature) DO NOTHING
		`, targetTable),
			signature, slot, blockTime, mintAddress, marketplace,
			"SOL", seller, "cancelled")

		if err != nil {
			log.Error().
				Err(err).
				Str("signature", signature).
				Str("mint", mintAddress).
				Str("seller", seller).
				Msg("Error inserting NFT listing cancellation record")
			// Continue - this is just an informational record
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("signature", signature).
		Str("mint", mintAddress).
		Str("marketplace", marketplace).
		Str("seller", seller).
		Int64("rowsAffected", rowsAffected).
		Msg("Successfully processed NFT listing cancellation")

	return nil
}

func (i *NFTPriceIndexer) validateDatabaseConnection(ctx context.Context, pool *pgxpool.Pool, targetTable string) error {
	// First verify we can ping the database
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Format the table name
	targetTable = formatTableName(targetTable)

	// Check if the table exists and has the right schema
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	var tableExists bool
	err = conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT FROM pg_tables
            WHERE schemaname = 'public'
            AND tablename = $1
        )
    `, targetTable).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if !tableExists {
		return fmt.Errorf("target table %s does not exist", targetTable)
	}

	// Verify table has necessary columns
	var columnCount int
	err = conn.QueryRow(ctx, `
        SELECT COUNT(*) FROM information_schema.columns 
        WHERE table_schema = 'public' 
        AND table_name = $1
        AND column_name IN ('signature', 'slot', 'block_time', 'nft_mint', 'marketplace', 'price', 'seller', 'status')
    `, targetTable).Scan(&columnCount)

	if err != nil {
		return fmt.Errorf("failed to check table columns: %w", err)
	}

	if columnCount < 8 {
		return fmt.Errorf("table %s is missing required columns, found %d of 8 required columns", targetTable, columnCount)
	}

	// Test insert and rollback to verify permissions
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Test query with one row that will be rolled back
	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            signature, slot, block_time, nft_mint, nft_name, marketplace, 
            price, currency, seller, status
        ) VALUES (
            'test-signature-to-rollback', 0, NOW(), 'test-mint', 'Test NFT', 'TEST', 
            0, 'SOL', 'test-seller', 'test'
        )
    `, targetTable))

	if err != nil {
		return fmt.Errorf("failed to execute test insert: %w", err)
	}

	// Rollback the test transaction
	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("failed to rollback test transaction: %w", err)
	}

	return nil
}

func (i *NFTPriceIndexer) validateDatabaseSetup(ctx context.Context, pool *pgxpool.Pool, targetTable string) {
	// Check the database connection
	if err := pool.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to ping database")
		return
	}

	log.Info().Msg("✅ Database connection is healthy")

	// Check table existence
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to acquire connection")
		return
	}
	defer conn.Release()

	var tableExists bool
	err = conn.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT FROM pg_tables 
            WHERE schemaname = 'public' 
            AND tablename = $1
        )
    `, targetTable).Scan(&tableExists)

	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to check table existence")
		return
	}

	if !tableExists {
		log.Error().Str("table", targetTable).Msg("⚠️ Target table does not exist")

		// Try to list all tables
		rows, err := conn.Query(ctx, `
            SELECT tablename FROM pg_tables 
            WHERE schemaname = 'public'
        `)
		if err != nil {
			log.Error().Err(err).Msg("⚠️ Failed to list tables")
			return
		}
		defer rows.Close()

		tables := []string{}
		for rows.Next() {
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				log.Error().Err(err).Msg("⚠️ Failed to scan table name")
				continue
			}
			tables = append(tables, tableName)
		}

		log.Info().Strs("available_tables", tables).Msg("Available tables in database")
		return
	}

	log.Info().Str("table", targetTable).Msg("✅ Target table exists")

	// Check table schema
	rows, err := conn.Query(ctx, `
        SELECT column_name, data_type 
        FROM information_schema.columns 
        WHERE table_name = $1
    `, targetTable)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to get table schema")
		return
	}
	defer rows.Close()

	columns := map[string]string{}
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err != nil {
			log.Error().Err(err).Msg("⚠️ Failed to scan column info")
			continue
		}
		columns[colName] = dataType
	}

	log.Info().Interface("columns", columns).Msg("✅ Table schema validated")

	// Attempt a test insert and rollback
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ Failed to begin transaction for test insert")
		return
	}
	defer tx.Rollback(ctx)

	testInsertSQL := fmt.Sprintf(`
        INSERT INTO %s (
            signature, slot, block_time, nft_mint, marketplace, price, seller, status
        ) VALUES (
            'test-signature-to-be-rolled-back', 0, NOW(), 'test-mint', 'TEST', 0, 'test-seller', 'test'
        )
    `, targetTable)

	_, err = tx.Exec(ctx, testInsertSQL)
	if err != nil {
		log.Error().Err(err).Str("sql", testInsertSQL).Msg("⚠️ Test insert failed")

		// Check for specific error types
		if pgErr, ok := err.(*pgconn.PgError); ok {
			log.Error().
				Str("pgErrorCode", pgErr.Code).
				Str("pgErrorMessage", pgErr.Message).
				Str("pgErrorDetail", pgErr.Detail).
				Msg("⚠️ PostgreSQL error details")
		}
		return
	}

	// Successfully rolled back if we reach here
	log.Info().Msg("✅ Test insert and rollback successful")
}
