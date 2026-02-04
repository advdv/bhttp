package blwa

import (
	"github.com/caarlos0/env/v11"
	"github.com/cockroachdb/errors"
	"go.uber.org/zap/zapcore"
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

var _ Environment = BaseEnvironment{}

// ParseEnv parses environment variables into the given Environment type.
func ParseEnv[E Environment]() func() (E, error) {
	return func() (e E, err error) {
		if err := env.Parse(&e); err != nil {
			return e, errors.Wrap(err, "failed to parse environment")
		}
		return e, nil
	}
}
