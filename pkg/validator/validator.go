package validator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func IsValidEmail(email string) bool {

	rx := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return rx.MatchString(email)
}

func IsValidPassword(password string) bool {

	if len(password) < 8 {
		return false
	}

	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)

	return hasLetter && hasNumber
}

func IsValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

func ValidateDBCredentials(host string, port int, dbName, username, password string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if strings.TrimSpace(dbName) == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty")
	}

	return nil
}

func IsValidJSON(jsonStr string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(jsonStr), &js) == nil
}

func ValidateAndUnmarshal(jsonStr string, result interface{}) error {
	err := json.Unmarshal([]byte(jsonStr), result)
	if err != nil {
		log.Error().Err(err).Str("json", jsonStr).Msg("Failed to unmarshal JSON")
		return fmt.Errorf("invalid JSON format: %w", err)
	}
	return nil
}

func IsValidSolanaAddress(address string) bool {

	matched, err := regexp.MatchString(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`, address)
	if err != nil {
		log.Error().Err(err).Msg("Error matching Solana address regex")
		return false
	}
	return matched
}

func IsValidTableName(tableName string) bool {

	matched, err := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9_]*$`, tableName)
	if err != nil {
		log.Error().Err(err).Msg("Error matching table name regex")
		return false
	}
	return matched && len(tableName) <= 63
}

func ValidateIndexerParams(indexerType string, paramsJson []byte) error {

	if !IsValidJSON(string(paramsJson)) {
		return fmt.Errorf("invalid JSON format for params")
	}

	switch indexerType {
	case "nft_bids":
		var params struct {
			Collection string `json:"collection"`
		}
		if err := json.Unmarshal(paramsJson, &params); err != nil {
			return fmt.Errorf("invalid NFT bid parameters: %w", err)
		}
		if params.Collection == "" {
			return fmt.Errorf("collection address is required for NFT bid indexing")
		}
		if !IsValidSolanaAddress(params.Collection) {
			return fmt.Errorf("invalid collection address format")
		}

	case "nft_prices":
		var params struct {
			Collection string `json:"collection"`
		}
		if err := json.Unmarshal(paramsJson, &params); err != nil {
			return fmt.Errorf("invalid NFT price parameters: %w", err)
		}
		if params.Collection == "" {
			return fmt.Errorf("collection address is required for NFT price indexing")
		}
		if !IsValidSolanaAddress(params.Collection) {
			return fmt.Errorf("invalid collection address format")
		}

	case "token_borrow":
		var params struct {
			Tokens []string `json:"tokens"`
		}
		if err := json.Unmarshal(paramsJson, &params); err != nil {
			return fmt.Errorf("invalid token borrow parameters: %w", err)
		}
		if len(params.Tokens) == 0 {
			return fmt.Errorf("at least one token address is required for token borrow indexing")
		}
		for _, token := range params.Tokens {
			if !IsValidSolanaAddress(token) {
				return fmt.Errorf("invalid token address format: %s", token)
			}
		}

	case "token_prices":
		var params struct {
			Tokens []string `json:"tokens"`
		}
		if err := json.Unmarshal(paramsJson, &params); err != nil {
			return fmt.Errorf("invalid token price parameters: %w", err)
		}
		if len(params.Tokens) == 0 {
			return fmt.Errorf("at least one token address is required for token price indexing")
		}
		for _, token := range params.Tokens {
			if !IsValidSolanaAddress(token) {
				return fmt.Errorf("invalid token address format: %s", token)
			}
		}

	default:
		return fmt.Errorf("unsupported indexer type: %s", indexerType)
	}

	return nil
}
