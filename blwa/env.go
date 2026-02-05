package blwa

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/cockroachdb/errors"
	"go.uber.org/zap/zapcore"

	intervals "github.com/MawKKe/integer-interval-expressions-go"
)

// Environment defines the interface that all environment configurations must implement.
// Embed BaseEnvironment in your struct to satisfy this interface.
type Environment interface {
	port() int
	serviceName() string
	readinessCheckPath() string
	logLevel() zapcore.Level
	otelExporter() string
	awsRegion() string
	primaryRegion() string
	gatewayAccessLogGroup() string
	lambdaTimeout() time.Duration
	errorStatusCodes() string
}

// BaseEnvironment contains the required LWA environment variables.
// Embed this in your custom environment struct.
type BaseEnvironment struct {
	Port               int           `env:"AWS_LWA_PORT,required"`
	ServiceName        string        `env:"BW_SERVICE_NAME,required"`
	ReadinessCheckPath string        `env:"AWS_LWA_READINESS_CHECK_PATH,required"`
	LogLevel           zapcore.Level `env:"BW_LOG_LEVEL" envDefault:"info"`
	OtelExporter       string        `env:"BW_OTEL_EXPORTER" envDefault:"stdout"`
	AWSRegion          string        `env:"AWS_REGION,required"`
	PrimaryRegion      string        `env:"BW_PRIMARY_REGION,required"`
	// GatewayAccessLogGroup is the CloudWatch Log Group name for API Gateway
	// access logs. When set, traces include this log group for X-Ray log
	// correlation. Injected automatically by bwcdkrestgateway.
	GatewayAccessLogGroup string `env:"BW_GATEWAY_ACCESS_LOG_GROUP"`
	// LambdaTimeout is the configured Lambda function timeout. Used to configure
	// HTTP server timeouts. Should match the Lambda function's timeout setting.
	LambdaTimeout time.Duration `env:"BW_LAMBDA_TIMEOUT,required"`
	// ErrorStatusCodes is the raw AWS_LWA_ERROR_STATUS_CODES value. Lambda Web Adapter
	// uses this to determine which HTTP status codes indicate Lambda function errors.
	// The value supports comma-separated ranges (e.g., "500-599" or "500,502-504").
	// This is critical for correct error handling:
	//   - SQS/event-driven: Without proper error codes, failed messages are deleted
	//     instead of being retried, causing silent data loss.
	//   - API Gateway: Enables accurate Lambda error metrics in CloudWatch.
	// Validated at startup to ensure it includes 500 (general errors) and 504 (timeouts).
	ErrorStatusCodes string `env:"AWS_LWA_ERROR_STATUS_CODES,required"`
}

func (e BaseEnvironment) port() int {
	return e.Port
}

func (e BaseEnvironment) serviceName() string {
	return e.ServiceName
}

func (e BaseEnvironment) readinessCheckPath() string {
	return e.ReadinessCheckPath
}

func (e BaseEnvironment) logLevel() zapcore.Level {
	return e.LogLevel
}

func (e BaseEnvironment) otelExporter() string {
	return e.OtelExporter
}

func (e BaseEnvironment) awsRegion() string {
	return e.AWSRegion
}

func (e BaseEnvironment) primaryRegion() string {
	return e.PrimaryRegion
}

func (e BaseEnvironment) gatewayAccessLogGroup() string {
	return e.GatewayAccessLogGroup
}

func (e BaseEnvironment) lambdaTimeout() time.Duration {
	return e.LambdaTimeout
}

func (e BaseEnvironment) errorStatusCodes() string {
	return e.ErrorStatusCodes
}

var _ Environment = BaseEnvironment{}

// DefaultRequiredErrorStatusCodes are the HTTP status codes that must be present in
// AWS_LWA_ERROR_STATUS_CODES for correct Lambda error handling.
//
// 500 (Internal Server Error): Catches general application errors. Without this,
// unhandled exceptions would be treated as successful responses.
//
// 504 (Gateway Timeout): Catches timeout errors from the WithRequestDeadline
// middleware. When a request exceeds the Lambda deadline, the handler returns
// 504 to signal that the function ran out of time. This is critical because
// timeout errors should trigger retries for SQS/event sources.
//
// 507 (Insufficient Storage): Catches response buffer overflow errors. When a
// handler generates a response larger than the configured buffer limit, bhttp
// returns 507 to indicate the server cannot store the response representation.
// This helps identify handlers that need larger buffer limits or response streaming.
var DefaultRequiredErrorStatusCodes = []int{500, 504, 507}

// ParseEnv parses environment variables into the given Environment type.
func ParseEnv[E Environment]() func() (E, error) {
	return ParseEnvWithRequiredStatusCodes[E](DefaultRequiredErrorStatusCodes...)
}

// ParseEnvWithRequiredStatusCodes parses environment variables and validates that
// AWS_LWA_ERROR_STATUS_CODES contains the specified required status codes.
func ParseEnvWithRequiredStatusCodes[E Environment](requiredCodes ...int) func() (E, error) {
	return func() (e E, err error) {
		if err := env.Parse(&e); err != nil {
			return e, errors.Wrap(err, "failed to parse environment")
		}
		if err := ValidateErrorStatusCodes(e.errorStatusCodes(), requiredCodes...); err != nil {
			return e, err
		}
		return e, nil
	}
}

// ValidateErrorStatusCodes parses an AWS_LWA_ERROR_STATUS_CODES string and
// validates that it contains all required status codes.
//
// The format supports comma-separated values and ranges:
//   - Single codes: "500,502,504"
//   - Ranges: "500-599"
//   - Mixed: "500,502-504,599"
//
// Returns an error if the string cannot be parsed or if any required code is missing.
func ValidateErrorStatusCodes(errorStatusCodes string, requiredCodes ...int) error {
	expr, err := intervals.ParseExpression(errorStatusCodes)
	if err != nil {
		return errors.Wrapf(err, "failed to parse AWS_LWA_ERROR_STATUS_CODES %q", errorStatusCodes)
	}

	var missing []int
	for _, code := range requiredCodes {
		if !expr.Matches(code) {
			missing = append(missing, code)
		}
	}

	if len(missing) > 0 {
		return errors.Newf(
			"AWS_LWA_ERROR_STATUS_CODES %q must include status codes %v for correct Lambda error handling "+
				"(500 for general errors, 504 for timeouts); missing: %v; recommended value: \"500-599\"",
			errorStatusCodes, requiredCodes, missing,
		)
	}

	return nil
}

// FormatRequiredErrorStatusCodes formats the required status codes for display.
func FormatRequiredErrorStatusCodes(codes []int) string {
	return fmt.Sprintf("%v", codes)
}
