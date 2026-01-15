# Adding Configuration Options to Team Operator

This guide walks through the process of adding new configuration options to Posit Team products managed by the Team Operator. It covers the complete flow from Site CRD to product-specific configuration.

## Configuration Architecture Overview

The Team Operator uses a hierarchical configuration model:

```
Site CRD (user-facing)
    |
    v
Internal{Product}Spec (in site_types.go)
    |
    v
site_controller_{product}.go (propagation logic)
    |
    v
Product CR (Connect, Workbench, PackageManager, etc.)
    |
    v
Product Controller (generates actual config files)
```

### Key Concepts

1. **Site CRD**: The primary user-facing resource. Users configure their entire Posit Team deployment through a single Site resource.

2. **Internal{Product}Spec**: Nested structs within SiteSpec that contain product-specific configuration at the Site level.

3. **Product CRs**: Individual Custom Resources (Connect, Workbench, etc.) created by the Site controller. These are implementation details users typically don't interact with directly.

4. **Propagation**: The Site controller maps Site-level configuration to the appropriate Product CR fields.

## Step-by-Step: Adding a New Config Option

### Prerequisites

Before adding a config option, gather the following:

| Item | Description | Example |
|------|-------------|---------|
| Product | Which product does this config affect? | Workbench |
| Site Field Name | Go-style field name (PascalCase) | `MaxConnections` |
| Product Config Path | The actual config key the product expects | `Scheduler.MaxConnections` |
| Go Type | string, int, bool, *int, struct, etc. | `*int` |
| Description | What does this config control? | "Maximum concurrent connections" |
| Default Value | What's the default if not specified? | `100` |

### Step 1: Add Field to Site Types

**File**: `api/core/v1beta1/site_types.go`

Find the appropriate `Internal{Product}Spec` struct and add your field.

```go
type InternalConnectSpec struct {
    // ... existing fields ...

    // MaxConnections sets the maximum number of concurrent connections
    // Maps to product config: Scheduler.MaxConnections
    // +optional
    MaxConnections *int `json:"maxConnections,omitempty"`
}
```

#### Field Documentation Pattern

Always include:
1. A description of what the field controls
2. A comment showing the product config path it maps to
3. The `+optional` marker for optional fields
4. Kubebuilder validation markers if applicable

```go
// Description of the field and what it controls
// Maps to product config: Section.ConfigKey
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=1000
// +optional
FieldName Type `json:"fieldName,omitempty"`
```

### Step 2: Add Field to Product Types (If Needed)

**File**: `api/core/v1beta1/{product}_types.go`

If the product needs the config in its spec (for the product controller to use), add it to the appropriate config struct.

```go
// In connect_types.go
type ConnectSchedulerConfig struct {
    // ... existing fields ...

    // MaxConnections sets the maximum concurrent connections
    // +optional
    MaxConnections int `json:"maxConnections,omitempty"`
}
```

### Step 3: Add Propagation Logic

**File**: `internal/controller/core/site_controller_{product}.go`

In the `reconcile{Product}` function, add logic to map the Site field to the Product CR.

#### Pattern 1: Simple Value Propagation

```go
func (r *SiteReconciler) reconcileConnect(
    ctx context.Context,
    req controllerruntime.Request,
    site *v1beta1.Site,
    // ... other params
) error {
    // ... existing code ...

    targetConnect := v1beta1.Connect{
        // ... existing fields ...
        Spec: v1beta1.ConnectSpec{
            Config: v1beta1.ConnectConfig{
                Scheduler: &v1beta1.ConnectSchedulerConfig{
                    // ... existing fields ...
                },
            },
        },
    }

    // Propagate MaxConnections if set
    if site.Spec.Connect.MaxConnections != nil {
        targetConnect.Spec.Config.Scheduler.MaxConnections = *site.Spec.Connect.MaxConnections
    }

    // ... rest of function
}
```

#### Pattern 2: Conditional/Nested Propagation

For fields that require nil-safety or conditional logic:

```go
// Ensure parent structs exist before setting
if site.Spec.Workbench.ExperimentalFeatures != nil {
    if site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize != nil {
        threadPoolSize = *site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize
    }
}

// Apply to target
targetWorkbench.Spec.Config.RServer.WwwThreadPoolSize = threadPoolSize
```

#### Pattern 3: Struct Propagation

For nested configuration objects:

```go
// Propagate entire settings struct
if site.Spec.Connect.GPUSettings != nil {
    if site.Spec.Connect.GPUSettings.NvidiaGPULimit > 0 {
        targetConnect.Spec.Config.Scheduler.NvidiaGPULimit = site.Spec.Connect.GPUSettings.NvidiaGPULimit
    }
    if site.Spec.Connect.GPUSettings.MaxNvidiaGPULimit > 0 {
        targetConnect.Spec.Config.Scheduler.MaxNvidiaGPULimit = site.Spec.Connect.GPUSettings.MaxNvidiaGPULimit
    }
}
```

### Step 4: Update Product Controller (If Needed)

