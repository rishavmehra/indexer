package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Helius   HeliusConfig
	Logger   LoggerConfig
}

type ServerConfig struct {
	Port string
	Env  string
}

type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MigrationURL string
}

type JWTConfig struct {
	Secret    string
	ExpiresIn time.Duration
}

type HeliusConfig struct {
	APIKey         string
	WebhookSecret  string
	WebhookBaseURL string
	WebhookID      string
}

type LoggerConfig struct {
	Level string
}

func LoadConfig(path string) (config Config, err error) {
	_ = godotenv.Load(path)

	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("SERVER_ENV", "development")
	viper.SetDefault("JWT_EXPIRES_IN", "24h")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("MIGRATION_PATH", "file://internal/db/migrations")
	viper.SetDefault("HELIUS_WEBHOOK_ID", "")

	viper.AutomaticEnv()

	jwtExpiresIn, err := time.ParseDuration(viper.GetString("JWT_EXPIRES_IN"))
	if err != nil {
		return config, fmt.Errorf("invalid JWT_EXPIRES_IN: %w", err)
	}

	config = Config{
		Server: ServerConfig{
			Port: viper.GetString("SERVER_PORT"),
			Env:  viper.GetString("SERVER_ENV"),
		},
		Database: DatabaseConfig{
			Host:         viper.GetString("DB_HOST"),
			Port:         viper.GetString("DB_PORT"),
			User:         viper.GetString("DB_USER"),
			Password:     viper.GetString("DB_PASSWORD"),
			DBName:       viper.GetString("DB_NAME"),
			SSLMode:      viper.GetString("DB_SSL_MODE"),
			MigrationURL: viper.GetString("MIGRATION_PATH"),
		},
		JWT: JWTConfig{
			Secret:    viper.GetString("JWT_SECRET"),
			ExpiresIn: jwtExpiresIn,
		},
		Helius: HeliusConfig{
			APIKey:         viper.GetString("HELIUS_API_KEY"),
			WebhookSecret:  viper.GetString("HELIUS_WEBHOOK_SECRET"),
			WebhookBaseURL: viper.GetString("HELIUS_WEBHOOK_BASE_URL"),
			WebhookID:      viper.GetString("HELIUS_WEBHOOK_ID"),
		},
		Logger: LoggerConfig{
			Level: viper.GetString("LOG_LEVEL"),
		},
	}

	if config.JWT.Secret == "" {
		return config, fmt.Errorf("JWT_SECRET is required")
	}
	if config.Database.Host == "" || config.Database.User == "" || config.Database.DBName == "" {
		return config, fmt.Errorf("database configuration is incomplete")
	}
	if config.Helius.APIKey == "" {
		return config, fmt.Errorf("HELIUS_API_KEY is required")
	}

	return config, nil
}

func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}
