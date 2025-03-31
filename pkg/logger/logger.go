package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func SetupLogger(level string) {

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	output.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}

	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("  %s  ", i)
	}

	output.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}

	output.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}

	logLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	log.Logger = zerolog.New(output).
		Level(logLevel).
		With().
		Timestamp().
		Caller().
		Logger()
}

func GetLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}
