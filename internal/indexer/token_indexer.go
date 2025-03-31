package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/models"
)

type TokenPriceIndexer struct {
	BaseIndexer
	Tokens    []string
	Platforms []string
}

func NewTokenPriceIndexer(id string, params json.RawMessage) (Indexer, error) {
	base := NewBaseIndexer(id, params)

	var tokenParams models.TokenPriceParams
	if err := json.Unmarshal(params, &tokenParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token price parameters: %w", err)
	}

	if len(tokenParams.Tokens) == 0 {
		return nil, fmt.Errorf("at least one token address is required")
	}

	return &TokenPriceIndexer{
		BaseIndexer: base,
		Tokens:      tokenParams.Tokens,
		Platforms:   tokenParams.Platforms,
	}, nil
}

func (i *TokenPriceIndexer) Initialize(ctx context.Context, conn *pgx.Conn, targetTable string) error {

	return i.InitializeWithAPIKey(ctx, conn, targetTable, "")
}

func (i *TokenPriceIndexer) InitializeWithAPIKey(ctx context.Context, conn *pgx.Conn, targetTable string, heliusAPIKey string) error {
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
				token_address TEXT NOT NULL,
				token_name TEXT,
				token_symbol TEXT,
				platform TEXT NOT NULL,
				price_usd NUMERIC NOT NULL DEFAULT 0,
				price_sol NUMERIC DEFAULT 0,
				volume_24h NUMERIC,
				market_cap NUMERIC,
				liquidity NUMERIC,
				price_change_24h NUMERIC,
				total_supply NUMERIC,
				transaction_id TEXT,
				updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
				slot BIGINT NOT NULL,
				UNIQUE(token_address, platform)
			)
		`, targetTable))
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}

		_, err = conn.Exec(ctx, fmt.Sprintf(`
			CREATE INDEX %s_token_address_idx ON %s(token_address);
			CREATE INDEX %s_platform_idx ON %s(platform);
			CREATE INDEX %s_updated_at_idx ON %s(updated_at);
			CREATE INDEX %s_slot_idx ON %s(slot);
		`,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
			targetTable, targetTable,
		))
		if err != nil {
			return fmt.Errorf("failed to create indices: %w", err)
		}

		log.Info().
			Str("targetTable", targetTable).
			Msg("Successfully created token price table with enhanced schema")
	}

	if heliusAPIKey != "" {
		log.Info().Strs("tokens", i.Tokens).Msg("Pre-fetching token metadata at initialization")
		metadataFetcher := NewTokenMetadataFetcher(heliusAPIKey)
		tokenMetadata := metadataFetcher.FetchMultipleTokenMetadata(ctx, i.Tokens)

		if len(tokenMetadata) > 0 {
			for tokenAddr, metadata := range tokenMetadata {
				log.Info().
					Str("token", tokenAddr).
					Str("name", metadata.Name).
					Str("symbol", metadata.Symbol).
					Msg("Initializing token metadata in database")

				for _, platform := range []string{"UNKNOWN", "JUPITER", "RAYDIUM", "ORCA", "OPENBOOK"} {
					_, err := conn.Exec(ctx, fmt.Sprintf(`
						INSERT INTO %s (
							token_address, token_name, token_symbol, platform, 
							price_usd, price_sol, updated_at, slot
						) VALUES (
							$1, $2, $3, $4, 0, 0, NOW(), 0
						) ON CONFLICT (token_address, platform) 
						DO UPDATE SET 
							token_name = CASE WHEN %s.token_name IS NULL OR %s.token_name = '' OR %s.token_name = 'UNKNOWN' THEN $2 ELSE %s.token_name END,
							token_symbol = CASE WHEN %s.token_symbol IS NULL OR %s.token_symbol = '' OR %s.token_symbol = 'UNKNOWN' THEN $3 ELSE %s.token_symbol END
					`, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable),
						tokenAddr, metadata.Name, metadata.Symbol, platform,
					)
					if err != nil {
						log.Warn().Err(err).Str("token", tokenAddr).Str("platform", platform).Msg("Failed to initialize token metadata")
					}
				}
			}
		}
	}

	return nil
}

func (i *TokenPriceIndexer) GetWebhookConfig(indexerID string) (WebhookConfig, error) {

	config := WebhookConfig{
		WebhookType:      "enhanced",
		AccountAddresses: i.Tokens,
		TransactionTypes: []string{"ANY"},
	}

	return config, nil
}

func (i *TokenPriceIndexer) ProcessPayload(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload) error {

	if len(payload.Transaction.Signatures) == 0 {
		log.Debug().Msg("Skipping payload with no signatures")
		return nil
	}

	targetTable = formatTableName(targetTable)

	log.Info().
		Strs("tracking_tokens", i.Tokens).
		Strs("tracking_platforms", i.Platforms).
		Str("targetTable", targetTable).
		Msg("Processing token price payload")

	var enhancedDetails map[string]interface{}
	if err := json.Unmarshal(payload.Transaction.EnhancedDetails, &enhancedDetails); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal enhanced details")
		return fmt.Errorf("failed to unmarshal enhanced details: %w", err)
	}

	source := "UNKNOWN"
	if sourceVal, hasSource := enhancedDetails["source"].(string); hasSource && sourceVal != "" {
		source = sourceVal
	}

	if len(i.Platforms) > 0 {
		platformMatch := false
		for _, p := range i.Platforms {
			if strings.EqualFold(source, p) {
				platformMatch = true
				break
			}
		}
		if !platformMatch {
			log.Debug().Str("platform", source).Msg("Platform not in tracking list, skipping")
			return nil
		}
	}

	transactionID := ""
	if len(payload.Transaction.Signatures) > 0 {
		transactionID = payload.Transaction.Signatures[0]
	}

	switch {
	case enhancedDetails["type"] == "SWAP":

		return i.processSwapTransaction(ctx, pool, targetTable, enhancedDetails, source, payload.Slot, transactionID)

	case enhancedDetails["type"] == "JUPITER_SWAP":

		return i.processJupiterSwap(ctx, pool, targetTable, enhancedDetails, source, payload.Slot, transactionID)
	}

	if events, hasEvents := enhancedDetails["events"].([]interface{}); hasEvents && len(events) > 0 {
		for _, eventRaw := range events {
			event, ok := eventRaw.(map[string]interface{})
			if !ok {
				continue
			}

			eventType, ok := event["type"].(string)
			if !ok || (eventType != "SWAP" && eventType != "JUPITER_SWAP") {
				continue
			}

			log.Info().Str("eventType", eventType).Msg("Found swap event, processing")
			if err := i.processSwapEvent(ctx, pool, targetTable, event, source, payload.Slot, transactionID); err != nil {
				log.Error().Err(err).Msg("Error processing swap event")

			}
		}
	}

	if tokenTransfers, hasTransfers := enhancedDetails["tokenTransfers"].([]interface{}); hasTransfers && len(tokenTransfers) > 0 {
		log.Info().Int("transferCount", len(tokenTransfers)).Msg("Found token transfers")

		for _, transferRaw := range tokenTransfers {
			if err := i.processTokenTransfer(ctx, pool, targetTable, transferRaw, source, payload.Slot, transactionID); err != nil {
				log.Error().Err(err).Msg("Error processing token transfer")

			}
		}
	}

	if nativeBalances, hasNative := enhancedDetails["nativeBalanceChanges"].([]interface{}); hasNative && len(nativeBalances) > 0 {
		log.Info().Int("nativeBalanceCount", len(nativeBalances)).Msg("Found native balance changes")

	}

	if tokenBalances, hasBalances := enhancedDetails["tokenBalances"].([]interface{}); hasBalances && len(tokenBalances) > 0 {
		log.Info().Int("balanceCount", len(tokenBalances)).Msg("Found token balances")

		for _, balanceRaw := range tokenBalances {
			if err := i.processTokenBalance(ctx, pool, targetTable, balanceRaw, source, payload.Slot, transactionID); err != nil {
				log.Error().Err(err).Msg("Error processing token balance")

			}
		}
	}

	if accounts, hasAccounts := enhancedDetails["accountData"].([]interface{}); hasAccounts && len(accounts) > 0 {
		log.Info().Int("accountCount", len(accounts)).Msg("Found account data")

	}

	return nil
}

func (i *TokenPriceIndexer) processTokenTransfer(ctx context.Context, pool *pgxpool.Pool, targetTable string, transferRaw interface{}, platform string, slot int64, transactionID string) error {
	transfer, ok := transferRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid token transfer data format")
	}

	mint, hasMint := transfer["mint"].(string)
	if !hasMint {
		return fmt.Errorf("missing mint address in token transfer")
	}

	tokenMatch := false
	for _, t := range i.Tokens {
		if strings.EqualFold(mint, t) {
			tokenMatch = true
			break
		}
	}

	if !tokenMatch {
		return nil
	}

	log.Info().
		Str("mint", mint).
		Str("platform", platform).
		Msg("Processing price data for tracked token")

	var tokenSymbol string
	if symbol, ok := transfer["tokenSymbol"].(string); ok && symbol != "" {
		tokenSymbol = symbol
	} else if symbol, ok := transfer["symbol"].(string); ok && symbol != "" {
		tokenSymbol = symbol
	}

	var tokenName string
	if name, ok := transfer["tokenName"].(string); ok && name != "" {
		tokenName = name
	} else if name, ok := transfer["name"].(string); ok && name != "" {
		tokenName = name
	}

	var amount float64
	if amountVal, ok := transfer["tokenAmount"].(float64); ok {
		amount = amountVal
	} else if amountVal, ok := transfer["amount"].(float64); ok {
		amount = amountVal
	}

	var priceUSD float64 = 0
	if usdValue, ok := transfer["usdValue"].(float64); ok {
		priceUSD = usdValue
	}

	log.Debug().
		Str("token", mint).
		Str("symbol", tokenSymbol).
		Str("name", tokenName).
		Float64("amount", amount).
		Float64("priceUSD", priceUSD).
		Msg("Extracted token data from transfer")

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            token_address, token_name, token_symbol, platform, 
            price_usd, price_sol, volume_24h, market_cap, liquidity,
            price_change_24h, total_supply, transaction_id, updated_at, slot
        ) VALUES (
            $1, $2, $3, $4, $5, $6, NULL, NULL, NULL, NULL, NULL, $7, NOW(), $8
        ) ON CONFLICT (token_address, platform) 
        DO UPDATE SET 
            token_name = CASE WHEN EXCLUDED.token_name != '' AND %s.token_name IS NULL THEN EXCLUDED.token_name ELSE %s.token_name END,
            token_symbol = CASE WHEN EXCLUDED.token_symbol != '' AND %s.token_symbol IS NULL THEN EXCLUDED.token_symbol ELSE %s.token_symbol END,
            price_usd = CASE WHEN EXCLUDED.price_usd > 0 THEN EXCLUDED.price_usd ELSE %s.price_usd END,
            price_sol = CASE WHEN EXCLUDED.price_sol > 0 THEN EXCLUDED.price_sol ELSE %s.price_sol END,
            updated_at = CASE WHEN EXCLUDED.slot > %s.slot THEN NOW() ELSE %s.updated_at END,
            slot = GREATEST(EXCLUDED.slot, %s.slot),
            transaction_id = CASE WHEN EXCLUDED.slot > %s.slot THEN EXCLUDED.transaction_id ELSE %s.transaction_id END
    `, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable),
		mint, tokenName, tokenSymbol, platform,
		priceUSD, amount, transactionID, slot,
	)

	if err != nil {
		return fmt.Errorf("failed to insert/update token price: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("token", mint).
		Str("symbol", tokenSymbol).
		Str("platform", platform).
		Float64("price_usd", priceUSD).
		Float64("price_sol", amount).
		Int64("slot", slot).
		Msg("Successfully updated token price data from transfer")

	return nil
}

func (i *TokenPriceIndexer) processTokenBalance(ctx context.Context, pool *pgxpool.Pool, targetTable string, balanceRaw interface{}, platform string, slot int64, transactionID string) error {
	balance, ok := balanceRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid token balance data format")
	}

	mint, hasMint := balance["mint"].(string)
	if !hasMint {
		return fmt.Errorf("missing mint address in token balance")
	}

	tokenMatch := false
	for _, t := range i.Tokens {
		if strings.EqualFold(mint, t) {
			tokenMatch = true
			break
		}
	}

	if !tokenMatch {
		return nil
	}

	var tokenSymbol, tokenName string
	if tokenInfo, ok := balance["tokenInfo"].(map[string]interface{}); ok {
		if symbol, ok := tokenInfo["symbol"].(string); ok {
			tokenSymbol = symbol
		}
		if name, ok := tokenInfo["name"].(string); ok {
			tokenName = name
		}
	}

	if tokenSymbol == "" && tokenName == "" {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            token_address, token_name, token_symbol, platform, 
            price_usd, price_sol, transaction_id, updated_at, slot
        ) VALUES (
            $1, $2, $3, $4, 0, 0, $5, NOW(), $6
        ) ON CONFLICT (token_address, platform) 
        DO UPDATE SET 
            token_name = CASE 
                WHEN %s.token_name IS NULL OR %s.token_name = '' THEN 
                    CASE WHEN EXCLUDED.token_name != '' THEN EXCLUDED.token_name ELSE %s.token_name END
                ELSE %s.token_name
            END,
            token_symbol = CASE 
                WHEN %s.token_symbol IS NULL OR %s.token_symbol = '' THEN 
                    CASE WHEN EXCLUDED.token_symbol != '' THEN EXCLUDED.token_symbol ELSE %s.token_symbol END
                ELSE %s.token_symbol
            END
    `, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable),
		mint, tokenName, tokenSymbol, platform, transactionID, slot,
	)

	if err != nil {
		return fmt.Errorf("failed to update token metadata: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("token", mint).
		Str("symbol", tokenSymbol).
		Str("name", tokenName).
		Msg("Updated token metadata from balance")

	return nil
}

func (i *TokenPriceIndexer) processSwapTransaction(ctx context.Context, pool *pgxpool.Pool, targetTable string, txDetails map[string]interface{}, platform string, slot int64, transactionID string) error {

	var inputToken, outputToken map[string]interface{}
	var found bool

	if swap, ok := txDetails["swap"].(map[string]interface{}); ok {
		inputToken, found = swap["tokenIn"].(map[string]interface{})
		if !found {
			log.Warn().Msg("Missing input token in swap")
		}

		outputToken, found = swap["tokenOut"].(map[string]interface{})
		if !found {
			log.Warn().Msg("Missing output token in swap")
		}
	}

	if inputToken != nil {
		if err := i.processSwapTokenData(ctx, pool, targetTable, inputToken, platform, slot, transactionID); err != nil {
			log.Error().Err(err).Msg("Error processing swap input token")
		}
	}

	if outputToken != nil {
		if err := i.processSwapTokenData(ctx, pool, targetTable, outputToken, platform, slot, transactionID); err != nil {
			log.Error().Err(err).Msg("Error processing swap output token")
		}
	}

	return nil
}

func (i *TokenPriceIndexer) shouldFetchMarketData(platform string) bool {

	majorPlatforms := map[string]bool{
		"JUPITER": true,
		"RAYDIUM": true,
		"ORCA":    true,
	}
	return majorPlatforms[strings.ToUpper(platform)]
}

func (i *TokenPriceIndexer) fetchExternalMarketData(ctx context.Context, tokenAddress, tokenSymbol, platform string) {

	log.Info().
		Str("token", tokenAddress).
		Str("symbol", tokenSymbol).
		Str("platform", platform).
		Msg("Would fetch external market data here")

}

func (i *TokenPriceIndexer) processJupiterSwap(ctx context.Context, pool *pgxpool.Pool, targetTable string, txDetails map[string]interface{}, platform string, slot int64, transactionID string) error {

	var inputToken, outputToken map[string]interface{}
	var found bool

	if swapInfo, ok := txDetails["jupiterSwap"].(map[string]interface{}); ok {
		inputToken, found = swapInfo["inputToken"].(map[string]interface{})
		if !found {
			log.Warn().Msg("Missing input token in Jupiter swap")
		}

		outputToken, found = swapInfo["outputToken"].(map[string]interface{})
		if !found {
			log.Warn().Msg("Missing output token in Jupiter swap")
		}
	}

	if inputToken != nil {
		if err := i.processJupiterToken(ctx, pool, targetTable, inputToken, "JUPITER", slot, transactionID); err != nil {
			log.Error().Err(err).Msg("Error processing Jupiter input token")
		}
	}

	if outputToken != nil {
		if err := i.processJupiterToken(ctx, pool, targetTable, outputToken, "JUPITER", slot, transactionID); err != nil {
			log.Error().Err(err).Msg("Error processing Jupiter output token")
		}
	}

	return nil
}

func (i *TokenPriceIndexer) processJupiterToken(ctx context.Context, pool *pgxpool.Pool, targetTable string, tokenData map[string]interface{}, platform string, slot int64, transactionID string) error {

	mintAddress, ok := tokenData["mint"].(string)
	if !ok {
		return fmt.Errorf("missing mint address in Jupiter token")
	}

	tokenMatch := false
	for _, t := range i.Tokens {
		if strings.EqualFold(mintAddress, t) {
			tokenMatch = true
			break
		}
	}

	if !tokenMatch {
		return nil
	}

	tokenName := ""
	if name, ok := tokenData["name"].(string); ok {
		tokenName = name
	}

	tokenSymbol := ""
	if symbol, ok := tokenData["symbol"].(string); ok {
		tokenSymbol = symbol
	}

	var priceUSD float64
	if price, ok := tokenData["priceUsd"].(float64); ok {
		priceUSD = price
	}

	var priceSol float64
	if price, ok := tokenData["priceSol"].(float64); ok {
		priceSol = price
	}

	var volume24h, marketCap, liquidity, priceChange24h, totalSupply float64

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            token_address, token_name, token_symbol, platform, 
            price_usd, price_sol, volume_24h, market_cap, liquidity,
            price_change_24h, total_supply, transaction_id, updated_at, slot
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), $13
        ) ON CONFLICT (token_address, platform) 
        DO UPDATE SET 
            token_name = CASE WHEN EXCLUDED.token_name != '' THEN EXCLUDED.token_name ELSE %s.token_name END,
            token_symbol = CASE WHEN EXCLUDED.token_symbol != '' THEN EXCLUDED.token_symbol ELSE %s.token_symbol END,
            price_usd = CASE WHEN EXCLUDED.price_usd > 0 THEN EXCLUDED.price_usd ELSE %s.price_usd END,
            price_sol = CASE WHEN EXCLUDED.price_sol > 0 THEN EXCLUDED.price_sol ELSE %s.price_sol END,
            volume_24h = CASE WHEN EXCLUDED.volume_24h > 0 THEN EXCLUDED.volume_24h ELSE %s.volume_24h END,
            market_cap = CASE WHEN EXCLUDED.market_cap > 0 THEN EXCLUDED.market_cap ELSE %s.market_cap END,
            liquidity = CASE WHEN EXCLUDED.liquidity > 0 THEN EXCLUDED.liquidity ELSE %s.liquidity END,
            price_change_24h = CASE WHEN EXCLUDED.price_change_24h != 0 THEN EXCLUDED.price_change_24h ELSE %s.price_change_24h END,
            total_supply = CASE WHEN EXCLUDED.total_supply > 0 THEN EXCLUDED.total_supply ELSE %s.total_supply END,
            transaction_id = CASE WHEN EXCLUDED.slot > %s.slot THEN EXCLUDED.transaction_id ELSE %s.transaction_id END,
            updated_at = CASE WHEN EXCLUDED.slot > %s.slot THEN NOW() ELSE %s.updated_at END,
            slot = GREATEST(EXCLUDED.slot, %s.slot)
    `, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable),
		mintAddress, tokenName, tokenSymbol, platform,
		priceUSD, priceSol, volume24h, marketCap, liquidity,
		priceChange24h, totalSupply, transactionID, slot,
	)

	if err != nil {
		return fmt.Errorf("failed to update token data: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("token", mintAddress).
		Str("symbol", tokenSymbol).
		Str("platform", platform).
		Float64("priceUsd", priceUSD).
		Float64("priceSol", priceSol).
		Msg("Successfully processed Jupiter token data")

	return nil
}

func (i *TokenPriceIndexer) processSwapEvent(ctx context.Context, pool *pgxpool.Pool, targetTable string, eventRaw interface{}, platform string, slot int64, transactionID string) error {
	event, ok := eventRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	swapInfo, ok := event["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid swap data format")
	}

	swapJSON, _ := json.Marshal(swapInfo)
	log.Debug().RawJSON("swapInfo", swapJSON).Msg("Processing SWAP event")

	for _, tokenField := range []string{"tokenIn", "tokenOut"} {
		if err := i.processSwapTokenField(ctx, pool, targetTable, swapInfo, tokenField, platform, slot, transactionID); err != nil {
			log.Error().Err(err).Str("field", tokenField).Msg("Error processing swap token")
		}
	}

	return nil
}

