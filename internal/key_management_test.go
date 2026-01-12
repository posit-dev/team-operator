package internal

import (
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureLauncherKeys(t *testing.T) {
	priv, pub, err := GenerateLauncherKeys()

	require.Nil(t, err)

	require.Contains(t, priv, "-----BEGIN RSA PRIVATE KEY-----")
	require.Contains(t, priv, "-----END RSA PRIVATE KEY-----")
	require.Contains(t, pub, "-----BEGIN RSA PUBLIC KEY-----")
	require.Contains(t, pub, "-----END RSA PUBLIC KEY-----")
}

func TestGenerateSecureCookieKey(t *testing.T) {
	t.Run("should generate a 64 character hexadecimal key", func(t *testing.T) {
		key, err := GenerateSecureCookieKey()
		require.NoError(t, err)
		require.Len(t, key, 64)

		// Verify it's all hexadecimal characters
		matched, err := regexp.MatchString("^[a-f0-9]{64}$", key)
		require.NoError(t, err)
		require.True(t, matched, "Key should contain only hexadecimal characters")
	})

	t.Run("should generate unique keys on each call", func(t *testing.T) {
		key1, err1 := GenerateSecureCookieKey()
		require.NoError(t, err1)

		key2, err2 := GenerateSecureCookieKey()
		require.NoError(t, err2)

		require.NotEqual(t, key1, key2, "Keys should be unique")
	})

	t.Run("should not contain any whitespace or newlines", func(t *testing.T) {
		key, err := GenerateSecureCookieKey()
		require.NoError(t, err)

		require.False(t, strings.Contains(key, " "), "Key should not contain spaces")
		require.False(t, strings.Contains(key, "\n"), "Key should not contain newlines")
		require.False(t, strings.Contains(key, "\r"), "Key should not contain carriage returns")
		require.False(t, strings.Contains(key, "\t"), "Key should not contain tabs")
	})
}

func TestValidateSecureCookieKey(t *testing.T) {
	t.Run("should validate a correct 64 character hex key", func(t *testing.T) {
		validKey := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
		valid, errors := ValidateSecureCookieKey(validKey)
		require.True(t, valid)
		require.Empty(t, errors)
	})

	t.Run("should reject a UUID format key", func(t *testing.T) {
		uuidKey := "550e8400-e29b-41d4-a716-446655440000"
		valid, errors := ValidateSecureCookieKey(uuidKey)
		require.False(t, valid)
		require.Contains(t, errors[0], "length")
	})

	t.Run("should reject keys with invalid characters", func(t *testing.T) {
		invalidKey := "g1b2c3d4e5f67890123456789012345678901234567890123456789012345678" // 'g' is not hex
		valid, errors := ValidateSecureCookieKey(invalidKey)
		require.False(t, valid)
		// Should find at least one error about hexadecimal
		found := false
		for _, err := range errors {
			if strings.Contains(err, "hexadecimal") {
				found = true
				break
			}
		}
		require.True(t, found, "Should have error about hexadecimal characters")
	})

	t.Run("should reject keys that are too short", func(t *testing.T) {
		shortKey := "a1b2c3d4"
		valid, errors := ValidateSecureCookieKey(shortKey)
		require.False(t, valid)
		require.Contains(t, errors[0], "length")
	})

	t.Run("should reject keys with trailing newlines", func(t *testing.T) {
		keyWithNewline := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678\n"
		valid, errors := ValidateSecureCookieKey(keyWithNewline)
		require.False(t, valid)
		// Should find at least one error about whitespace
		found := false
		for _, err := range errors {
			if strings.Contains(err, "whitespace") {
				found = true
				break
			}
		}
		require.True(t, found, "Should have error about whitespace")
	})
}

func TestDetectKeyFormat(t *testing.T) {
	t.Run("should detect UUID format", func(t *testing.T) {
		uuidKey := "550e8400-e29b-41d4-a716-446655440000"
		format := DetectKeyFormat(uuidKey)
		require.Equal(t, "uuid", format)
	})

	t.Run("should detect hex256 format", func(t *testing.T) {
		hexKey := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
		format := DetectKeyFormat(hexKey)
		require.Equal(t, "hex256", format)
	})

	t.Run("should detect binary format", func(t *testing.T) {
		binaryKey := string(make([]byte, 32)) // 32-byte binary data
		format := DetectKeyFormat(binaryKey)
		require.Equal(t, "binary", format)
	})

	t.Run("should return unknown for invalid formats", func(t *testing.T) {
		invalidKey := "not-a-valid-key"
		format := DetectKeyFormat(invalidKey)
		require.Equal(t, "unknown", format)
	})
}

func TestMigrateUUIDKey(t *testing.T) {
	t.Run("should convert UUID to hex256 format", func(t *testing.T) {
		uuidKey := "550e8400-e29b-41d4-a716-446655440000"
		needsMigration, newKey, err := MigrateUUIDKey(uuidKey)

		require.NoError(t, err)
		require.True(t, needsMigration)
		require.Len(t, newKey, 64)

		// Verify new key is valid hex
		matched, _ := regexp.MatchString("^[a-f0-9]{64}$", newKey)
		require.True(t, matched)
	})

	t.Run("should convert binary to hex256 format", func(t *testing.T) {
		binaryKey := string(make([]byte, 32))
		needsMigration, newKey, err := MigrateUUIDKey(binaryKey)

		require.NoError(t, err)
		require.True(t, needsMigration)
		require.Len(t, newKey, 64)

		// Verify new key is valid hex
		matched, _ := regexp.MatchString("^[a-f0-9]{64}$", newKey)
		require.True(t, matched)
	})

	t.Run("should not migrate hex256 keys", func(t *testing.T) {
		hexKey := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
		needsMigration, newKey, err := MigrateUUIDKey(hexKey)

		require.NoError(t, err)
		require.False(t, needsMigration)
		require.Empty(t, newKey)
	})

	t.Run("should handle invalid keys gracefully", func(t *testing.T) {
		invalidKey := "not-a-valid-key"
		needsMigration, newKey, err := MigrateUUIDKey(invalidKey)

		require.Error(t, err)
		require.False(t, needsMigration)
		require.Empty(t, newKey)
	})
}

func TestGenerateLauncherKeys(t *testing.T) {
	t.Run("should generate valid PEM-encoded RSA keys", func(t *testing.T) {
		priv, pub, err := GenerateLauncherKeys()

		require.NoError(t, err)
		require.NotEmpty(t, priv)
		require.NotEmpty(t, pub)

		// Check that private key starts with PEM header
		require.Contains(t, priv, "-----BEGIN RSA PRIVATE KEY-----")
		require.Contains(t, priv, "-----END RSA PRIVATE KEY-----")

		// Check that public key starts with PEM header
		require.Contains(t, pub, "-----BEGIN RSA PUBLIC KEY-----")
		require.Contains(t, pub, "-----END RSA PUBLIC KEY-----")
	})

	t.Run("should generate unique keys on each call", func(t *testing.T) {
		priv1, pub1, err1 := GenerateLauncherKeys()
		priv2, pub2, err2 := GenerateLauncherKeys()

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NotEqual(t, priv1, priv2)
		require.NotEqual(t, pub1, pub2)
	})
}

func TestGenerateTestUserPassword(t *testing.T) {
	t.Run("generates valid test user password", func(t *testing.T) {
		password, err := GenerateTestUserPassword()
		require.NoError(t, err)

		// Check length (should be 32 chars as hex encoding of 16 bytes)
		require.Equal(t, 32, len(password))

		// Verify it's valid hex
		_, err = hex.DecodeString(password)
		require.NoError(t, err)

		// Ensure it doesn't contain "rstudio" (case-insensitive)
		require.NotContains(t, strings.ToLower(password), "rstudio")

		// Check minimum length requirement (8+ chars)
		require.GreaterOrEqual(t, len(password), 8)
	})

	t.Run("generates unique passwords", func(t *testing.T) {
		password1, err1 := GenerateTestUserPassword()
		password2, err2 := GenerateTestUserPassword()

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NotEqual(t, password1, password2)
	})
}
