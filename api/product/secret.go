package product

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/pkg/errors"
	v12 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/yaml"
)

// SecretNotFoundError indicates a secret key was not found in the vault
type SecretNotFoundError struct {
	secretType SiteSecretType
	vaultName  string
	key        string
	err        error
}

func newSecretNotFoundError(secretType SiteSecretType, vaultName, key string, err error) *SecretNotFoundError {
	return &SecretNotFoundError{
		secretType: secretType,
		vaultName:  vaultName,
		key:        key,
		err:        err,
	}
}

func (e *SecretNotFoundError) Error() string {
	return fmt.Sprintf("secret key '%s' not found in vault '%s' (type: %s): %v",
		e.key, e.vaultName, e.secretType, e.err)
}

func (e *SecretNotFoundError) Unwrap() error {
	return e.err
}

// SecretAccessError indicates an error accessing the secret store
type SecretAccessError struct {
	secretType SiteSecretType
	vaultName  string
	key        string
	err        error
}

func newSecretAccessError(secretType SiteSecretType, vaultName, key string, err error) *SecretAccessError {
	return &SecretAccessError{
		secretType: secretType,
		vaultName:  vaultName,
		key:        key,
		err:        err,
	}
}

func (e *SecretAccessError) Error() string {
	return fmt.Sprintf("access error fetching secret key '%s' from vault '%s' (type: %s): %v",
		e.key, e.vaultName, e.secretType, e.err)
}

func (e *SecretAccessError) Unwrap() error {
	return e.err
}

type secretObjectJmesPath struct {
	Path        string `json:"path,omitempty"`
	ObjectAlias string `json:"objectAlias,omitempty"`
}
type secretObject struct {
	ObjectName         string                 `json:"objectName,omitempty"`
	ObjectType         string                 `json:"objectType,omitempty"`
	ObjectVersionLabel string                 `json:"objectVersionLabel,omitempty"`
	JmesPath           []secretObjectJmesPath `json:"jmesPath,omitempty"`
}

type TestSecretProvider struct {
	Secrets    map[string]string `json:"secrets,omitempty"`
	StrictMode bool              `json:"strictMode,omitempty"`
}

var GlobalTestSecretProvider = &TestSecretProvider{
	Secrets:    map[string]string{},
	StrictMode: false, // Default to fallback behavior for backward compatibility
}

func (t *TestSecretProvider) SetSecret(key, val string) error {
	t.Secrets[key] = val
	return nil
}

func (t *TestSecretProvider) GetSecret(key string) (string, error) {
	if secret, ok := t.Secrets[key]; !ok {
		return "", errors.New("key not found")
	} else {
		return secret, nil
	}
}

func (t *TestSecretProvider) GetSecretWithFallback(key string) string {
	if secret, ok := t.Secrets[key]; !ok {
		return key
	} else {
		return secret
	}
}

func (t *TestSecretProvider) SetStrictMode(strict bool) {
	t.StrictMode = strict
}

func (t *TestSecretProvider) Reset() {
	t.Secrets = map[string]string{}
	t.StrictMode = false
}

func mapToJmesPath(input map[string]string) (jmes []secretObjectJmesPath) {
	for k, v := range input {
		jmes = append(jmes, secretObjectJmesPath{
			// ensure this path is quoted appropriately...
			Path:        fmt.Sprintf("\"%s\"", v),
			ObjectAlias: k,
		})
	}
	return jmes
}

func generateSecretObjectYaml(name string, keys map[string]string) (string, error) {
	tmp := secretObject{
		ObjectName:         name,
		ObjectType:         "secretsmanager",
		ObjectVersionLabel: "AWSCURRENT",
		JmesPath:           mapToJmesPath(keys),
	}
	if y, err := yaml.Marshal([]secretObject{tmp}); err != nil {
		return "", err
	} else {
		return string(y), nil
	}
}

func generateSecretObjects(p KubernetesOwnerProvider, secrets map[string]map[string]string) (output []*v1.SecretObject) {
	for k, v := range secrets {
		var objData []*v1.SecretObjectData
		for dk, dv := range v {
			objData = append(objData, &v1.SecretObjectData{
				ObjectName: dv,
				Key:        dk,
			})
		}
		output = append(output,
			&v1.SecretObject{
				SecretName: k,
				Type:       "Opaque",
				Labels:     p.KubernetesLabels(),
				Data:       objData,
			},
		)
	}
	return output
}

