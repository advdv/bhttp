package blwa_test

import (
	"testing"

	"github.com/advdv/bhttp/blwa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateErrorStatusCodes(t *testing.T) {
	t.Run("valid single codes", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("500,504", 500, 504)
		require.NoError(t, err)
	})

	t.Run("valid range covering all required", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("500-599", 500, 504)
		require.NoError(t, err)
	})

	t.Run("valid mixed format", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("500,502-505", 500, 504)
		require.NoError(t, err)
	})

	t.Run("valid with extra codes", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("400,500-599", 500, 504)
		require.NoError(t, err)
	})

	t.Run("missing 500", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("502-504", 500, 504)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing: [500]")
		assert.Contains(t, err.Error(), "recommended value: \"500-599\"")
	})

	t.Run("missing 504", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("500-503", 500, 504)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing: [504]")
	})

	t.Run("missing both 500 and 504", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("502-503", 500, 504)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
		assert.Contains(t, err.Error(), "504")
	})

	t.Run("empty string fails parsing", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("", 500, 504)
		require.Error(t, err)
		// Empty expressions are rejected by the library as invalid
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("invalid format fails parsing", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("not-a-number", 500, 504)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("no required codes always passes", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("500")
		require.NoError(t, err)
	})

	t.Run("custom required codes", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("400-499", 400, 404)
		require.NoError(t, err)
	})

	t.Run("custom required codes missing", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("400-403", 400, 404)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing: [404]")
	})

	t.Run("open-ended range", func(t *testing.T) {
		// "500-" means 500 and above (infinite range)
		err := blwa.ValidateErrorStatusCodes("500-", 500, 504, 599)
		require.NoError(t, err)
	})

	t.Run("multiple separate ranges", func(t *testing.T) {
		err := blwa.ValidateErrorStatusCodes("500,502-503,504", 500, 504)
		require.NoError(t, err)
	})

	t.Run("real-world AWS LWA format", func(t *testing.T) {
		// Typical recommended configuration
		err := blwa.ValidateErrorStatusCodes("500-599", blwa.DefaultRequiredErrorStatusCodes...)
		require.NoError(t, err)
	})

	t.Run("minimal valid configuration", func(t *testing.T) {
		// Absolute minimum required
		err := blwa.ValidateErrorStatusCodes("500,504,507", blwa.DefaultRequiredErrorStatusCodes...)
		require.NoError(t, err)
	})
}

func TestDefaultRequiredErrorStatusCodes(t *testing.T) {
	assert.Contains(t, blwa.DefaultRequiredErrorStatusCodes, 500)
	assert.Contains(t, blwa.DefaultRequiredErrorStatusCodes, 504)
	assert.Contains(t, blwa.DefaultRequiredErrorStatusCodes, 507)
	assert.Len(t, blwa.DefaultRequiredErrorStatusCodes, 3)
}
