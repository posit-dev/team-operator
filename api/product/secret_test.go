package product

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecretObjectYaml(t *testing.T) {
	res, err := generateSecretObjectYaml("name", map[string]string{
		"client-secret":   "pub-client-secret",
		"secret.key":      "pub-secret-key",
		"pub-db-password": "pub-db-password",
		"pub.lic":         "pub-license",
		"dev.lic":         "dev-license",
		"pkg.lic":         "pkg-license",
	})

	require.Nil(t, err)
	require.Contains(t, res, "path: '\"pub-client-secret\"'")
	require.Contains(t, res, "path: '\"pub-license\"'")
}

func TestGlobalSecretProvider(t *testing.T) {
	secretKey := "secret"
	secretValue := "value"
	err := GlobalTestSecretProvider.SetSecret(secretKey, secretValue)
	assert.Nil(t, err)

	val, err := GlobalTestSecretProvider.GetSecret(secretKey)
	assert.Nil(t, err)
	assert.Equal(t, secretValue, val)

	// a secret not present
	val, err = GlobalTestSecretProvider.GetSecret("other")
	assert.NotNil(t, err)
	assert.Equal(t, "", val)

	// a secret not present with fallback
	val = GlobalTestSecretProvider.GetSecretWithFallback("other")
	assert.Equal(t, "other", val)
}
