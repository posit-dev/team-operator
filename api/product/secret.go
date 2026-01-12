package product

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/pkg/errors"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/yaml"
)

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
	Secrets map[string]string `json:"secrets,omitempty"`
}

var GlobalTestSecretProvider = &TestSecretProvider{
	Secrets: map[string]string{},
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
		if sess, err := session.NewSession(&aws.Config{
			Region: aws.String(getAWSRegion()),
		}); err != nil {
			return "", err
		} else {
			sm := secretsmanager.New(sess)
			query := &secretsmanager.GetSecretValueInput{
				SecretId:     aws.String(vaultName),
				VersionId:    nil,
				VersionStage: aws.String("AWSCURRENT"),
			}
			if valueOutput, err := sm.GetSecretValue(query); err != nil {
				return "", err
			} else {
				secretValue := map[string]json.RawMessage{}
				if err := json.Unmarshal([]byte(*valueOutput.SecretString), &secretValue); err != nil {
					return "", err
				}

				if rawSecretEntry, ok := secretValue[key]; !ok {
					// failed to find the configured key
					return "", errors.New(fmt.Sprintf("could not find the configured key '%s' in secret '%s' with type '%s'", key, vaultName, secretType))
				} else {
					var secretEntry string
					if err := json.Unmarshal(rawSecretEntry, &secretEntry); err != nil {
						// error unmarshalling secret
						return "", err
					} else {
						// SUCCESS!! we got the secret!
						return secretEntry, nil
					}
				}
			}
		}
	case SiteSecretKubernetes:
		kubernetesSecretName := client.ObjectKey{Name: vaultName, Namespace: req.Namespace}

		existingSecret := &v12.Secret{}
		if err := r.Get(ctx, kubernetesSecretName, existingSecret); err != nil {
			l.Error(err, "Error retrieving kubernetes secret", "secret", kubernetesSecretName)
			return "", err
		} else {
			secretEntry := existingSecret.Data[key]
			return string(secretEntry), nil
		}
	case SiteSecretTest:
		// try using the global test secret provider (or fallback to the key)
		return GlobalTestSecretProvider.GetSecretWithFallback(key), nil
	default:
		err := errors.New("unknown secret type")
		l.Error(err, "Unknown secret type", "type", secretType)
		return "", err
	}
}
