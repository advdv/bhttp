package blwatest

import (
	"strconv"
	"testing"
)

// Env provides a chainable builder for setting [blwa.BaseEnvironment] env vars
// via t.Setenv. Create one with [SetBaseEnv].
type Env struct {
	t testing.TB
}

// SetBaseEnv sets all [blwa.BaseEnvironment] env vars to sensible test defaults.
// Port is required because each test must use a unique port to avoid collisions.
//
// Defaults:
//   - BW_SERVICE_NAME: "test"
//   - AWS_LWA_READINESS_CHECK_PATH: "/health"
//   - AWS_REGION: "us-east-1"
//   - BW_PRIMARY_REGION: "eu-west-1"
//   - BW_LAMBDA_TIMEOUT: "30s"
//   - AWS_LWA_ERROR_STATUS_CODES: "500-599"
//   - OTEL_SDK_DISABLED: "true"
//   - AWS_ACCESS_KEY_ID: "test"
//   - AWS_SECRET_ACCESS_KEY: "test"
//
// Use the returned [Env] to override individual values:
//
//	blwatest.SetBaseEnv(t, 18085).AWSRegion("eu-west-1").PrimaryRegion("eu-central-1")
func SetBaseEnv(t testing.TB, port int) *Env {
	t.Helper()
	t.Setenv("AWS_LWA_PORT", strconv.Itoa(port))
	t.Setenv("BW_SERVICE_NAME", "test")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/health")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("BW_PRIMARY_REGION", "eu-west-1")
	t.Setenv("BW_LAMBDA_TIMEOUT", "30s")
	t.Setenv("AWS_LWA_ERROR_STATUS_CODES", "500-599")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	return &Env{t: t}
}

// ServiceName overrides BW_SERVICE_NAME.
func (e *Env) ServiceName(name string) *Env {
	e.t.Helper()
	e.t.Setenv("BW_SERVICE_NAME", name)
	return e
}

// ReadinessCheckPath overrides AWS_LWA_READINESS_CHECK_PATH.
func (e *Env) ReadinessCheckPath(path string) *Env {
	e.t.Helper()
	e.t.Setenv("AWS_LWA_READINESS_CHECK_PATH", path)
	return e
}

// AWSRegion overrides AWS_REGION.
func (e *Env) AWSRegion(region string) *Env {
	e.t.Helper()
	e.t.Setenv("AWS_REGION", region)
	return e
}

// PrimaryRegion overrides BW_PRIMARY_REGION.
func (e *Env) PrimaryRegion(region string) *Env {
	e.t.Helper()
	e.t.Setenv("BW_PRIMARY_REGION", region)
	return e
}

// LambdaTimeout overrides BW_LAMBDA_TIMEOUT.
func (e *Env) LambdaTimeout(d string) *Env {
	e.t.Helper()
	e.t.Setenv("BW_LAMBDA_TIMEOUT", d)
	return e
}