func GetSecretProviderClassForAllSecrets(p KubernetesOwnerProvider, name, namespace, vaultName string, secretRefs map[string]string, kubernetesSecrets map[string]map[string]string) (*v1.SecretProviderClass, error) {
	if secretObjectYaml, err := generateSecretObjectYaml(vaultName, secretRefs); err != nil {
		return nil, err
	} else {
		return &v1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				Labels:          p.KubernetesLabels(),
				OwnerReferences: p.OwnerReferencesForChildren(),
			},
			Spec: v1.SecretProviderClassSpec{
				Provider:      "aws",
				SecretObjects: generateSecretObjects(p, kubernetesSecrets),
				Parameters: map[string]string{
					"objects": secretObjectYaml,
					"region":  getAWSRegion(),
				},
			},
		}, nil
	}
}

type SiteSecretType string

const (
	SiteSecretKubernetes SiteSecretType = "kubernetes"
	SiteSecretAws        SiteSecretType = "aws"
	SiteSecretTest       SiteSecretType = "test"
	SiteSecretNone       SiteSecretType = ""
)

// getAWSRegion returns the AWS region to use for secret operations.
// It checks the AWS_REGION environment variable first, then falls back to AWS_DEFAULT_REGION,
// and finally defaults to us-east-2 for backwards compatibility.
func getAWSRegion() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}
	// Fallback to the original hardcoded region for backwards compatibility
	return endpoints.UsEast2RegionID
}

func FetchSecret(ctx context.Context, r SomeReconciler, req ctrl.Request, secretType SiteSecretType, vaultName, key string) (string, error) {
	l := r.GetLogger(ctx)
	switch secretType {
	case SiteSecretAws:
		return fetchAWSSecret(secretType, vaultName, key)

	case SiteSecretKubernetes:
		return fetchKubernetesSecret(ctx, r, secretType, vaultName, req.Namespace, key)

	case SiteSecretTest:
		// Use the global test secret provider
		if GlobalTestSecretProvider.StrictMode {
			// In strict mode, return typed errors for missing secrets
			secret, err := GlobalTestSecretProvider.GetSecret(key)
			if err != nil {
				return "", newSecretNotFoundError(secretType, vaultName, key, err)
			}
			return secret, nil

		}
		// In non-strict mode, use fallback behavior (returns key if not found)
		return GlobalTestSecretProvider.GetSecretWithFallback(key), nil

	default:
		err := errors.New("unknown secret type")
		l.Error(err, "Unknown secret type", "type", secretType)
		return "", err
	}
}

func fetchAWSSecret(secretType SiteSecretType, vaultName, key string) (string, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(getAWSRegion()),
	})
	if err != nil {
		return "", newSecretAccessError(secretType, vaultName, key, err)
	}
	sm := secretsmanager.New(sess)
	query := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(vaultName),
		VersionId:    nil,
		VersionStage: aws.String("AWSCURRENT"),
	}
	valueOutput, err := sm.GetSecretValue(query)
	if err != nil {
		var awsErr awserr.Error
		if errors.As(err, &awsErr) && awsErr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
			return "", newSecretNotFoundError(secretType, vaultName, key, err)
		}
		return "", newSecretAccessError(secretType, vaultName, key, err)
	}

	secretValue := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(*valueOutput.SecretString), &secretValue); err != nil {
		// Malformed secret - access error
		return "", newSecretAccessError(secretType, vaultName, key, err)
	}

	rawSecretEntry, ok := secretValue[key]
	if !ok {
		// Vault exists but key doesn't - this is "not found"
		return "", newSecretNotFoundError(secretType, vaultName, key,
			fmt.Errorf("key %q not present in secret", key))
	}

	var secretEntry string
	if err := json.Unmarshal(rawSecretEntry, &secretEntry); err != nil {
		// Key exists but can't unmarshal - access error
		return "", newSecretAccessError(secretType, vaultName, key, err)
	}

	return secretEntry, nil
}

func fetchKubernetesSecret(ctx context.Context, r SomeReconciler, secretType SiteSecretType,
	vaultName, namespace, key string,
) (string, error) {
	kubernetesSecretName := client.ObjectKey{Name: vaultName, Namespace: namespace}

	existingSecret := &v12.Secret{}
	if err := r.Get(ctx, kubernetesSecretName, existingSecret); err != nil {
		if kerrors.IsNotFound(err) {
			// Secret doesn't exist
			return "", newSecretNotFoundError(secretType, vaultName, key, err)
		}
		// Other error (permissions, network, etc.)
		return "", newSecretAccessError(secretType, vaultName, key, err)
	}

	secretEntry, exists := existingSecret.Data[key]
	if !exists {
		// Secret exists but key doesn't
		return "", newSecretNotFoundError(secretType, vaultName, key,
			fmt.Errorf("key %q not found in kubernetes secret", key))
	}

	return string(secretEntry), nil
}
