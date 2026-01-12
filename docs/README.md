# Team Operator

The Team Operator is a Kubernetes operator that manages the deployment and configuration of Posit Team products within a Kubernetes cluster.

## Overview

The Team Operator automates the deployment and lifecycle management of:
- **Workbench** - Interactive development environment
- **Connect** - Publishing and sharing platform
- **Package Manager** - Package repository management
- **Chronicle** - Telemetry and monitoring
- **Keycloak** - Authentication and identity management

## Architecture

The operator uses a hierarchical configuration model:

```
Site CRD (single source of truth)
    ├── Connect configuration
    ├── Workbench configuration
    ├── Package Manager configuration
    ├── Chronicle configuration
    └── Keycloak configuration
```

The Site controller watches for Site resources and reconciles product-specific Custom Resources for each enabled product.

## Key Concepts

### Site CRD

The `Site` Custom Resource is the primary configuration point. It contains:
- Global settings (domain, secrets, storage)
- Product-specific configuration sections
- Feature flags and experimental options

### Configuration Propagation

Configuration flows from Site CRD to individual product CRDs:

1. User edits Site spec
2. Site controller detects change
3. Site controller updates product CRs
4. Product controllers reconcile deployments

See [Adding Config Options](../guides/adding-config-options.md) for details on extending configuration.

## Quick Start

### View Sites

```bash
kubectl get sites -n posit-team
```

### Edit a Site

```bash
kubectl edit site main -n posit-team
```

### Check Operator Logs

```bash
kubectl logs -n posit-team deploy/team-operator
```

## Related Documentation

- [Site Management Guide](../guides/product-team-site-management.md) - For product teams
- [Adding Config Options](../guides/adding-config-options.md) - For contributors
