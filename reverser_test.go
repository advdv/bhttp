package bhttp_test

import (
	"testing"

	"github.com/advdv/bhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverser(t *testing.T) {
	rev := bhttp.NewReverser()

	t.Run("should allow naming patterns", func(t *testing.T) {
		s := rev.Named("homepage", "/{$}")
		assert.Equal(t, "/{$}", s)

		s, err := rev.NamedPattern("blog_post", "/blog/{id}/{$}")
		require.NoError(t, err)
		assert.Equal(t, "/blog/{id}/{$}", s)
	})

	t.Run("should reverse named patterns", func(t *testing.T) {
		res, err := rev.Reverse("homepage")
		require.NoError(t, err)
		assert.Equal(t, "/", res)
	})

	t.Run("should error if pattern already exists", func(t *testing.T) {
		_, err := rev.NamedPattern("homepage", "/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("should panic for Named error", func(t *testing.T) {
		assert.PanicsWithValue(t, "bhttp: failed to parse pattern: empty pattern", func() {
			rev.Named("bogus", "")
		})
	})

	t.Run("should error if reversing unknown name", func(t *testing.T) {
		_, err := rev.Reverse("bogus")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no pattern named: \"bogus\"")
	})

	t.Run("should error if url building fails", func(t *testing.T) {
		_, err := rev.Reverse("blog_post")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not enough values")
	})
}
