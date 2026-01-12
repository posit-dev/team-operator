package internal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/rstudio/rskey/crypt"
	"github.com/rstudio/rskey/workbench"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The KeyProvisioner interface is used by both ImplConnect and Workbench to provision their "management keys" and store
// them in a secret.
type KeyProvisioner interface {
	KeySecretName() string
}

func CleanupProvisioningKey(ctx context.Context, k KeyProvisioner, r product.SomeReconciler, req ctrl.Request) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-provisioning-key",
		"secret", k.KeySecretName(),
	)

	s := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: k.KeySecretName(), Namespace: req.Namespace}, s); err != nil && errors.IsNotFound(err) {
		// secret is missing, good to go
	} else if err != nil {
		// unknown error
		l.Error(err, "error trying to retrieve kubernetes secret")
		return err
	} else {
		// secret found. Let's get rid of it
		l.Info("deleting secret")
		if err := r.Delete(ctx, s); err != nil && errors.IsNotFound(err) {
			// success!
		} else if err != nil {
			l.Error(err, "error trying to delete kubernetes secret")
			return err
		}
	}

	return nil
}

func GenerateLauncherKeys() (string, string, error) {
	if rsaKey, err := rsa.GenerateKey(rand.Reader, 2056); err != nil {
		return "", "", err
	} else {
		if err := rsaKey.Validate(); err != nil {
			return "", "", err
		}
		rsaKeyBytes := x509.MarshalPKCS1PrivateKey(rsaKey)
		pemKey := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: rsaKeyBytes,
		})

		rsaPubKey := rsaKey.PublicKey
		pubKeyBytes := x509.MarshalPKCS1PublicKey(&rsaPubKey)
		pubKey := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pubKeyBytes,
		})

		return string(pemKey), string(pubKey), nil
	}
	// TODO: generate PEM and PUB keys for launcher...
	// Workbench really needs this...
}

func EnsureWorkbenchSecretKey(ctx context.Context, k KeyProvisioner, r product.SomeReconciler, req ctrl.Request, owner product.OwnerProvider) (*workbench.Key, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "ensure-secret-key",
	)

	// initialize or fetch the key...
	if secretData, err := EnsureSecretKubernetes(
		ctx, r, req, owner, k.KeySecretName(),
		func() map[string]string {
			// Generate proper hex key instead of UUID
			newKey, err := GenerateSecureCookieKey()
			if err != nil {
				l.Error(err, "error generating secure cookie key")
				// Fallback to UUID if generation fails
				newKey = string(uuid.NewUUID())
			}

			// Log key format for debugging
			format := DetectKeyFormat(newKey)
			l.Info("generated secure-cookie-key", "format", format, "length", len(newKey))

			// Generate test user password
			testUserPassword, err := GenerateTestUserPassword()
			if err != nil {
				l.Error(err, "error generating test user password")
				// Fallback to a simple hex string if generation fails
				testUserPassword = hex.EncodeToString([]byte("DefaultTestPassword123!"))
			}

			priv, pub, _ := GenerateLauncherKeys()
			return map[string]string{
				"key":               newKey,
				"launcher.pem":      priv,
				"launcher.pub":      pub,
				"testuser-password": testUserPassword,
			}
		}); err != nil {
		l.Error(err, "error defining key")
		return nil, err
	} else {
		keyStr := secretData["key"]

		// Check if existing key needs migration
		format := DetectKeyFormat(keyStr)
		if format == "uuid" || format == "binary" {
			l.Info("detected legacy format key, migrating to hex256", "current_format", format, "length", len(keyStr))

			// Generate a new hex256 key
			needsMigration, newKey, err := MigrateUUIDKey(keyStr)
			if err != nil {
				l.Error(err, "error migrating key")
				return nil, err
			}

			if needsMigration {
				// Update the secret with the new key, preserving launcher keys
				secretData["key"] = newKey

				// Ensure launcher keys exist (generate if missing)
				if _, hasPriv := secretData["launcher.pem"]; !hasPriv {
					priv, pub, err := GenerateLauncherKeys()
					if err != nil {
						l.Error(err, "error generating launcher keys during migration")
					} else {
						secretData["launcher.pem"] = priv
						secretData["launcher.pub"] = pub
						l.Info("generated missing launcher keys during migration")
					}
				}

				if err := UpdateSecretKubernetes(ctx, r, req, owner, k.KeySecretName(), secretData); err != nil {
					l.Error(err, "error updating secret with migrated key")
					return nil, err
				}
				keyStr = newKey
				l.Info("successfully migrated key to hex256 format", "new_length", len(keyStr))
			}
		} else if format == "hex256" {
			l.V(1).Info("secure-cookie-key format verified", "format", format)
		} else {
			l.Error(nil, "unknown key format detected", "format", format, "length", len(keyStr))
			// Generate a new key if format is unknown
			l.Info("generating new hex256 key due to unknown format")
			newKey, err := GenerateSecureCookieKey()
			if err != nil {
				l.Error(err, "error generating new key")
				return nil, err
			}
			secretData["key"] = newKey

			// Ensure launcher keys exist (generate if missing)
			if _, hasPriv := secretData["launcher.pem"]; !hasPriv {
				priv, pub, err := GenerateLauncherKeys()
				if err != nil {
					l.Error(err, "error generating launcher keys for unknown format")
				} else {
					secretData["launcher.pem"] = priv
					secretData["launcher.pub"] = pub
					l.Info("generated missing launcher keys for unknown format")
				}
			}

			if err := UpdateSecretKubernetes(ctx, r, req, owner, k.KeySecretName(), secretData); err != nil {
				l.Error(err, "error updating secret with new key")
				return nil, err
			}
			keyStr = newKey
		}

		// Ensure testuser-password exists (generate if missing)
		if _, hasTestUserPassword := secretData["testuser-password"]; !hasTestUserPassword {
			testUserPassword, err := GenerateTestUserPassword()
			if err != nil {
				l.Error(err, "error generating test user password")
				testUserPassword = hex.EncodeToString([]byte("DefaultTestPassword123!"))
			}
			secretData["testuser-password"] = testUserPassword

			if err := UpdateSecretKubernetes(ctx, r, req, owner, k.KeySecretName(), secretData); err != nil {
				l.Error(err, "error updating secret with test user password")
				// Continue even if update fails - the key itself is still valid
			} else {
				l.Info("added missing test user password to secret")
			}
		}

		// Validate the key
		if valid, errors := ValidateSecureCookieKey(keyStr); !valid && format == "hex256" {
			l.Error(nil, "secure-cookie-key validation failed", "errors", errors)
		}

		key, err := workbench.NewKeyFromReader(strings.NewReader(keyStr))
		// TODO: should probably return launcher.pem and launcher.pub too...?
		// - they are not needed... yet...
		if err != nil {
			l.Error(err, "error parsing key from secret data")
			return nil, err
		}
		return key, nil
	}
}

