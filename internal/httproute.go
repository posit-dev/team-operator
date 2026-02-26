package internal

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/rstudio/goex/ptr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HTTPRouteConfig contains all parameters for creating/updating an HTTPRoute
type HTTPRouteConfig struct {
	Name, Namespace        string
	GatewayName, GatewayNS string
	Hostname               string
	BackendService         string
	BackendPort            int32
	Labels                 map[string]string
	RequestHeaders         map[string]string
	ResponseHeaders        map[string]string
	SessionPersistence     bool
}

// EnsureHTTPRoute creates or updates an HTTPRoute for a product
func EnsureHTTPRoute(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	l logr.Logger,
	owner client.Object,
	cfg HTTPRouteConfig,
) error {
	if cfg.GatewayName == "" || cfg.GatewayNS == "" {
		return fmt.Errorf("GatewayRef name and namespace must not be empty")
	}

	l = l.WithValues(
		"function", "EnsureHTTPRoute",
		"name", cfg.Name,
		"namespace", cfg.Namespace,
	)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
	}

	l.Info("Creating or updating HTTPRoute")
	_, err := CreateOrUpdateResource(ctx, c, scheme, l, route, owner, func() error {
		// Set labels (required by CreateOrUpdateResource)
		route.Labels = cfg.Labels

		// Build filters for header manipulation
		filters := []gatewayv1.HTTPRouteFilter{}

		if len(cfg.RequestHeaders) > 0 {
			headerFilter := gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{},
				},
			}
			// Sort keys for deterministic ordering
			keys := make([]string, 0, len(cfg.RequestHeaders))
			for k := range cfg.RequestHeaders {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				headerFilter.RequestHeaderModifier.Set = append(
					headerFilter.RequestHeaderModifier.Set,
					gatewayv1.HTTPHeader{
						Name:  gatewayv1.HTTPHeaderName(key),
						Value: cfg.RequestHeaders[key],
					},
				)
			}
			filters = append(filters, headerFilter)
		}

		if len(cfg.ResponseHeaders) > 0 {
			headerFilter := gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{},
				},
			}
			// Sort keys for deterministic ordering
			keys := make([]string, 0, len(cfg.ResponseHeaders))
			for k := range cfg.ResponseHeaders {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				headerFilter.ResponseHeaderModifier.Set = append(
					headerFilter.ResponseHeaderModifier.Set,
					gatewayv1.HTTPHeader{
						Name:  gatewayv1.HTTPHeaderName(key),
						Value: cfg.ResponseHeaders[key],
					},
				)
			}
			filters = append(filters, headerFilter)
		}

		// Build the HTTPRoute spec
		rule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  ptr.To(gatewayv1.PathMatchPathPrefix),
					Value: ptr.To("/"),
				},
			}},
			BackendRefs: []gatewayv1.HTTPBackendRef{{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: gatewayv1.ObjectName(cfg.BackendService),
						Port: (*gatewayv1.PortNumber)(&cfg.BackendPort),
					},
				},
			}},
			Filters: filters,
		}

		// Add session persistence if needed
		if cfg.SessionPersistence {
			// Note: Secure/HttpOnly/SameSite cookie attributes are not set here because
			// gatewayv1.SessionPersistence does not expose those fields — they are delegated
			// to the gateway implementation. Behavior may vary across gateway providers.
			rule.SessionPersistence = &gatewayv1.SessionPersistence{
				SessionName: ptr.To(cfg.Name),
				Type:        ptr.To(gatewayv1.CookieBasedSessionPersistence),
			}
		}

		route.Spec = gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name:      gatewayv1.ObjectName(cfg.GatewayName),
					Namespace: (*gatewayv1.Namespace)(&cfg.GatewayNS),
				}},
			},
			Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(cfg.Hostname)},
			Rules:     []gatewayv1.HTTPRouteRule{rule},
		}

		return nil
	})
	if err != nil {
		l.Error(err, "Error creating or updating HTTPRoute")
		return err
	}

	l.Info("Successfully created or updated HTTPRoute")
	return nil
}

// DeleteHTTPRoute removes an HTTPRoute
func DeleteHTTPRoute(ctx context.Context, c client.Client, l logr.Logger, name, namespace string) error {
	l = l.WithValues(
		"function", "DeleteHTTPRoute",
		"name", name,
		"namespace", namespace,
	)

	route := &gatewayv1.HTTPRoute{}
	route.Name = name
	route.Namespace = namespace

	if err := c.Delete(ctx, route); err != nil {
		if client.IgnoreNotFound(err) != nil {
			l.Error(err, "Failed to delete HTTPRoute")
			return fmt.Errorf("failed to delete HTTPRoute: %w", err)
		}
		l.Info("HTTPRoute not found (already deleted)")
		return nil
	}

	l.Info("Successfully deleted HTTPRoute")
	return nil
}
