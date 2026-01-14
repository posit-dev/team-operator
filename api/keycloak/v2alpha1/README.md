# Keycloak CRDs

This package contains Go struct definitions that mirror the Keycloak Operator's CRD schema.

## Background

The [Keycloak Operator](https://www.keycloak.org/operator/installation) is written in Java and provides its own CRDs for managing Keycloak instances. Team Operator needs to create `Keycloak` custom resources to integrate authentication with Posit Team products.

## Why Manual Struct Definitions?

Kubebuilder and controller-runtime typically generate CRDs *from* Go structs. However, the Keycloak Operator works in reverse - it defines CRDs in Java, and we consume them. Since there's no standard tooling to generate Go structs from external CRDs, we've manually built these structs to match the Keycloak Operator's CRD spec.

## Files

- `keycloak_types.go` - Go struct definitions for `Keycloak` and `KeycloakSpec`
- `groupversion_info.go` - API group and version registration
- `zz_generated.deepcopy.go` - Auto-generated deep copy methods

## Usage

These types are used by the Site controller to create Keycloak instances when SSO is configured:

```go
import keycloakv2alpha1 "github.com/posit-dev/team-operator/api/keycloak/v2alpha1"

keycloak := &keycloakv2alpha1.Keycloak{
    Spec: keycloakv2alpha1.KeycloakSpec{
        Hostname: &keycloakv2alpha1.KeycloakHostnameSpec{
            Hostname: "auth.example.com",
        },
        Instances: 1,
    },
}
```

## Maintenance

When upgrading the Keycloak Operator, verify that the struct definitions in `keycloak_types.go` still match their CRD schema. Check the [Keycloak Operator documentation](https://www.keycloak.org/operator/basic-deployment) for schema changes.
