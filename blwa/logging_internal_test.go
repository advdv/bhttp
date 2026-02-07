package blwa

import (
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type testEnv struct {
	level   zapcore.Level
	otelExp string
}

func (e testEnv) port() int                  { return 8080 }
func (e testEnv) serviceName() string        { return "test" }
func (e testEnv) readinessCheckPath() string { return "/health" }
func (e testEnv) logLevel() zapcore.Level    { return e.level }
func (e testEnv) otelExporter() string {
	if e.otelExp == "" {
		return "stdout"
	}
	return e.otelExp
}
func (e testEnv) awsRegion() string             { return "us-east-1" }
func (e testEnv) primaryRegion() string         { return "us-east-1" }
func (e testEnv) gatewayAccessLogGroup() string { return "" }
func (e testEnv) lambdaTimeout() time.Duration  { return 30 * time.Second }
func (e testEnv) errorStatusCodes() string      { return "500-599" }

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		env     testEnv
		wantErr bool
	}{
		{
			name:    "info level",
			env:     testEnv{level: zapcore.InfoLevel},
			wantErr: false,
		},
		{
			name:    "debug level",
			env:     testEnv{level: zapcore.DebugLevel},
			wantErr: false,
		},
		{
			name:    "warn level",
			env:     testEnv{level: zapcore.WarnLevel},
			wantErr: false,
		},
		{
			name:    "error level",
			env:     testEnv{level: zapcore.ErrorLevel},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.env)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if logger == nil {
				t.Error("NewLogger() returned nil logger")
			}
		})
	}
}

func TestBaseEnvironment_LogLevel_Parsing(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantLevel zapcore.Level
	}{
		{"debug", "debug", zapcore.DebugLevel},
		{"info", "info", zapcore.InfoLevel},
		{"warn", "warn", zapcore.WarnLevel},
		{"error", "error", zapcore.ErrorLevel},
		{"DEBUG uppercase", "DEBUG", zapcore.DebugLevel},
		{"INFO uppercase", "INFO", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWS_LWA_PORT", "8080")
			t.Setenv("BW_SERVICE_NAME", "test")
			t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/health")
			t.Setenv("BW_LOG_LEVEL", tt.envValue)
			t.Setenv("AWS_REGION", "us-east-1")
			t.Setenv("BW_PRIMARY_REGION", "us-east-1")
			t.Setenv("BW_LAMBDA_TIMEOUT", "30s")
			t.Setenv("AWS_LWA_ERROR_STATUS_CODES", "500-599")

			parse := ParseEnv[BaseEnvironment]()
			env, err := parse()
			if err != nil {
				t.Fatalf("ParseEnv() error = %v", err)
			}

			if env.LogLevel != tt.wantLevel {
				t.Errorf("LogLevel = %v, want %v", env.LogLevel, tt.wantLevel)
			}
		})
	}
}

func TestZapLogger(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := newZapBHTTPLogger(zap.New(core))

	t.Run("unhandled serve error", func(t *testing.T) {
		err := errors.New("test serve error")
		logger.LogUnhandledServeError(err)

		entries := logs.TakeAll()
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}
		if entries[0].Message != "unhandled server error" {
			t.Errorf("unexpected message: %s", entries[0].Message)
		}
		if entries[0].LoggerName != "bhttp.blwa" {
			t.Errorf("unexpected logger name: %s", entries[0].LoggerName)
		}
		if entries[0].Level != zapcore.ErrorLevel {
			t.Errorf("unexpected level: %s", entries[0].Level)
		}
	})

	t.Run("implicit flush error", func(t *testing.T) {
		err := errors.New("test flush error")
		logger.LogImplicitFlushError(err)

		entries := logs.TakeAll()
		if len(entries) != 1 {
			t.Fatalf("expected 1 log entry, got %d", len(entries))
		}
		if entries[0].Message != "error while flushing implicitly" {
			t.Errorf("unexpected message: %s", entries[0].Message)
		}
		if entries[0].LoggerName != "bhttp.blwa" {
			t.Errorf("unexpected logger name: %s", entries[0].LoggerName)
		}
		if entries[0].Level != zapcore.ErrorLevel {
			t.Errorf("unexpected level: %s", entries[0].Level)
		}
	})
}

func TestBaseEnvironment_LogLevel_Default(t *testing.T) {
	t.Setenv("AWS_LWA_PORT", "8080")
	t.Setenv("BW_SERVICE_NAME", "test")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/health")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("BW_PRIMARY_REGION", "us-east-1")
	t.Setenv("BW_LAMBDA_TIMEOUT", "30s")
	t.Setenv("AWS_LWA_ERROR_STATUS_CODES", "500-599")

	parse := ParseEnv[BaseEnvironment]()
	env, err := parse()
	if err != nil {
		t.Fatalf("ParseEnv() error = %v", err)
	}

	if env.LogLevel != zapcore.InfoLevel {
		t.Errorf("LogLevel default = %v, want %v", env.LogLevel, zapcore.InfoLevel)
	}
}
