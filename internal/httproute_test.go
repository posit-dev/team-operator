package internal

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// noKindMatchClient wraps a client and returns a NoKindMatchError on Delete,
// simulating an environment where the Gateway API CRD is not installed.
type noKindMatchClient struct {
	client.Client
}

func (c *noKindMatchClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return &apimeta.NoKindMatchError{
		GroupKind: schema.GroupKind{Group: "gateway.networking.k8s.io", Kind: "HTTPRoute"},
	}
}

func TestEnsureHTTPRoute(t *testing.T) {
	scheme := runtime.NewScheme()
	err := gatewayv1.Install(scheme)
	require.NoError(t, err)
	err = positcov1beta1.AddToScheme(scheme)
	require.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	require.NoError(t, err)

	t.Run("creates HTTPRoute with correct spec", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		ctx := context.Background()
		logger := logr.Discard()

		cfg := HTTPRouteConfig{
			Name:           "test-route",
			Namespace:      "test-ns",
			GatewayName:    "test-gateway",
			GatewayNS:      "gateway-ns",
			Hostname:       "example.com",
			BackendService: "test-service",
			BackendPort:    8080,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "team-operator",
				"test-label":                   "test-value",
			},
			RequestHeaders: map[string]string{
				"X-Custom-Header": "value1",
				"X-Another":       "value2",
			},
			ResponseHeaders: map[string]string{
				"X-Response": "response-value",
			},
			SessionPersistence: true,
		}

		// Create a fake owner object - use a ConfigMap as a simple client.Object
		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner",
				Namespace: "test-ns",
				UID:       "test-uid",
			},
		}

		err := EnsureHTTPRoute(ctx, fakeClient, scheme, logger, owner, cfg)
		require.NoError(t, err)

		// Verify the HTTPRoute was created
		route := &gatewayv1.HTTPRoute{}
		err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-route", Namespace: "test-ns"}, route)
		require.NoError(t, err)

		// Verify labels
		assert.Equal(t, "team-operator", route.Labels["app.kubernetes.io/managed-by"])
		assert.Equal(t, "test-value", route.Labels["test-label"])

		// Verify spec
		assert.Equal(t, "example.com", string(route.Spec.Hostnames[0]))
		assert.Equal(t, "test-service", string(route.Spec.Rules[0].BackendRefs[0].Name))
		assert.Equal(t, int32(8080), int32(*route.Spec.Rules[0].BackendRefs[0].Port))

		// Verify session persistence
		assert.NotNil(t, route.Spec.Rules[0].SessionPersistence)
		assert.Equal(t, "test-route", *route.Spec.Rules[0].SessionPersistence.SessionName)

		// Verify headers - they should be sorted alphabetically
		require.Len(t, route.Spec.Rules[0].Filters, 2)

		// Request headers should be sorted: X-Another, X-Custom-Header
		reqFilter := route.Spec.Rules[0].Filters[0]
		assert.Equal(t, gatewayv1.HTTPRouteFilterRequestHeaderModifier, reqFilter.Type)
		assert.Len(t, reqFilter.RequestHeaderModifier.Set, 2)
		assert.Equal(t, "X-Another", string(reqFilter.RequestHeaderModifier.Set[0].Name))
		assert.Equal(t, "value2", reqFilter.RequestHeaderModifier.Set[0].Value)
		assert.Equal(t, "X-Custom-Header", string(reqFilter.RequestHeaderModifier.Set[1].Name))
		assert.Equal(t, "value1", reqFilter.RequestHeaderModifier.Set[1].Value)

		// Response headers
		respFilter := route.Spec.Rules[0].Filters[1]
		assert.Equal(t, gatewayv1.HTTPRouteFilterResponseHeaderModifier, respFilter.Type)
		assert.Len(t, respFilter.ResponseHeaderModifier.Set, 1)
		assert.Equal(t, "X-Response", string(respFilter.ResponseHeaderModifier.Set[0].Name))
	})

	t.Run("header ordering is deterministic", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		ctx := context.Background()
		logger := logr.Discard()

		// Create two routes with the same headers in different order
		headers := map[string]string{
			"Z-Header": "z",
			"A-Header": "a",
			"M-Header": "m",
		}

		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner",
				Namespace: "test-ns",
				UID:       "test-uid",
			},
		}

		cfg := HTTPRouteConfig{
			Name:           "test-route-1",
			Namespace:      "test-ns",
			GatewayName:    "test-gateway",
			GatewayNS:      "gateway-ns",
			Hostname:       "example.com",
			BackendService: "test-service",
			BackendPort:    80,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "team-operator",
			},
			RequestHeaders:     headers,
			SessionPersistence: false,
		}

		err := EnsureHTTPRoute(ctx, fakeClient, scheme, logger, owner, cfg)
		require.NoError(t, err)

		route := &gatewayv1.HTTPRoute{}
		err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-route-1", Namespace: "test-ns"}, route)
		require.NoError(t, err)

		// Headers should be sorted alphabetically: A-Header, M-Header, Z-Header
		reqFilter := route.Spec.Rules[0].Filters[0]
		require.Len(t, reqFilter.RequestHeaderModifier.Set, 3)
		assert.Equal(t, "A-Header", string(reqFilter.RequestHeaderModifier.Set[0].Name))
		assert.Equal(t, "M-Header", string(reqFilter.RequestHeaderModifier.Set[1].Name))
		assert.Equal(t, "Z-Header", string(reqFilter.RequestHeaderModifier.Set[2].Name))
	})

	t.Run("empty GatewayName returns error", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		ctx := context.Background()
		logger := logr.Discard()

		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner",
				Namespace: "test-ns",
				UID:       "test-uid",
			},
		}

		cfg := HTTPRouteConfig{
			Name:           "test-route-empty-gw",
			Namespace:      "test-ns",
			GatewayName:    "",
			GatewayNS:      "gateway-ns",
			Hostname:       "example.com",
			BackendService: "test-service",
			BackendPort:    80,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "team-operator",
			},
		}

		err := EnsureHTTPRoute(ctx, fakeClient, scheme, logger, owner, cfg)
		assert.ErrorContains(t, err, "GatewayRef name and namespace must not be empty")
	})

	t.Run("empty GatewayNS returns error", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		ctx := context.Background()
		logger := logr.Discard()

		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner",
				Namespace: "test-ns",
				UID:       "test-uid",
			},
		}

		cfg := HTTPRouteConfig{
			Name:           "test-route-empty-ns",
			Namespace:      "test-ns",
			GatewayName:    "test-gateway",
			GatewayNS:      "",
			Hostname:       "example.com",
			BackendService: "test-service",
			BackendPort:    80,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "team-operator",
			},
		}

		err := EnsureHTTPRoute(ctx, fakeClient, scheme, logger, owner, cfg)
		assert.ErrorContains(t, err, "GatewayRef name and namespace must not be empty")
	})

	t.Run("nil Labels is rejected because managed-by label is required", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		ctx := context.Background()
		logger := logr.Discard()

		owner := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner",
				Namespace: "test-ns",
				UID:       "test-uid",
			},
		}

		cfg := HTTPRouteConfig{
			Name:               "test-route-labels",
			Namespace:          "test-ns",
			GatewayName:        "test-gateway",
			GatewayNS:          "gateway-ns",
			Hostname:           "example.com",
			BackendService:     "test-service",
			BackendPort:        80,
			Labels:             nil, // nil omits managed-by label
			SessionPersistence: false,
		}

		// nil Labels causes an error because CreateOrUpdateResource requires the managed-by label
		err := EnsureHTTPRoute(ctx, fakeClient, scheme, logger, owner, cfg)
		assert.Error(t, err, "nil Labels should be rejected because managed-by label is required")
	})
}

