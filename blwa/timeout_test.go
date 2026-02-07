package blwa_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/blwa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeoutConfig_ServerTimeouts(t *testing.T) {
	defaultBuffer := blwa.DefaultDeadlineBuffer // 500ms

	tests := []struct {
		name                  string
		lambdaTimeout         time.Duration
		deadlineBuffer        time.Duration
		wantReadHeaderTimeout time.Duration
		wantReadTimeout       time.Duration
		wantWriteTimeout      time.Duration
		wantIdleTimeout       time.Duration
	}{
		{
			name:                  "short lambda timeout (3s) uses default buffer",
			lambdaTimeout:         3 * time.Second,
			deadlineBuffer:        0,                       // use default
			wantReadHeaderTimeout: 2500 * time.Millisecond, // 3s - 500ms buffer, capped at effective timeout
			wantReadTimeout:       2500 * time.Millisecond, // 3s - 500ms
			wantWriteTimeout:      2500 * time.Millisecond,
			wantIdleTimeout:       2500 * time.Millisecond,
		},
		{
			name:                  "typical lambda timeout (30s) uses default buffer",
			lambdaTimeout:         30 * time.Second,
			deadlineBuffer:        0,                              // use default
			wantReadHeaderTimeout: 5 * time.Second,                // capped at 5s
			wantReadTimeout:       30*time.Second - defaultBuffer, // 29.5s
			wantWriteTimeout:      30*time.Second - defaultBuffer,
			wantIdleTimeout:       30*time.Second - defaultBuffer,
		},
		{
			name:                  "max lambda timeout (15min) uses default buffer",
			lambdaTimeout:         15 * time.Minute,
			deadlineBuffer:        0,                              // use default
			wantReadHeaderTimeout: 5 * time.Second,                // capped at 5s
			wantReadTimeout:       15*time.Minute - defaultBuffer, // 14m59.5s
			wantWriteTimeout:      15*time.Minute - defaultBuffer,
			wantIdleTimeout:       15*time.Minute - defaultBuffer,
		},
		{
			name:                  "custom buffer (1s)",
			lambdaTimeout:         30 * time.Second,
			deadlineBuffer:        1 * time.Second,
			wantReadHeaderTimeout: 5 * time.Second, // capped at 5s
			wantReadTimeout:       29 * time.Second,
			wantWriteTimeout:      29 * time.Second,
			wantIdleTimeout:       29 * time.Second,
		},
		{
			name:                  "buffer equals timeout falls back to full timeout",
			lambdaTimeout:         500 * time.Millisecond,
			deadlineBuffer:        500 * time.Millisecond,
			wantReadHeaderTimeout: 500 * time.Millisecond, // fallback to lambda timeout
			wantReadTimeout:       500 * time.Millisecond,
			wantWriteTimeout:      500 * time.Millisecond,
			wantIdleTimeout:       500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := blwa.TimeoutConfig{
				LambdaTimeout:  tt.lambdaTimeout,
				DeadlineBuffer: tt.deadlineBuffer,
			}
			rht, rt, wt, it := tc.ServerTimeouts()

			assert.Equal(t, tt.wantReadHeaderTimeout, rht, "ReadHeaderTimeout")
			assert.Equal(t, tt.wantReadTimeout, rt, "ReadTimeout")
			assert.Equal(t, tt.wantWriteTimeout, wt, "WriteTimeout")
			assert.Equal(t, tt.wantIdleTimeout, it, "IdleTimeout")
		})
	}
}

