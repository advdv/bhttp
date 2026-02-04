package blwa

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
)

// mockSecretReader implements SecretReader for testing.
type mockSecretReader struct {
	secrets map[string]string
	err     error
}

func (m *mockSecretReader) GetSecretString(_ context.Context, secretID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	secret, ok := m.secrets[secretID]
	if !ok {
		return "", errors.Errorf("secret %q not found", secretID)
	}
	return secret, nil
}

func TestRuntime_Secret(t *testing.T) {
	tests := []struct {
		name      string
		secrets   map[string]string
		readerErr error
		secretID  string
		jsonPath  []string
		want      string
		wantErr   string
	}{
		{
			name: "read raw string secret",
			secrets: map[string]string{
				"my-api-key": "secret-key-value",
			},
			secretID: "my-api-key",
			jsonPath: nil,
			want:     "secret-key-value",
		},
		{
			name: "read JSON secret with simple path",
			secrets: map[string]string{
				"my-db-creds": `{"database": {"password": "secret123"}}`,
			},
			secretID: "my-db-creds",
			jsonPath: []string{"database.password"},
			want:     "secret123",
		},
		{
			name: "read JSON secret with nested array",
			secrets: map[string]string{
				"my-config": `{"items": [{"name": "first"}, {"name": "second"}]}`,
			},
			secretID: "my-config",
			jsonPath: []string{"items.1.name"},
			want:     "second",
		},
		{
			name: "path not found in JSON secret",
			secrets: map[string]string{
				"my-secret": `{"foo": "bar"}`,
			},
			secretID: "my-secret",
			jsonPath: []string{"missing.path"},
			wantErr:  `secret path "missing.path" not found`,
		},
		{
			name:      "secret reader error",
			secrets:   map[string]string{},
			readerErr: errors.New("AWS error"),
			secretID:  "any-secret",
			jsonPath:  nil,
			wantErr:   "AWS error",
		},
		{
			name: "too many jsonPath arguments",
			secrets: map[string]string{
				"my-secret": `{"foo": "bar"}`,
			},
			secretID: "my-secret",
			jsonPath: []string{"one", "two"},
			wantErr:  "at most one jsonPath argument",
		},
		{
			name: "read numeric value from JSON as string",
			secrets: map[string]string{
				"my-config": `{"port": 5432}`,
			},
			secretID: "my-config",
			jsonPath: []string{"port"},
			want:     "5432",
		},
		{
			name: "empty jsonPath returns raw secret",
			secrets: map[string]string{
				"my-secret": `{"foo": "bar"}`,
			},
			secretID: "my-secret",
			jsonPath: []string{""},
			want:     `{"foo": "bar"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &mockSecretReader{
				secrets: tt.secrets,
				err:     tt.readerErr,
			}

			rt := &Runtime[BaseEnvironment]{secretReader: reader}
			ctx := context.Background()

			got, err := rt.Secret(ctx, tt.secretID, tt.jsonPath...)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !containsSubstr(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntime_Secret_NoReaderConfigured(t *testing.T) {
	rt := &Runtime[BaseEnvironment]{secretReader: nil}
	ctx := context.Background()

	_, err := rt.Secret(ctx, "any-secret")
	if err == nil {
		t.Fatal("expected error when secret reader not configured")
	}
	if !containsSubstr(err.Error(), "secret reader not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSecretFromReader(t *testing.T) {
	tests := []struct {
		name     string
		secrets  map[string]string
		secretID string
		jsonPath []string
		want     string
		wantErr  string
	}{
		{
			name: "raw string secret",
			secrets: map[string]string{
				"my-secret": "raw-value",
			},
			secretID: "my-secret",
			jsonPath: nil,
			want:     "raw-value",
		},
		{
			name: "JSON secret with path",
			secrets: map[string]string{
				"my-json": `{"key": "value"}`,
			},
			secretID: "my-json",
			jsonPath: []string{"key"},
			want:     "value",
		},
		{
			name: "secret not found",
			secrets: map[string]string{
				"other": "value",
			},
			secretID: "missing",
			jsonPath: nil,
			wantErr:  `secret "missing" not found`,
		},
		{
			name: "too many jsonPath arguments",
			secrets: map[string]string{
				"my-secret": "value",
			},
			secretID: "my-secret",
			jsonPath: []string{"one", "two"},
			wantErr:  "at most one jsonPath argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &mockSecretReader{secrets: tt.secrets}
			ctx := context.Background()

			got, err := secretFromReader(ctx, reader, tt.secretID, tt.jsonPath...)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !containsSubstr(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