**File**: `internal/controller/core/{product}_controller.go`

If the product controller needs to use the new config to generate configuration files, update the relevant config generation logic.

```go
// Example: Adding to a config file generation
func (r *ConnectReconciler) generateConfig(connect *v1beta1.Connect) string {
    cfg := connect.Spec.Config

    // ... existing config generation ...

    if cfg.Scheduler.MaxConnections > 0 {
        // Add to generated config
    }
}
```

### Step 5: Add Tests

**File**: `internal/controller/core/site_test.go`

Add tests to verify propagation works correctly.

```go
func TestSiteReconciler_MaxConnections(t *testing.T) {
    siteName := "max-connections-site"
    siteNamespace := "posit-team"
    site := defaultSite(siteName)

    maxConn := 500
    site.Spec.Connect.MaxConnections = &maxConn

    cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
    assert.Nil(t, err)

    testConnect := getConnect(t, cli, siteNamespace, siteName)

    assert.Equal(t, 500, testConnect.Spec.Config.Scheduler.MaxConnections)
}

func TestSiteReconciler_MaxConnections_Default(t *testing.T) {
    siteName := "default-connections-site"
    siteNamespace := "posit-team"
    site := defaultSite(siteName)
    // Don't set MaxConnections - test default behavior

    cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
    assert.Nil(t, err)

    testConnect := getConnect(t, cli, siteNamespace, siteName)

    // Verify default value or zero value behavior
    assert.Equal(t, 0, testConnect.Spec.Config.Scheduler.MaxConnections)
}
```

### Step 6: Regenerate Code

After modifying types, regenerate the CRD manifests:

```bash
just generate
just manifests
```

## Common Type Patterns

### Optional Integer (Pointer)

Use pointers for optional numeric values where zero is a valid setting:

```go
// MaxWorkers sets the maximum number of workers
// Maps to product config: Scheduler.MaxWorkers
// +optional
MaxWorkers *int `json:"maxWorkers,omitempty"`
```

Propagation:
```go
if site.Spec.Product.MaxWorkers != nil {
    target.Spec.Config.Scheduler.MaxWorkers = *site.Spec.Product.MaxWorkers
}
```

### Enum (String with Validation)

```go
// LogLevel sets the logging verbosity
// Maps to product config: Logging.All.LogLevel
// +kubebuilder:validation:Enum=debug;info;warn;error
// +optional
LogLevel string `json:"logLevel,omitempty"`
```

### Boolean with Default False

```go
// EnableFeatureX enables experimental feature X
// Maps to product config: Features.ExperimentalX
// +optional
EnableFeatureX bool `json:"enableFeatureX,omitempty"`
```

**Note**: With `omitempty`, false values are omitted from JSON. Only propagate when explicitly true:

```go
if site.Spec.Product.EnableFeatureX {
    target.Spec.Config.Features.ExperimentalX = true
}
```

### Nested Struct

```go
// GPUSettings configures GPU resources for sessions
// +optional
GPUSettings *GPUSettings `json:"gpuSettings,omitempty"`

type GPUSettings struct {
    // NvidiaGPULimit sets the default NVIDIA GPU limit
    // Maps to product config: Scheduler.NvidiaGPULimit
    // +optional
    NvidiaGPULimit int `json:"nvidiaGPULimit,omitempty"`

    // MaxNvidiaGPULimit sets the maximum NVIDIA GPU limit
    // Maps to product config: Scheduler.MaxNvidiaGPULimit
    // +optional
    MaxNvidiaGPULimit int `json:"maxNvidiaGPULimit,omitempty"`
}
```

### String Slice

```go
// AdminGroups specifies groups with admin access
// +optional
AdminGroups []string `json:"adminGroups,omitempty"`
```

Propagation (joining for product config):
```go
adminGroup := "default-admin"
if len(site.Spec.Workbench.AdminGroups) > 0 {
    adminGroup = strings.Join(site.Spec.Workbench.AdminGroups, ",")
}
targetWorkbench.Spec.Config.RServer.AdminGroup = adminGroup
```

### Map of Strings

```go
// AddEnv adds arbitrary environment variables
// +optional
AddEnv map[string]string `json:"addEnv,omitempty"`
```

## File Reference

| Product | Site Types Struct | Product Types File | Controller |
|---------|-------------------|-------------------|------------|
| Connect | `InternalConnectSpec` | `connect_types.go` | `site_controller_connect.go` |
| Workbench | `InternalWorkbenchSpec` | `workbench_types.go` | `site_controller_workbench.go` |
| Package Manager | `InternalPackageManagerSpec` | `packagemanager_types.go` | `site_controller_package_manager.go` |
| Chronicle | `InternalChronicleSpec` | `chronicle_types.go` | `site_controller_chronicle.go` |
| Keycloak | `InternalKeycloakSpec` | `keycloak_types.go` | `site_controller_keycloak.go` |
| Flightdeck | `InternalFlightdeckSpec` | `flightdeck_types.go` | `site_controller_flightdeck.go` |

