package blwa

import (
	"github.com/advdv/bhttp"
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

type zapLogger struct{ *zap.Logger }

func (l zapLogger) LogUnhandledServeError(err error) {
	l.Logger.Error("unhandled server error", zap.Error(err))
}

func (l zapLogger) LogImplicitFlushError(err error) {
	l.Logger.Error("error while flushing implicitly", zap.Error(err))
}

func newZapBHTTPLogger(l *zap.Logger) bhttp.Logger {
	return zapLogger{l.Named("bhttp").Named("blwa")}
}