func EnsureProvisioningKey(ctx context.Context, k KeyProvisioner, r product.SomeReconciler, req ctrl.Request, owner product.OwnerProvider) (*crypt.Key, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "ensure-provisioning-key",
	)

	// initialize or fetch key...
	if secretData, err := EnsureSecretKubernetes(
		ctx, r, req, owner, k.KeySecretName(),
		func() map[string]string {
			// TODO: at some point we should handle errors...
			newKey, err := crypt.NewKey()
			if err != nil {
				// TODO: is it ok to use `l` here...? from another scope?
				l.Error(err, "error generating new key")
			}

			return map[string]string{
				"key": newKey.HexString(),
			}
		},
	); err != nil {
		l.Error(err, "error defining key")
		return nil, err
	} else {
		keyStr := secretData["key"]
		key, err := crypt.NewKeyFromReader(strings.NewReader(keyStr))
		if err != nil {
			l.Error(err, "error parsing key from secret data")
			return nil, err
		}
		return key, nil
	}
}

// GenerateSecureCookieKey generates a 256-bit (64 hex character) secure cookie key
// suitable for Workbench load balancing configurations
func GenerateSecureCookieKey() (string, error) {
	// Generate 32 random bytes (256 bits)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hexadecimal string (64 characters)
	return hex.EncodeToString(randomBytes), nil
}

// ValidateSecureCookieKey validates that a key meets the format requirements
// for Workbench secure cookie keys (64 hex characters, no whitespace)
func ValidateSecureCookieKey(key string) (bool, []string) {
	var errors []string

	// Check for whitespace
	if strings.TrimSpace(key) != key {
		errors = append(errors, "key contains whitespace or newlines")
	}

	// Check length
	if len(key) != 64 {
		errors = append(errors, fmt.Sprintf("key length must be 64 characters, got %d", len(key)))
	}

	// Check if all characters are hexadecimal
	matched, _ := regexp.MatchString("^[a-f0-9]{64}$", key)
	if !matched && len(key) == 64 {
		errors = append(errors, "key must contain only hexadecimal characters (0-9, a-f)")
	}

	return len(errors) == 0, errors
}

// DetectKeyFormat determines if a key is in UUID, hex256, binary, or unknown format
func DetectKeyFormat(key string) string {
	// Check for UUID format (36 chars with dashes)
	if matched, _ := regexp.MatchString("^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$", strings.ToLower(key)); matched {
		return "uuid"
	}

	// Check for hex256 format (64 hex chars)
	if matched, _ := regexp.MatchString("^[a-f0-9]{64}$", key); matched {
		return "hex256"
	}

	// Check if it might be raw binary (32 bytes)
	if len(key) == 32 {
		return "binary"
	}

	return "unknown"
}

// GenerateTestUserPassword generates a secure password for the RSW test user
// Uses a hex-encoded random string similar to secure-cookie-key
func GenerateTestUserPassword() (string, error) {
	// Generate 16 random bytes (will produce 32 character hex string)
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string - this ensures no "rstudio" substring and meets complexity requirements
	return hex.EncodeToString(bytes), nil
}

// MigrateUUIDKey converts a UUID or binary format key to hex256 format if needed
func MigrateUUIDKey(currentKey string) (bool, string, error) {
	format := DetectKeyFormat(currentKey)

	switch format {
	case "uuid", "binary":
		// Need to generate a new hex256 key
		newKey, err := GenerateSecureCookieKey()
		if err != nil {
			return false, "", err
		}
		return true, newKey, nil
	case "hex256":
		// Already in correct format
		return false, "", nil
	default:
		return false, "", fmt.Errorf("invalid key format: %s", format)
	}
}