## Validation Checklist

Before submitting your PR, verify:

- [ ] Field has correct JSON tag (camelCase)
- [ ] Field has descriptive comment including product config mapping
- [ ] Field has `+optional` marker if optional
- [ ] Field has kubebuilder validation markers if applicable
- [ ] Propagation logic handles nil/zero values correctly
- [ ] Propagation respects parent struct nil-safety
- [ ] Test covers the happy path (value is propagated)
- [ ] Test covers the default case (value not set)
- [ ] `just generate` and `just manifests` run without errors
- [ ] `just test` passes
- [ ] Product config name matches product documentation

## Common Pitfalls

### 1. Forgetting Nil Checks

**Wrong**:
```go
// Panic if ExperimentalFeatures is nil
threadPoolSize := *site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize
```

**Right**:
```go
if site.Spec.Workbench.ExperimentalFeatures != nil &&
   site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize != nil {
    threadPoolSize = *site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize
}
```

### 2. Wrong JSON Tag Case

**Wrong**:
```go
MaxConnections *int `json:"MaxConnections,omitempty"` // PascalCase
```

**Right**:
```go
MaxConnections *int `json:"maxConnections,omitempty"` // camelCase
```

### 3. Missing omitempty

Including `omitempty` prevents zero values from appearing in serialized YAML:

```go
// Without omitempty, "enabled: false" appears in output
Enabled bool `json:"enabled"`

// With omitempty, false is omitted entirely
Enabled bool `json:"enabled,omitempty"`
```

### 4. Product Config Name Mismatch

Always verify the product config path matches what the product expects:
- Check product admin guides
- Look at existing examples in the codebase
- Test with the actual product

### 5. Not Regenerating CRDs

After modifying types, always run:
```bash
just generate
just manifests
```

### 6. Overwriting Existing Config

Be careful not to overwrite configuration that may have been set elsewhere:

**Wrong**:
```go
// Overwrites any existing Scheduler config
targetConnect.Spec.Config.Scheduler = &v1beta1.ConnectSchedulerConfig{
    MaxConnections: maxConn,
}
```

**Right**:
```go
// Preserves existing config, only sets the new field
if targetConnect.Spec.Config.Scheduler == nil {
    targetConnect.Spec.Config.Scheduler = &v1beta1.ConnectSchedulerConfig{}
}
targetConnect.Spec.Config.Scheduler.MaxConnections = maxConn
```

## Finding Product Config Names

To determine the correct product config path:

1. **Check product documentation**: Admin guides typically list all configuration options
2. **Look at existing examples**: Search `site_controller_{product}.go` for similar options
3. **Check Helm charts**: Product Helm chart `values.yaml` files show config structure
4. **Ask the product team**: When uncertain, verify with the product team

## Example: Complete Walkthrough

Let's add a `SessionTimeout` config to Connect that maps to `Scheduler.SessionTimeout`.

### 1. Add to site_types.go

```go
type InternalConnectSpec struct {
    // ... existing fields ...

    // SessionTimeout sets the session timeout in seconds
    // Maps to product config: Scheduler.SessionTimeout
    // +kubebuilder:validation:Minimum=60
    // +optional
    SessionTimeout *int `json:"sessionTimeout,omitempty"`
}
```

### 2. Add to connect_types.go

```go
type ConnectSchedulerConfig struct {
    // ... existing fields ...

    // SessionTimeout in seconds
    SessionTimeout int `json:"sessionTimeout,omitempty"`
}
```

### 3. Add propagation in site_controller_connect.go

```go
func (r *SiteReconciler) reconcileConnect(...) error {
    // ... existing setup ...

    targetConnect := v1beta1.Connect{
        Spec: v1beta1.ConnectSpec{
            Config: v1beta1.ConnectConfig{
                Scheduler: &v1beta1.ConnectSchedulerConfig{
                    // ... existing fields ...
                },
            },
        },
    }

    // Add after targetConnect initialization
    if site.Spec.Connect.SessionTimeout != nil {
        targetConnect.Spec.Config.Scheduler.SessionTimeout = *site.Spec.Connect.SessionTimeout
    }

    // ... rest of function ...
}
```

### 4. Add test in site_test.go

```go
func TestSiteReconciler_ConnectSessionTimeout(t *testing.T) {
    siteName := "session-timeout-site"
    siteNamespace := "posit-team"
    site := defaultSite(siteName)

    timeout := 300
    site.Spec.Connect.SessionTimeout = &timeout

    cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
    assert.Nil(t, err)

    testConnect := getConnect(t, cli, siteNamespace, siteName)

    assert.Equal(t, 300, testConnect.Spec.Config.Scheduler.SessionTimeout)
}
```

### 5. Regenerate and test

```bash
just generate
just manifests
just test
```

## Related Documentation

- [Kubernetes Custom Resource Definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/)
- [Kubebuilder Markers](https://book.kubebuilder.io/reference/markers.html)
- [Team Operator README](/README.md)
