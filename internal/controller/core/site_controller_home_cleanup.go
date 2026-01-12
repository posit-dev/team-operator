package core

import (
	"context"

	"github.com/posit-dev/team-operator/internal"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// cleanupLegacyHomeApp removes legacy home app resources that were deployed prior to the
// flightdeck migration. This function is called during normal Site reconciliation to ensure
// that old home app resources are cleaned up automatically across all sites.
func (r *SiteReconciler) cleanupLegacyHomeApp(
	ctx context.Context,
	req controllerruntime.Request,
) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-legacy-home",
	)

	componentName := req.Name + "-home"
	key := client.ObjectKey{
		Name:      componentName,
		Namespace: req.Namespace,
	}

	// Delete Ingress
	existingIngress := &networkingv1.Ingress{}
	if err := internal.BasicDelete(ctx, r, l, key, existingIngress); err != nil {
		l.Error(err, "error deleting legacy home ingress")
		return err
	}

	// Delete Service
	existingService := &corev1.Service{}
	if err := internal.BasicDelete(ctx, r, l, key, existingService); err != nil {
		l.Error(err, "error deleting legacy home service")
		return err
	}

	// Delete Deployment
	existingDeployment := &v1.Deployment{}
	if err := internal.BasicDelete(ctx, r, l, key, existingDeployment); err != nil {
		l.Error(err, "error deleting legacy home deployment")
		return err
	}

	// Delete SecretProviderClass (if the CRD exists)
	// The SecretProviderClass CRD may not be installed in all environments (e.g., test environments),
	// so we gracefully handle the NoKindMatchError
	secretProviderClassName := componentName + "-secrets"
	spcKey := client.ObjectKey{
		Name:      secretProviderClassName,
		Namespace: req.Namespace,
	}
	existingSpc := &secretsstorev1.SecretProviderClass{}
	if err := internal.BasicDelete(ctx, r, l, spcKey, existingSpc); err != nil {
		// Check if the error is because the CRD doesn't exist
		if _, isNoKindMatch := err.(*meta.NoKindMatchError); isNoKindMatch {
			l.V(1).Info("SecretProviderClass CRD not available, skipping cleanup")
		} else {
			l.Error(err, "error deleting legacy home SecretProviderClass")
			return err
		}
	}

	l.V(1).Info("successfully cleaned up legacy home app resources")

	return nil
}
