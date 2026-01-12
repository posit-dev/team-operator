---
name: team-operator-config
description: Scaffold a new config option for Posit Team products
---

# Team Operator Config Option Skill

## When to Use

When adding a new configuration option to any Posit Team product:
- Connect
- Workbench
- Package Manager
- Chronicle
- Keycloak

## Information Needed

Before running this skill, know:
1. **Product**: Which product does this config affect?
2. **Site Field Name**: Go-style field name for the Site CRD (e.g., `MaxConnections`)
3. **Product Config Name**: The actual config key the product expects (e.g., `Scheduler.MaxConnections` or `server.max_connections`) - check product documentation
4. **Type**: Go type (string, int, bool, *int, struct)
5. **Description**: What does this config control?
6. **Default**: What's the default value?
7. **Site-Level or Product-Level**: Does this belong on Site or just the product?

**Important**: Product config names are often inconsistent with Site CRD naming. Always verify the expected config path from product documentation or existing examples.

## Config Propagation Pattern

```
site_types.go (SiteSpec)
    -> InternalProductSpec (Site-level config)
        -> site_controller_{product}.go (Propagation logic)
            -> Product CR using PRODUCT'S expected config path
```

## Steps

### Step 1: Add to Site Spec (if site-level)

File: `team-operator/api/core/v1beta1/site_types.go`

Find the relevant `Internal{Product}Spec` struct and add:

```go
// {Description}
// Maps to product config: {ProductConfigName}
// +optional
{SiteFieldName} {Type} `json:"{jsonFieldName},omitempty"`
```

### Step 2: Add to Product Types (if needed)

File: `team-operator/api/core/v1beta1/{product}_types.go`

Find the relevant config struct and add the field. Match the product's expected structure.

### Step 3: Add Propagation Logic

File: `team-operator/internal/controller/core/site_controller_{product}.go`

In the reconcile function, add propagation logic mapping Site field to Product config path:

```go
// For simple types:
// Site: {SiteFieldName} -> Product: {ProductConfigPath}
if site.Spec.{Product}.{SiteFieldName} != {ZeroValue} {
    target{Product}.Spec.Config.{ProductConfigPath} = site.Spec.{Product}.{SiteFieldName}
}

// For pointer types (optional values):
if site.Spec.{Product}.{SiteFieldName} != nil {
    target{Product}.Spec.Config.{ProductConfigPath} = *site.Spec.{Product}.{SiteFieldName}
}
```

### Step 4: Add Tests

File: `team-operator/internal/controller/core/site_controller_{product}_test.go`

Add test case verifying the mapping:

```go
It("should propagate {SiteFieldName} to product config {ProductConfigPath}", func() {
    site := &v1beta1.Site{
        Spec: v1beta1.SiteSpec{
            {Product}: v1beta1.Internal{Product}Spec{
                {SiteFieldName}: {TestValue},
            },
        },
    }

    result := reconcile{Product}(site)

    Expect(result.Spec.Config.{ProductConfigPath}).To(Equal({TestValue}))
})
```

### Step 5: Update Documentation

File: `docs/guides/product-team-site-management.md`

Update the example Site spec to include the new field with a comment.

File: `docs/team-operator/{product}-config.md` (create if needed)

Document both the Site field name AND the product config it maps to.

## Validation Checklist

- [ ] Field has correct JSON tag (camelCase)
- [ ] Product config path matches product's expected configuration
- [ ] Field has kubebuilder validation if needed
- [ ] Default value is handled correctly
- [ ] Propagation respects zero values vs explicit values
- [ ] Test covers positive and negative cases
- [ ] Docs updated with example showing the mapping

## Common Patterns

### Optional Integer (pointer)
```go
// MaxConnections sets the maximum number of connections
// Maps to product config: Scheduler.MaxConnections
// +optional
MaxConnections *int `json:"maxConnections,omitempty"`
```

### Enum (string with validation)
```go
// LogLevel sets the logging verbosity
// Maps to product config: Logging.All.LogLevel
// +kubebuilder:validation:Enum=debug;info;warn;error
// +optional
LogLevel string `json:"logLevel,omitempty"`
```

### Nested Struct
```go
// GPUSettings configures GPU resources
// +optional
GPUSettings *GPUSettings `json:"gpuSettings,omitempty"`

type GPUSettings struct {
    // NvidiaGPULimit sets the GPU limit
    // Maps to product config: Scheduler.NvidiaGPULimit
    // +optional
    NvidiaGPULimit int `json:"nvidiaGpuLimit,omitempty"`
}
```

### Boolean with default false
```go
// EnableFeatureX enables the experimental feature X
// Maps to product config: Features.ExperimentalX
// +optional
EnableFeatureX bool `json:"enableFeatureX,omitempty"`
```

Note: For booleans, `omitempty` will omit `false` values, so only propagate when explicitly `true`.

## File Quick Reference

| Product | Site Types | Product Types | Controller |
|---------|------------|---------------|------------|
| Connect | `site_types.go` (InternalConnectSpec) | `connect_types.go` | `site_controller_connect.go` |
| Workbench | `site_types.go` (InternalWorkbenchSpec) | `workbench_types.go` | `site_controller_workbench.go` |
| Package Manager | `site_types.go` (InternalPackageManagerSpec) | `packagemanager_types.go` | `site_controller_packagemanager.go` |
| Chronicle | `site_types.go` (InternalChronicleSpec) | `chronicle_types.go` | `site_controller_chronicle.go` |
| Keycloak | `site_types.go` (InternalKeycloakSpec) | `keycloak_types.go` | `site_controller_keycloak.go` |

## Finding Product Config Names

To find the expected product config path:
1. Check the product's admin guide or configuration reference
2. Look at existing examples in `site_controller_{product}.go` for similar options
3. Check the product's Helm chart `values.yaml` for config structure
4. Ask the product team if uncertain
