package internal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/rstudio/goex/ptr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// EnsureHTTPRoute creates or updates an HTTPRoute for a product
func EnsureHTTPRoute(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	l logr.Logger,
	owner metav1.Object,
	name, namespace string,
	gatewayName, gatewayNamespace string,
	hostname string,
	backendService string,
	backendPort int32,
	requestHeaders map[string]string,
	responseHeaders map[string]string,
	useSessionPersistence bool,
) error {
	l = l.WithValues(
		"function", "EnsureHTTPRoute",
		"name", name,
		"namespace", namespace,
	)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	l.Info("Creating or updating HTTPRoute")
	_, err := CreateOrUpdateResource(ctx, c, scheme, l, route, owner.(client.Object), func() error {
		// Build filters for header manipulation
		filters := []gatewayv1.HTTPRouteFilter{}

		if len(requestHeaders) > 0 {
			headerFilter := gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{},
				},
			}
			for key, value := range requestHeaders {
				headerFilter.RequestHeaderModifier.Set = append(
					headerFilter.RequestHeaderModifier.Set,
					gatewayv1.HTTPHeader{
						Name:  gatewayv1.HTTPHeaderName(key),
						Value: value,
					},
				)
			}
			filters = append(filters, headerFilter)
		}

		if len(responseHeaders) > 0 {
			headerFilter := gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{},
				},
			}
			for key, value := range responseHeaders {
				headerFilter.ResponseHeaderModifier.Set = append(
					headerFilter.ResponseHeaderModifier.Set,
					gatewayv1.HTTPHeader{
						Name:  gatewayv1.HTTPHeaderName(key),
						Value: value,
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
						Name: gatewayv1.ObjectName(backendService),
						Port: (*gatewayv1.PortNumber)(&backendPort),
					},
				},
			}},
			Filters: filters,
		}

		// Add session persistence if needed
		if useSessionPersistence {
			rule.SessionPersistence = &gatewayv1.SessionPersistence{
				SessionName: ptr.To(name),
				Type:        ptr.To(gatewayv1.CookieBasedSessionPersistence),
			}
		}

		route.Spec = gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Name:      gatewayv1.ObjectName(gatewayName),
					Namespace: (*gatewayv1.Namespace)(&gatewayNamespace),
				}},
			},
			Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(hostname)},
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
