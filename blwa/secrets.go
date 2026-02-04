package blwa

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-secretsmanager-caching-go/v2/secretcache"
	"github.com/cockroachdb/errors"
	"github.com/tidwall/gjson"
)

// SecretReader abstracts secret retrieval for testability and flexibility.
type SecretReader interface {
	GetSecretString(ctx context.Context, secretID string) (string, error)
}

// AWSSecretReader implements SecretReader using AWS Secrets Manager caching client.
type AWSSecretReader struct {
	cache *secretcache.Cache
}

// NewAWSSecretReader creates a new AWSSecretReader using the provided AWS config.
func NewAWSSecretReader(cfg aws.Config) (*AWSSecretReader, error) {
	client := secretsmanager.NewFromConfig(cfg)
	cache, err := secretcache.New(
		func(c *secretcache.Cache) {
			c.Client = client
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create secret cache")
	}
	return &AWSSecretReader{cache: cache}, nil
}

// GetSecretString retrieves a secret value from AWS Secrets Manager with caching.
func (r *AWSSecretReader) GetSecretString(ctx context.Context, secretID string) (string, error) {
	secret, err := r.cache.GetSecretStringWithContext(ctx, secretID)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get secret %q", secretID)
	}
	return secret, nil
}

// secretFromReader retrieves a secret value, optionally extracting a JSON path.
// If jsonPath is provided, the secret is parsed as JSON and the path is extracted.
// If jsonPath is empty, the raw secret string is returned.
func secretFromReader(ctx context.Context, reader SecretReader, secretID string, jsonPath ...string) (string, error) {
	if len(jsonPath) > 1 {
		return "", errors.New("blwa: Secret accepts at most one jsonPath argument")
	}

	secret, err := reader.GetSecretString(ctx, secretID)
	if err != nil {
		return "", err
	}

	if len(jsonPath) == 0 || jsonPath[0] == "" {
		return secret, nil
	}

	path := jsonPath[0]
	result := gjson.Get(secret, path)
	if !result.Exists() {
		return "", errors.Errorf("secret path %q not found in secret %q", path, secretID)
	}

	return result.String(), nil
}