func (i *TokenPriceIndexer) processSwapTokenField(ctx context.Context, pool *pgxpool.Pool, targetTable string, swapInfo map[string]interface{}, tokenField string, platform string, slot int64, transactionID string) error {

	tokenData, ok := swapInfo[tokenField].(map[string]interface{})
	if !ok {

		log.Debug().Str("field", tokenField).Msg("Token field not found at top level, checking alternatives")
		return nil
	}

	mint, ok := tokenData["mint"].(string)
	if !ok {
		return fmt.Errorf("missing mint address in %s", tokenField)
	}

	tokenMatch := false
	for _, t := range i.Tokens {
		if strings.EqualFold(mint, t) {
			tokenMatch = true
			break
		}
	}

	if !tokenMatch {
		return nil
	}

	tokenSymbol := ""
	if symbol, ok := tokenData["symbol"].(string); ok && symbol != "" {
		tokenSymbol = symbol
	} else if tokenInfo, ok := tokenData["tokenInfo"].(map[string]interface{}); ok {
		if symbol, ok := tokenInfo["symbol"].(string); ok {
			tokenSymbol = symbol
		}
	}

	tokenName := ""
	if name, ok := tokenData["name"].(string); ok && name != "" {
		tokenName = name
	} else if tokenInfo, ok := tokenData["tokenInfo"].(map[string]interface{}); ok {
		if name, ok := tokenInfo["name"].(string); ok {
			tokenName = name
		}
	}

	var priceUSD, priceSOL float64 = 0, 0

	if priceData, ok := tokenData["price"].(map[string]interface{}); ok {
		if usd, ok := priceData["usd"].(float64); ok {
			priceUSD = usd
		}
		if sol, ok := priceData["sol"].(float64); ok {
			priceSOL = sol
		}
	} else if usd, ok := tokenData["usdValue"].(float64); ok {
		priceUSD = usd
	}

	var volume24h, marketCap, liquidity, priceChange24h, totalSupply float64 = 0, 0, 0, 0, 0

	if marketData, ok := tokenData["marketData"].(map[string]interface{}); ok {
		if vol, ok := marketData["volume24h"].(float64); ok {
			volume24h = vol
		}
		if cap, ok := marketData["marketCap"].(float64); ok {
			marketCap = cap
		}
		if liq, ok := marketData["liquidity"].(float64); ok {
			liquidity = liq
		}
		if change, ok := marketData["priceChange24h"].(float64); ok {
			priceChange24h = change
		}
		if supply, ok := marketData["totalSupply"].(float64); ok {
			totalSupply = supply
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            token_address, token_name, token_symbol, platform, 
            price_usd, price_sol, volume_24h, market_cap, liquidity,
            price_change_24h, total_supply, transaction_id, updated_at, slot
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), $13
        ) ON CONFLICT (token_address, platform) 
        DO UPDATE SET 
            token_name = CASE 
                WHEN EXCLUDED.token_name != '' THEN EXCLUDED.token_name 
                ELSE COALESCE(%s.token_name, '')
            END,
            token_symbol = CASE 
                WHEN EXCLUDED.token_symbol != '' THEN EXCLUDED.token_symbol 
                ELSE COALESCE(%s.token_symbol, '')
            END,
            price_usd = CASE 
                WHEN EXCLUDED.price_usd > 0 THEN EXCLUDED.price_usd 
                ELSE COALESCE(%s.price_usd, 0)
            END,
            price_sol = CASE 
                WHEN EXCLUDED.price_sol > 0 THEN EXCLUDED.price_sol 
                ELSE COALESCE(%s.price_sol, 0)
            END,
            volume_24h = CASE 
                WHEN EXCLUDED.volume_24h > 0 THEN EXCLUDED.volume_24h 
                ELSE %s.volume_24h
            END,
            market_cap = CASE 
                WHEN EXCLUDED.market_cap > 0 THEN EXCLUDED.market_cap 
                ELSE %s.market_cap
            END,
            liquidity = CASE 
                WHEN EXCLUDED.liquidity > 0 THEN EXCLUDED.liquidity 
                ELSE %s.liquidity
            END,
            price_change_24h = CASE 
                WHEN EXCLUDED.price_change_24h != 0 THEN EXCLUDED.price_change_24h 
                ELSE %s.price_change_24h
            END,
            total_supply = CASE 
                WHEN EXCLUDED.total_supply > 0 THEN EXCLUDED.total_supply 
                ELSE %s.total_supply
            END,
            transaction_id = CASE 
                WHEN EXCLUDED.slot > COALESCE(%s.slot, 0) THEN EXCLUDED.transaction_id 
                ELSE %s.transaction_id
            END,
            updated_at = NOW(),
            slot = GREATEST(EXCLUDED.slot, COALESCE(%s.slot, 0))
    `, targetTable,
		targetTable, targetTable, targetTable, targetTable,
		targetTable, targetTable, targetTable, targetTable, targetTable,
		targetTable, targetTable, targetTable),
		mint, tokenName, tokenSymbol, platform,
		priceUSD, priceSOL, volume24h, marketCap, liquidity,
		priceChange24h, totalSupply, transactionID, slot,
	)

	if err != nil {
		return fmt.Errorf("failed to insert/update token from swap: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("token", mint).
		Str("symbol", tokenSymbol).
		Str("platform", platform).
		Float64("priceUSD", priceUSD).
		Float64("priceSOL", priceSOL).
		Float64("volume24h", volume24h).
		Float64("marketCap", marketCap).
		Int64("slot", slot).
		Msg("Successfully processed swap token")

	return nil
}

func (i *TokenPriceIndexer) processSwapTokenData(ctx context.Context, pool *pgxpool.Pool, targetTable string, tokenData map[string]interface{}, platform string, slot int64, transactionID string) error {

	mintAddress, ok := tokenData["mint"].(string)
	if !ok {

		if address, ok := tokenData["address"].(string); ok {
			mintAddress = address
		} else {
			return fmt.Errorf("missing mint address in token data")
		}
	}

	tokenMatch := false
	for _, t := range i.Tokens {
		if strings.EqualFold(mintAddress, t) {
			tokenMatch = true
			break
		}
	}

	if !tokenMatch {
		return nil
	}

	tokenName := ""
	if name, ok := tokenData["name"].(string); ok {
		tokenName = name
	}

	tokenSymbol := ""
	if symbol, ok := tokenData["symbol"].(string); ok {
		tokenSymbol = symbol
	}

	var priceUSD, priceSol float64 = 0, 0
	if priceInfo, ok := tokenData["price"].(map[string]interface{}); ok {
		if usd, ok := priceInfo["usd"].(float64); ok {
			priceUSD = usd
		}
		if sol, ok := priceInfo["sol"].(float64); ok {
			priceSol = sol
		}
	}

	var volume24h, marketCap, liquidity, priceChange24h, totalSupply float64 = 0, 0, 0, 0, 0
	if marketData, ok := tokenData["marketData"].(map[string]interface{}); ok {
		if vol, ok := marketData["volume24h"].(float64); ok {
			volume24h = vol
		}
		if cap, ok := marketData["marketCap"].(float64); ok {
			marketCap = cap
		}
		if liq, ok := marketData["liquidity"].(float64); ok {
			liquidity = liq
		}
		if change, ok := marketData["priceChange24h"].(float64); ok {
			priceChange24h = change
		}
		if supply, ok := marketData["totalSupply"].(float64); ok {
			totalSupply = supply
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            token_address, token_name, token_symbol, platform, 
            price_usd, price_sol, volume_24h, market_cap, liquidity,
            price_change_24h, total_supply, transaction_id, updated_at, slot
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), $13
        ) ON CONFLICT (token_address, platform) 
        DO UPDATE SET 
            token_name = CASE WHEN EXCLUDED.token_name != '' THEN EXCLUDED.token_name ELSE %s.token_name END,
            token_symbol = CASE WHEN EXCLUDED.token_symbol != '' THEN EXCLUDED.token_symbol ELSE %s.token_symbol END,
            price_usd = CASE WHEN EXCLUDED.price_usd > 0 THEN EXCLUDED.price_usd ELSE %s.price_usd END,
            price_sol = CASE WHEN EXCLUDED.price_sol > 0 THEN EXCLUDED.price_sol ELSE %s.price_sol END,
            volume_24h = CASE WHEN EXCLUDED.volume_24h > 0 THEN EXCLUDED.volume_24h ELSE %s.volume_24h END,
            market_cap = CASE WHEN EXCLUDED.market_cap > 0 THEN EXCLUDED.market_cap ELSE %s.market_cap END,
            liquidity = CASE WHEN EXCLUDED.liquidity > 0 THEN EXCLUDED.liquidity ELSE %s.liquidity END,
            price_change_24h = CASE WHEN EXCLUDED.price_change_24h != 0 THEN EXCLUDED.price_change_24h ELSE %s.price_change_24h END,
            total_supply = CASE WHEN EXCLUDED.total_supply > 0 THEN EXCLUDED.total_supply ELSE %s.total_supply END,
            transaction_id = CASE WHEN EXCLUDED.slot > %s.slot THEN EXCLUDED.transaction_id ELSE %s.transaction_id END,
            updated_at = CASE WHEN EXCLUDED.slot > %s.slot THEN NOW() ELSE %s.updated_at END,
            slot = GREATEST(EXCLUDED.slot, %s.slot)
    `, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable),
		mintAddress, tokenName, tokenSymbol, platform,
		priceUSD, priceSol, volume24h, marketCap, liquidity,
		priceChange24h, totalSupply, transactionID, slot,
	)

	if err != nil {
		return fmt.Errorf("failed to update token from swap: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("token", mintAddress).
		Str("symbol", tokenSymbol).
		Str("platform", platform).
		Float64("priceUSD", priceUSD).
		Float64("priceSol", priceSol).
		Float64("volume24h", volume24h).
		Float64("marketCap", marketCap).
		Msg("Successfully processed token from swap")

	return nil
}

type TokenBorrowIndexer struct {
	BaseIndexer
	Tokens    []string
	Platforms []string
}

func NewTokenBorrowIndexer(id string, params json.RawMessage) (Indexer, error) {
	base := NewBaseIndexer(id, params)

	var tokenParams models.TokenBorrowParams
	if err := json.Unmarshal(params, &tokenParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token borrow parameters: %w", err)
	}

	if len(tokenParams.Tokens) == 0 {
		return nil, fmt.Errorf("at least one token address is required")
	}

	return &TokenBorrowIndexer{
		BaseIndexer: base,
		Tokens:      tokenParams.Tokens,
		Platforms:   tokenParams.Platforms,
	}, nil
}

func (i *TokenBorrowIndexer) GetWebhookConfig(indexerID string) (WebhookConfig, error) {

	config := WebhookConfig{
		WebhookType:      "enhanced",
		AccountAddresses: i.Tokens,
		TransactionTypes: []string{"ANY"},
	}

	return config, nil
}

func (i *TokenBorrowIndexer) ProcessPayload(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload) error {

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

		if !isLendingEvent(eventType) {
			continue
		}

		eventData, ok := event["data"].(map[string]interface{})
		if !ok {
			continue
		}

		var tokenAddress string
		var platform string
		var availableAmount float64
		var borrowRate float64
		var supplyRate float64
		var utilizationRate float64
		var totalBorrowed float64
		var totalSupplied float64

		if token, ok := eventData["token"].(map[string]interface{}); ok {
			if mint, ok := token["mint"].(string); ok {

				for _, t := range i.Tokens {
					if strings.EqualFold(mint, t) {
						tokenAddress = mint
						break
					}
				}
			}
		}

		if tokenAddress == "" {
			continue
		}

		if source, ok := eventData["source"].(string); ok {
			platform = source
		} else if protocol, ok := eventData["protocol"].(string); ok {
			platform = protocol
		}

		if len(i.Platforms) > 0 {
			found := false
			for _, p := range i.Platforms {
				if strings.EqualFold(platform, p) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if marketData, ok := eventData["marketData"].(map[string]interface{}); ok {
			if amount, ok := marketData["availableAmount"].(float64); ok {
				availableAmount = amount
			}
			if bRate, ok := marketData["borrowRate"].(float64); ok {
				borrowRate = bRate
			}
			if sRate, ok := marketData["supplyRate"].(float64); ok {
				supplyRate = sRate
			}
			if uRate, ok := marketData["utilizationRate"].(float64); ok {
				utilizationRate = uRate
			}
			if borrowed, ok := marketData["totalBorrowed"].(float64); ok {
				totalBorrowed = borrowed
			}
			if supplied, ok := marketData["totalSupplied"].(float64); ok {
				totalSupplied = supplied
			}
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (
				token_address, platform, available_amount, borrow_rate, supply_rate,
				utilization_rate, total_borrowed, total_supplied, updated_at, slot
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, NOW(), $9
			) ON CONFLICT (token_address, platform) 
			DO UPDATE SET 
				available_amount = EXCLUDED.available_amount,
				borrow_rate = EXCLUDED.borrow_rate,
				supply_rate = EXCLUDED.supply_rate,
				utilization_rate = EXCLUDED.utilization_rate,
				total_borrowed = EXCLUDED.total_borrowed,
				total_supplied = EXCLUDED.total_supplied,
				updated_at = NOW(),
				slot = EXCLUDED.slot
		`, targetTable),
			tokenAddress, platform, availableAmount, borrowRate, supplyRate,
			utilizationRate, totalBorrowed, totalSupplied, payload.Slot,
		)
		if err != nil {
			return fmt.Errorf("failed to process token borrow event: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return nil
}

func isLendingEvent(eventType string) bool {
	lendingEvents := map[string]bool{
		"BORROW":       true,
		"REPAY":        true,
		"SUPPLY":       true,
		"WITHDRAW":     true,
		"LIQUIDATE":    true,
		"UPDATE_RATES": true,
	}
	return lendingEvents[eventType]
}

func (i *TokenPriceIndexer) ProcessPayloadWithMetadata(ctx context.Context, pool *pgxpool.Pool, targetTable string, payload models.HeliusWebhookPayload, heliusAPIKey string) error {

	if len(payload.Transaction.Signatures) == 0 {
		log.Debug().Msg("Skipping payload with no signatures")
		return nil
	}

	targetTable = formatTableName(targetTable)

	log.Info().
		Strs("tracking_tokens", i.Tokens).
		Strs("tracking_platforms", i.Platforms).
		Str("targetTable", targetTable).
		Msg("Processing token price payload with metadata")

	var enhancedDetails map[string]interface{}
	if err := json.Unmarshal(payload.Transaction.EnhancedDetails, &enhancedDetails); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal enhanced details")
		return fmt.Errorf("failed to unmarshal enhanced details: %w", err)
	}

	source := "UNKNOWN"
	if sourceVal, hasSource := enhancedDetails["source"].(string); hasSource && sourceVal != "" {
		source = sourceVal
	}

	if len(i.Platforms) > 0 {
		platformMatch := false
		for _, p := range i.Platforms {
			if strings.EqualFold(source, p) {
				platformMatch = true
				break
			}
		}
		if !platformMatch {
			log.Debug().Str("platform", source).Msg("Platform not in tracking list, skipping")
			return nil
		}
	}

	transactionID := ""
	if len(payload.Transaction.Signatures) > 0 {
		transactionID = payload.Transaction.Signatures[0]
	}

	switch {
	case enhancedDetails["type"] == "SWAP":

		if err := i.processSwapTransaction(ctx, pool, targetTable, enhancedDetails, source, payload.Slot, transactionID); err != nil {
			return err
		}
	case enhancedDetails["type"] == "JUPITER_SWAP":

		if err := i.processJupiterSwap(ctx, pool, targetTable, enhancedDetails, source, payload.Slot, transactionID); err != nil {
			return err
		}
	}

	if events, hasEvents := enhancedDetails["events"].([]interface{}); hasEvents && len(events) > 0 {
		for _, eventRaw := range events {
			event, ok := eventRaw.(map[string]interface{})
			if !ok {
				continue
			}

			eventType, ok := event["type"].(string)
			if !ok || (eventType != "SWAP" && eventType != "JUPITER_SWAP") {
				continue
			}

			log.Info().Str("eventType", eventType).Msg("Found swap event, processing")
			if err := i.processSwapEvent(ctx, pool, targetTable, event, source, payload.Slot, transactionID); err != nil {
				log.Error().Err(err).Msg("Error processing swap event")

			}
		}
	}

	if tokenTransfers, hasTransfers := enhancedDetails["tokenTransfers"].([]interface{}); hasTransfers && len(tokenTransfers) > 0 {
		log.Info().Int("transferCount", len(tokenTransfers)).Msg("Found token transfers")

		for _, transferRaw := range tokenTransfers {
			if err := i.processTokenTransferWithMetadata(ctx, pool, targetTable, transferRaw, source, payload.Slot, transactionID, heliusAPIKey); err != nil {
				log.Error().Err(err).Msg("Error processing token transfer")

			}
		}
	}

	if nativeBalances, hasNative := enhancedDetails["nativeBalanceChanges"].([]interface{}); hasNative && len(nativeBalances) > 0 {
		log.Info().Int("nativeBalanceCount", len(nativeBalances)).Msg("Found native balance changes")

	}

	if tokenBalances, hasBalances := enhancedDetails["tokenBalances"].([]interface{}); hasBalances && len(tokenBalances) > 0 {
		log.Info().Int("balanceCount", len(tokenBalances)).Msg("Found token balances")

		for _, balanceRaw := range tokenBalances {
			if err := i.processTokenBalance(ctx, pool, targetTable, balanceRaw, source, payload.Slot, transactionID); err != nil {
				log.Error().Err(err).Msg("Error processing token balance")

			}
		}
	}

	if accounts, hasAccounts := enhancedDetails["accountData"].([]interface{}); hasAccounts && len(accounts) > 0 {
		log.Info().Int("accountCount", len(accounts)).Msg("Found account data")

		for _, accountRaw := range accounts {
			account, ok := accountRaw.(map[string]interface{})
			if !ok {
				continue
			}

			accountAddress, ok := account["account"].(string)
			if !ok || accountAddress == "" {
				continue
			}

			for _, tokenAddr := range i.Tokens {
				if strings.EqualFold(accountAddress, tokenAddr) {

					i.enhanceTokenMetadataInDB(ctx, pool, targetTable, tokenAddr, source, heliusAPIKey)
				}
			}
		}
	}

	if heliusAPIKey != "" {

		tokensNeedingMetadata := make([]string, 0)

		conn, err := pool.Acquire(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to acquire connection from pool")
			return nil
		}
		defer conn.Release()

		rows, err := conn.Query(ctx, fmt.Sprintf(`
			SELECT DISTINCT token_address 
			FROM %s 
			WHERE token_name IS NULL OR token_name = '' OR token_name = 'UNKNOWN' 
			   OR token_symbol IS NULL OR token_symbol = '' OR token_symbol = 'UNKNOWN'
		`, targetTable))
		if err != nil {
			log.Error().Err(err).Msg("Failed to query tokens needing metadata")
			return nil
		}
		defer rows.Close()

		for rows.Next() {
			var tokenAddr string
			if err := rows.Scan(&tokenAddr); err != nil {
				log.Error().Err(err).Msg("Failed to scan token address")
				continue
			}

			for _, t := range i.Tokens {
				if strings.EqualFold(t, tokenAddr) {
					tokensNeedingMetadata = append(tokensNeedingMetadata, tokenAddr)
					break
				}
			}
		}

		if len(tokensNeedingMetadata) > 0 {
			log.Info().Strs("tokens", tokensNeedingMetadata).Msg("Fetching metadata for tokens with missing info")

			metadataFetcher := NewTokenMetadataFetcher(heliusAPIKey)
			tokenMetadata := metadataFetcher.FetchMultipleTokenMetadata(ctx, tokensNeedingMetadata)

			if len(tokenMetadata) > 0 {
				tx, err := pool.Begin(ctx)
				if err != nil {
					log.Error().Err(err).Msg("Failed to begin transaction")
					return nil
				}
				defer tx.Rollback(ctx)

				for tokenAddr, metadata := range tokenMetadata {
					if metadata.Name == "" && metadata.Symbol == "" {
						continue
					}

					_, err = tx.Exec(ctx, fmt.Sprintf(`
						UPDATE %s 
						SET 
							token_name = CASE 
								WHEN token_name IS NULL OR token_name = '' OR token_name = 'UNKNOWN' THEN $1 
								ELSE token_name 
							END,
							token_symbol = CASE 
								WHEN token_symbol IS NULL OR token_symbol = '' OR token_symbol = 'UNKNOWN' THEN $2 
								ELSE token_symbol 
							END
						WHERE token_address = $3
					`, targetTable),
						metadata.Name, metadata.Symbol, tokenAddr,
					)

					if err != nil {
						log.Error().Err(err).Str("token", tokenAddr).Msg("Failed to update token metadata")
						continue
					}

					log.Info().
						Str("token", tokenAddr).
						Str("name", metadata.Name).
						Str("symbol", metadata.Symbol).
						Msg("Updated token metadata for all platforms")
				}

				if err := tx.Commit(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to commit metadata updates")
					return nil
				}
			}
		}
	}

	return nil
}

func (i *TokenPriceIndexer) processTokenTransferWithMetadata(ctx context.Context, pool *pgxpool.Pool, targetTable string, transferRaw interface{}, platform string, slot int64, transactionID string, heliusAPIKey string) error {
	transfer, ok := transferRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid token transfer data format")
	}

	mint, hasMint := transfer["mint"].(string)
	if !hasMint {
		return fmt.Errorf("missing mint address in token transfer")
	}

	tokenMatch := false
	for _, t := range i.Tokens {
		if strings.EqualFold(mint, t) {
			tokenMatch = true
			break
		}
	}

	if !tokenMatch {
		return nil
	}

	log.Info().
		Str("mint", mint).
		Str("platform", platform).
		Msg("Processing price data for tracked token")

	var tokenSymbol string
	if symbol, ok := transfer["tokenSymbol"].(string); ok && symbol != "" {
		tokenSymbol = symbol
	} else if symbol, ok := transfer["symbol"].(string); ok && symbol != "" {
		tokenSymbol = symbol
	}

	var tokenName string
	if name, ok := transfer["tokenName"].(string); ok && name != "" {
		tokenName = name
	} else if name, ok := transfer["name"].(string); ok && name != "" {
		tokenName = name
	}

	var amount float64
	if amountVal, ok := transfer["tokenAmount"].(float64); ok {
		amount = amountVal
	} else if amountVal, ok := transfer["amount"].(float64); ok {
		amount = amountVal
	}

	var priceUSD float64 = 0
	if usdValue, ok := transfer["usdValue"].(float64); ok {
		priceUSD = usdValue
	}

	if (tokenName == "" || tokenSymbol == "") && heliusAPIKey != "" {
		metadataFetcher := NewTokenMetadataFetcher(heliusAPIKey)
		metadata, err := metadataFetcher.FetchTokenMetadata(ctx, mint)
		if err != nil {
			log.Warn().Err(err).Str("token", mint).Msg("Failed to fetch token metadata from Helius DAS API")
		} else {

			if tokenName == "" {
				tokenName = metadata.Name
			}
			if tokenSymbol == "" {
				tokenSymbol = metadata.Symbol
			}

			log.Info().
				Str("token", mint).
				Str("name", tokenName).
				Str("symbol", tokenSymbol).
				Msg("Enhanced token metadata with fetched data")
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, fmt.Sprintf(`
        INSERT INTO %s (
            token_address, token_name, token_symbol, platform, 
            price_usd, price_sol, volume_24h, market_cap, liquidity,
            price_change_24h, total_supply, transaction_id, updated_at, slot
        ) VALUES (
            $1, $2, $3, $4, $5, $6, NULL, NULL, NULL, NULL, NULL, $7, NOW(), $8
        ) ON CONFLICT (token_address, platform) 
        DO UPDATE SET 
            token_name = CASE WHEN (EXCLUDED.token_name != '' AND EXCLUDED.token_name != 'UNKNOWN') 
                THEN (CASE WHEN (%s.token_name IS NULL OR %s.token_name = '' OR %s.token_name = 'UNKNOWN') 
                    THEN EXCLUDED.token_name ELSE %s.token_name END)
                ELSE %s.token_name END,
            token_symbol = CASE WHEN (EXCLUDED.token_symbol != '' AND EXCLUDED.token_symbol != 'UNKNOWN') 
                THEN (CASE WHEN (%s.token_symbol IS NULL OR %s.token_symbol = '' OR %s.token_symbol = 'UNKNOWN') 
                    THEN EXCLUDED.token_symbol ELSE %s.token_symbol END)
                ELSE %s.token_symbol END,
            price_usd = CASE WHEN EXCLUDED.price_usd > 0 THEN EXCLUDED.price_usd ELSE %s.price_usd END,
            price_sol = CASE WHEN EXCLUDED.price_sol > 0 THEN EXCLUDED.price_sol ELSE %s.price_sol END,
            updated_at = CASE WHEN EXCLUDED.slot > %s.slot THEN NOW() ELSE %s.updated_at END,
            slot = GREATEST(EXCLUDED.slot, %s.slot),
            transaction_id = CASE WHEN EXCLUDED.slot > %s.slot THEN EXCLUDED.transaction_id ELSE %s.transaction_id END
    `, targetTable,
		targetTable, targetTable, targetTable, targetTable, targetTable,
		targetTable, targetTable, targetTable, targetTable, targetTable,
		targetTable, targetTable, targetTable, targetTable, targetTable, targetTable, targetTable),
		mint, tokenName, tokenSymbol, platform,
		priceUSD, amount, transactionID, slot,
	)

	if err != nil {
		return fmt.Errorf("failed to insert/update token price: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().
		Str("token", mint).
		Str("symbol", tokenSymbol).
		Str("platform", platform).
		Float64("price_usd", priceUSD).
		Float64("price_sol", amount).
		Int64("slot", slot).
		Msg("Successfully updated token price data from transfer")

	return nil
}

func (i *TokenPriceIndexer) enhanceTokenMetadataInDB(ctx context.Context, pool *pgxpool.Pool, targetTable string, tokenAddress string, platform string, heliusAPIKey string) {

	if heliusAPIKey == "" {
		return
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to acquire connection from pool")
		return
	}
	defer conn.Release()

	var name, symbol string
	err = conn.QueryRow(ctx, fmt.Sprintf(`
		SELECT token_name, token_symbol 
		FROM %s 
		WHERE token_address = $1 AND platform = $2
	`, targetTable), tokenAddress, platform).Scan(&name, &symbol)

	if err == nil && name != "" && name != "UNKNOWN" && symbol != "" && symbol != "UNKNOWN" {
		return
	}

	metadataFetcher := NewTokenMetadataFetcher(heliusAPIKey)
	metadata, err := metadataFetcher.FetchTokenMetadata(ctx, tokenAddress)
	if err != nil {
		log.Warn().Err(err).Str("token", tokenAddress).Msg("Failed to fetch token metadata from Helius DAS API")
		return
	}

	if metadata.Name != "" || metadata.Symbol != "" {
		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to begin transaction")
			return
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s 
			SET 
				token_name = CASE 
					WHEN token_name IS NULL OR token_name = '' OR token_name = 'UNKNOWN' THEN $1 
					ELSE token_name 
				END,
				token_symbol = CASE 
					WHEN token_symbol IS NULL OR token_symbol = '' OR token_symbol = 'UNKNOWN' THEN $2 
					ELSE token_symbol 
				END
			WHERE token_address = $3 AND platform = $4
		`, targetTable),
			metadata.Name, metadata.Symbol, tokenAddress, platform,
		)

		if err != nil {
			log.Error().Err(err).Str("token", tokenAddress).Msg("Failed to update token metadata")
			return
		}

		if err := tx.Commit(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to commit metadata updates")
			return
		}

		log.Info().
			Str("token", tokenAddress).
			Str("platform", platform).
			Str("name", metadata.Name).
			Str("symbol", metadata.Symbol).
			Msg("Updated token metadata in database")
	}
}

func (i *TokenPriceIndexer) EnrichTokenMetadata(ctx context.Context, pool *pgxpool.Pool, targetTable string, heliusAPIKey string) error {
	if heliusAPIKey == "" {
		return nil
	}

	metadataFetcher := NewTokenMetadataFetcher(heliusAPIKey)

	tokenMetadata := metadataFetcher.FetchMultipleTokenMetadata(ctx, i.Tokens)

	if len(tokenMetadata) == 0 {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for tokenAddr, metadata := range tokenMetadata {
		if metadata.Name == "" && metadata.Symbol == "" {
			continue
		}

		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s 
			SET 
				token_name = CASE 
					WHEN token_name IS NULL OR token_name = '' OR token_name = 'UNKNOWN' THEN $1 
					ELSE token_name 
				END,
				token_symbol = CASE 
					WHEN token_symbol IS NULL OR token_symbol = '' OR token_symbol = 'UNKNOWN' THEN $2 
					ELSE token_symbol 
				END
			WHERE token_address = $3
		`, targetTable),
			metadata.Name, metadata.Symbol, tokenAddr,
		)

		if err != nil {
			log.Error().Err(err).Str("token", tokenAddr).Msg("Failed to update token metadata")
			continue
		}

		log.Info().
			Str("token", tokenAddr).
			Str("name", metadata.Name).
			Str("symbol", metadata.Symbol).
			Msg("Updated token metadata for all platforms")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
