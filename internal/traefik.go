package internal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TraefikMiddlewaresKey = "traefik.ingress.kubernetes.io/router.middlewares"

// TraefikOwner combines KubernetesOwnerProvider (for labels) with client.Object (for owner references).
// This allows passing a single object that satisfies both requirements.
type TraefikOwner interface {
	product.KubernetesOwnerProvider
	client.Object
}

func BuildTraefikMiddlewareAnnotation(ns string, middlewareNames ...string) string {
	output := ""
	for i, middlewareName := range middlewareNames {
		if i > 0 {
			output += ","
		}
		output += fmt.Sprintf("%s-%s@kubernetescrd", ns, middlewareName)
	}
	return output
}

func DeployTraefikForwardMiddleware(ctx context.Context, req ctrl.Request, c client.Client, scheme *runtime.Scheme, l logr.Logger, name string, owner TraefikOwner) error {
	l = l.WithValues(
		"function", "DeployTraefikForwardMiddleware",
	)

	middleware := &v1alpha1.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
		},
	}
	l.Info("CREATING Forward traefik middleware...")
	_, err := CreateOrUpdateResource(ctx, c, scheme, l, middleware, owner, func() error {
		middleware.Labels = owner.KubernetesLabels()
		middleware.Spec = v1alpha1.MiddlewareSpec{
			Headers: &dynamic.Headers{
				CustomRequestHeaders: map[string]string{
					"X-Forwarded-Port":  "443",
					"X-Forwarded-Proto": "https",
				},
			},
		}
		return nil
	})
	if err != nil {
		l.Error(err, "Error creating or updating Forward Middleware")
		return err
	}
	l.Info("DONE creating Forward traefik middleware...?")

	return nil
}

func DeployTraefikForwardMiddlewareWithHost(ctx context.Context, req ctrl.Request, c client.Client, scheme *runtime.Scheme, l logr.Logger, name string, owner TraefikOwner, host string) error {
	l = l.WithValues(
		"function", "DeployTraefikForwardMiddlewareWithHost",
	)

	middleware := &v1alpha1.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
		},
	}
	l.Info("CREATING Forward traefik middleware...")
	_, err := CreateOrUpdateResource(ctx, c, scheme, l, middleware, owner, func() error {
		middleware.Labels = owner.KubernetesLabels()
		middleware.Spec = v1alpha1.MiddlewareSpec{
			Headers: &dynamic.Headers{
				CustomRequestHeaders: map[string]string{
					"X-Forwarded-Port":  "443",
					"X-Forwarded-Proto": "https",
					"X-Forwarded-Host":  host,
					"X-Forwarded-For":   fmt.Sprintf("https://%s", host),
				},
			},
		}
		return nil
	})
	if err != nil {
		l.Error(err, "Error creating or updating Forward Middleware")
		return err
	}
	l.Info("DONE creating Forward traefik middleware...?")

	return nil
}

func TraefikStickyServiceAnnotations(p product.Product) map[string]string {
	return map[string]string{
		"traefik.ingress.kubernetes.io/service.sticky.cookie":          "true",
		"traefik.ingress.kubernetes.io/service.sticky.cookie.httponly": "true",
		"traefik.ingress.kubernetes.io/service.sticky.cookie.name":     p.ComponentName(),
		"traefik.ingress.kubernetes.io/service.sticky.cookie.samesite": "none",
		"traefik.ingress.kubernetes.io/service.sticky.cookie.secure":   "true",
	}
}