func TestDeleteHTTPRoute(t *testing.T) {
	scheme := runtime.NewScheme()
	err := gatewayv1.Install(scheme)
	require.NoError(t, err)

	t.Run("handles not-found gracefully", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		ctx := context.Background()
		logger := logr.Discard()

		// Try to delete a route that doesn't exist
		err := DeleteHTTPRoute(ctx, fakeClient, logger, "nonexistent-route", "test-ns")
		assert.NoError(t, err, "should not error when route doesn't exist")
	})

	t.Run("tolerates CRD not installed (NoKindMatchError)", func(t *testing.T) {
		fakeClient := &noKindMatchClient{
			Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		}
		ctx := context.Background()
		logger := logr.Discard()

		err := DeleteHTTPRoute(ctx, fakeClient, logger, "test-route", "test-ns")
		assert.NoError(t, err, "should not error when Gateway API CRD is not installed")
	})

	t.Run("deletes existing route", func(t *testing.T) {
		// Create a route first
		route := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-route",
				Namespace: "test-ns",
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(route).Build()
		ctx := context.Background()
		logger := logr.Discard()

		// Verify it exists
		existingRoute := &gatewayv1.HTTPRoute{}
		err := fakeClient.Get(ctx, client.ObjectKey{Name: "test-route", Namespace: "test-ns"}, existingRoute)
		require.NoError(t, err)

		// Delete it
		err = DeleteHTTPRoute(ctx, fakeClient, logger, "test-route", "test-ns")
		require.NoError(t, err)

		// Verify it's gone
		err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-route", Namespace: "test-ns"}, existingRoute)
		assert.Error(t, err, "route should be deleted")
	})
}