func TestWithRequestDeadline(t *testing.T) {
	t.Run("no LWA context passes through unchanged", func(t *testing.T) {
		var hasDeadline bool
		var deadline time.Time

		handler := blwa.WithRequestDeadline(500 * time.Millisecond)(
			bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
				deadline, hasDeadline = r.Context().Deadline()
				return nil
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		rw := bhttp.NewResponseWriter(w, 0)

		err := handler.ServeBareBHTTP(rw, req)
		require.NoError(t, err)

		assert.False(t, hasDeadline, "should not have deadline without LWA context")
		assert.True(t, deadline.IsZero())
	})

	t.Run("with LWA context sets deadline", func(t *testing.T) {
		var hasDeadline bool
		var deadline time.Time
		buffer := 500 * time.Millisecond
		lambdaDeadline := time.Now().Add(10 * time.Second)

		lwaMiddleware := testLWAContextMiddleware(lambdaDeadline)
		deadlineMiddleware := blwa.WithRequestDeadline(buffer)

		handler := lwaMiddleware(deadlineMiddleware(
			bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
				deadline, hasDeadline = r.Context().Deadline()
				return nil
			}),
		))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		rw := bhttp.NewResponseWriter(w, 0)

		err := handler.ServeBareBHTTP(rw, req)
		require.NoError(t, err)

		assert.True(t, hasDeadline, "should have deadline with LWA context")

		expectedDeadline := lambdaDeadline.Add(-buffer)
		assert.WithinDuration(t, expectedDeadline, deadline, 100*time.Millisecond)
	})

	t.Run("past deadline does not set context deadline", func(t *testing.T) {
		var hasDeadline bool
		buffer := 500 * time.Millisecond
		pastDeadline := time.Now().Add(-1 * time.Second)

		lwaMiddleware := testLWAContextMiddleware(pastDeadline)
		deadlineMiddleware := blwa.WithRequestDeadline(buffer)

		handler := lwaMiddleware(deadlineMiddleware(
			bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
				_, hasDeadline = r.Context().Deadline()
				return nil
			}),
		))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		rw := bhttp.NewResponseWriter(w, 0)

		err := handler.ServeBareBHTTP(rw, req)
		require.NoError(t, err)

		assert.False(t, hasDeadline, "should not set deadline for past time")
	})

	t.Run("LWA deadline shorter than server timeout is respected", func(t *testing.T) {
		var hasDeadline bool
		var deadline time.Time
		buffer := 500 * time.Millisecond

		lwaDeadline := time.Now().Add(5 * time.Second)

		lwaMiddleware := testLWAContextMiddleware(lwaDeadline)
		deadlineMiddleware := blwa.WithRequestDeadline(buffer)

		handler := lwaMiddleware(deadlineMiddleware(
			bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
				deadline, hasDeadline = r.Context().Deadline()
				return nil
			}),
		))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		rw := bhttp.NewResponseWriter(w, 0)

		err := handler.ServeBareBHTTP(rw, req)
		require.NoError(t, err)

		assert.True(t, hasDeadline, "should have deadline from LWA context")

		expectedDeadline := lwaDeadline.Add(-buffer)
		assert.WithinDuration(t, expectedDeadline, deadline, 100*time.Millisecond)

		remaining := time.Until(deadline)
		assert.Less(t, remaining, 5*time.Second, "deadline should be less than 5s (LWA deadline)")
		assert.Greater(t, remaining, 4*time.Second, "deadline should be greater than 4s (LWA - buffer)")
	})

	t.Run("default buffer is used when zero", func(t *testing.T) {
		var hasDeadline bool
		var deadline time.Time
		lambdaDeadline := time.Now().Add(10 * time.Second)

		lwaMiddleware := testLWAContextMiddleware(lambdaDeadline)
		deadlineMiddleware := blwa.WithRequestDeadline(0)

		handler := lwaMiddleware(deadlineMiddleware(
			bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
				deadline, hasDeadline = r.Context().Deadline()
				return nil
			}),
		))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		rw := bhttp.NewResponseWriter(w, 0)

		err := handler.ServeBareBHTTP(rw, req)
		require.NoError(t, err)

		assert.True(t, hasDeadline)

		expectedDeadline := lambdaDeadline.Add(-blwa.DefaultDeadlineBuffer)
		assert.WithinDuration(t, expectedDeadline, deadline, 100*time.Millisecond)
	})
}

func TestRequestDeadline(t *testing.T) {
	t.Run("returns zero time when no deadline", func(t *testing.T) {
		ctx := context.Background()
		deadline, ok := blwa.RequestDeadline(ctx)
		assert.False(t, ok)
		assert.True(t, deadline.IsZero())
	})

	t.Run("returns deadline when set", func(t *testing.T) {
		expectedDeadline := time.Now().Add(5 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), expectedDeadline)
		defer cancel()

		deadline, ok := blwa.RequestDeadline(ctx)
		assert.True(t, ok)
		assert.WithinDuration(t, expectedDeadline, deadline, time.Millisecond)
	})
}

func TestRequestRemainingTime(t *testing.T) {
	t.Run("returns zero when no deadline", func(t *testing.T) {
		ctx := context.Background()
		remaining := blwa.RequestRemainingTime(ctx)
		assert.Equal(t, time.Duration(0), remaining)
	})

	t.Run("returns zero when deadline passed", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
		defer cancel()

		remaining := blwa.RequestRemainingTime(ctx)
		assert.Equal(t, time.Duration(0), remaining)
	})

	t.Run("returns remaining time when deadline in future", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		remaining := blwa.RequestRemainingTime(ctx)
		assert.Greater(t, remaining, 4*time.Second)
		assert.LessOrEqual(t, remaining, 5*time.Second)
	})
}
