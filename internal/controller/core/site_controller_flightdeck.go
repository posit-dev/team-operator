package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	// DefaultFlightdeckRegistry is the default container registry for Flightdeck images
	DefaultFlightdeckRegistry = "docker.io/posit"
	// DefaultFlightdeckImageName is the default image name for Flightdeck
	DefaultFlightdeckImageName = "ptd-flightdeck"
	// DefaultFlightdeckTag is the default tag for Flightdeck images
	DefaultFlightdeckTag = "latest"
)

// ResolveFlightdeckImage resolves the Flightdeck container image from the provided configuration.
// If image is empty, returns the default image (docker.io/posit/ptd-flightdeck:latest).
// If image contains a slash, it's treated as a full image path and returned as-is.
// Otherwise, it's treated as a tag and combined with the default registry and image name.
func ResolveFlightdeckImage(image string) string {
	if image == "" {
		return fmt.Sprintf("%s/%s:%s", DefaultFlightdeckRegistry, DefaultFlightdeckImageName, DefaultFlightdeckTag)
	}
	// If it contains a slash, assume it's a full image reference
	if strings.Contains(image, "/") {
		return image
	}
	// Otherwise treat as a tag
	return fmt.Sprintf("%s/%s:%s", DefaultFlightdeckRegistry, DefaultFlightdeckImageName, image)
}

func (r *SiteReconciler) reconcileFlightdeck(
	ctx context.Context,
	req controllerruntime.Request,
	site *v1beta1.Site,
) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-flightdeck",
	)

	// Skip Flightdeck reconciliation if explicitly disabled
	if site.Spec.Flightdeck.Enabled != nil && !*site.Spec.Flightdeck.Enabled {
		l.V(1).Info("skipping Flightdeck reconciliation: explicitly disabled via Site.Spec.Flightdeck.Enabled=false")
		return nil
	}

	// Resolve the Flightdeck image (defaults to docker.io/posit/ptd-flightdeck:latest)
	flightdeckImage := ResolveFlightdeckImage(site.Spec.Flightdeck.Image)

	// Set default replicas if not provided
	replicas := site.Spec.Flightdeck.Replicas
	if replicas == 0 {
		replicas = 1
	}

	// Set default image pull policy if not provided.
	// Default to Always to ensure the latest image is pulled on each deployment.
	// This increases container startup time and network usage but ensures
	// consistency, especially important for mutable tags like 'latest'.
	// Users can override this in the Site spec if needed.
	imagePullPolicy := site.Spec.Flightdeck.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = "Always"
	}

	// Set default log settings if not provided
	logLevel := site.Spec.Flightdeck.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := site.Spec.Flightdeck.LogFormat
	if logFormat == "" {
		logFormat = "text"
	}

	flightdeck := &v1beta1.Flightdeck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	l.V(1).Info("creating or updating Flightdeck CRD")

	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, flightdeck, site, func() error {
		flightdeck.Labels = map[string]string{
			v1beta1.ManagedByLabelKey: LabelManagedByValue,
		}
		flightdeck.Spec = v1beta1.FlightdeckSpec{
			SiteName:             site.Name,
			Image:                flightdeckImage,
			ImagePullPolicy:      imagePullPolicy,
			Port:                 8080,
			Replicas:             replicas,
			FeatureEnabler:       site.Spec.Flightdeck.FeatureEnabler,
			Domain:               site.Spec.Domain,
			IngressClass:         site.Spec.IngressClass,
			IngressAnnotations:   site.Spec.IngressAnnotations,
			ImagePullSecrets:     site.Spec.ImagePullSecrets,
			AwsAccountId:         site.Spec.AwsAccountId,
			ClusterDate:          site.Spec.ClusterDate,
			WorkloadCompoundName: site.Spec.WorkloadCompoundName,
			LogLevel:             logLevel,
			LogFormat:            logFormat,
		}
		return nil
	}); err != nil {
		l.Error(err, "failed to create or update Flightdeck")
		return err
	}

	l.V(1).Info("successfully created or updated Flightdeck CRD")

	return nil
}
