package blwa

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a zap logger configured from the environment.
// Uses JSON encoding suitable for CloudWatch.
// LOG_LEVEL controls the level (debug, info, warn, error).
func NewLogger(env Environment) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(env.logLevel())
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return cfg.Build()
}
