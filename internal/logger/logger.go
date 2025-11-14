package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup initializes the global logger with pretty console output and timestamps
func Setup() {
	// Use pretty console writer for development
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	// Set global log level from environment (default: info)
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info", "":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// Get returns a new logger instance
func Get() *zerolog.Logger {
	return &log.Logger
}

// WithComponent returns a logger with a component field
func WithComponent(component string) zerolog.Logger {
	return log.Logger.With().Str("component", component).Logger()
}
