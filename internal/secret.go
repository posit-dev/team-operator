package internal

import (
	"context"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureSecretKubernetes allows you to have a "bootstrapping" function initialState(), which is only used to generate values
// on the creation of the secret. Updates are not handled presently.
// TODO: maybe we should have a struct that we pass in of the "expected shape of output secrets"...?
func EnsureSecretKubernetes(ctx context.Context, r product.SomeReconciler, req ctrl.Request, owner product.OwnerProvider, name string, initialState func() map[string]string) (map[string]string, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "store-secret-kubernetes",
		"database", name,
	)

	s := &v1.Secret{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: req.Namespace,
	}

	if err := r.Get(ctx, key, s); err != nil && errors.IsNotFound(err) {
		// does not exist, create
		defaultSecretData := initialState()

		secretValue := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: req.Namespace,
				Labels: map[string]string{
					v1beta1.ManagedByLabelKey: v1beta1.ManagedByLabelValue,
				},
				OwnerReferences: owner.OwnerReferencesForChildren(),
			},
			Immutable: nil,
			// not base64 encoded
			StringData: defaultSecretData,
		}

		if err := r.Create(ctx, secretValue); err != nil {
			return nil, err
		}
		return defaultSecretData, nil
	} else if err != nil {
		// unknown error
		return nil, err
	}

	// TODO: handle updates at some point?
	l.Info("the secret already exists; we do not currently support updating values")

	// NOTE: this is very interesting that the client automatically base64 decodes this data for us when calling string()
	outputStringData := make(map[string]string)
	for k, v := range s.Data {
		outputStringData[k] = string(v)
	}

	return outputStringData, nil
}

// UpdateSecretKubernetes updates an existing secret with new data
func UpdateSecretKubernetes(ctx context.Context, r product.SomeReconciler, req ctrl.Request, owner product.OwnerProvider, name string, data map[string]string) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "update-secret-kubernetes",
		"secret", name,
	)

	s := &v1.Secret{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: req.Namespace,
	}

	if err := r.Get(ctx, key, s); err != nil {
		if errors.IsNotFound(err) {
			l.Error(err, "secret not found for update")
		}
		return err
	}

	// Update the secret data
	s.StringData = data

	if err := r.Update(ctx, s); err != nil {
		l.Error(err, "error updating secret")
		return err
	}

	l.Info("successfully updated secret")
	return nil
}
