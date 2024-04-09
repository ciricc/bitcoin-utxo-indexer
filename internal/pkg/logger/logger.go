package logger

import (
	"log/slog"
	"os"

	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/rs/zerolog"
)

func NewSlogLogger(cfg *config.Config) *slog.Logger {
	return slog.Default()
}

func NewLogger(cfg *config.Config) zerolog.Logger {
	return zerolog.New(os.Stdout).With().
		Timestamp().
		Str("serviceName", cfg.Name).
		Str("ver", cfg.Version).
		Str("env", cfg.Environment).
		Caller().
		Logger()
}
