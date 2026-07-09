// Package logger provides the Agent structured logging contract.
package logger

import (
	"fmt"
	"io"
	"strings"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
	"github.com/rs/zerolog"
)

// New builds the foundation zerolog logger from validated Agent config.
func New(cfg config.LoggerConfig, out io.Writer) (zerolog.Logger, error) {
	if out == nil {
		return zerolog.Logger{}, fmt.Errorf("logger output cannot be nil")
	}
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format != JSONFormat && format != ConsoleFormat {
		return zerolog.Logger{}, fmt.Errorf("unsupported logger format %q", cfg.Format)
	}

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return zerolog.Logger{}, err
	}

	destination := out
	if format == ConsoleFormat {
		destination = NewConsoleWriter(out)
	}

	base := zerolog.New(destination).Level(level)
	context := base.With().Str("service", strings.TrimSpace(cfg.Service))
	if cfg.Timestamp {
		context = context.Timestamp()
	}

	return context.Logger(), nil
}
