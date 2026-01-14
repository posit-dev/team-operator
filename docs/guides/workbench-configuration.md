# Workbench Configuration Guide

This guide covers comprehensive configuration of Posit Workbench in Team Operator, including all available options, authentication, off-host execution, IDE settings, data integrations, and advanced features.

## Overview

Posit Workbench provides an interactive development environment for data science teams. In Team Operator, Workbench runs on Kubernetes with off-host execution enabled by default, meaning user sessions run as separate Kubernetes Jobs rather than on the Workbench server pod itself.

When configured via a Site resource, Workbench:
- Uses the Kubernetes Job Launcher for session management
- Supports multiple IDEs (RStudio, VS Code, Positron, Jupyter)
- Integrates with Site-level authentication
- Provides load balancing across multiple replicas
- Connects to data platforms like Databricks and Snowflake

## Table of Contents

1. [Basic Configuration](#basic-configuration)
2. [Authentication](#authentication)
3. [Off-Host Execution / Kubernetes Launcher](#off-host-execution--kubernetes-launcher)
4. [IDE Configuration](#ide-configuration)
5. [Data Integrations](#data-integrations)
6. [Session Customization](#session-customization)
7. [Non-Root Execution Mode](#non-root-execution-mode)
8. [Experimental Features](#experimental-features)
9. [Example Configurations](#example-configurations)
10. [Troubleshooting](#troubleshooting)

---

## Basic Configuration

### Image and Resources

Configure the Workbench server image and basic settings:

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: my-site
  namespace: posit-team
spec:
  workbench:
    # Server image (required)
    image: "ghcr.io/posit-dev/workbench:jammy-2024.12.0"

    # Image pull policy
    imagePullPolicy: IfNotPresent

    # Number of replicas (enables load balancing when > 1)
    replicas: 2

    # URL prefix for ingress (default: "workbench")
    domainPrefix: workbench
```

### Licensing

Workbench requires a valid license. Configure via Kubernetes Secret:

```yaml
spec:
  workbench:
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pw.lic
```

License types:
- `FILE`: License file stored in a Kubernetes Secret
- `KEY`: License key as an environment variable

### Volume Configuration

Workbench uses persistent storage for user home directories:

```yaml
spec:
  workbench:
    # Primary volume for /home directories
    volume:
      create: true
      size: "100Gi"
      accessModes:
        - "ReadWriteMany"
      storageClassName: "efs-sc"  # Optional: use specific storage class

    # Additional volumes (mounted to all sessions)
    additionalVolumes:
      - pvcName: project-data
        mountPath: /mnt/projects
        readOnly: false
      - pvcName: shared-datasets
        mountPath: /mnt/datasets
        readOnly: true
```

When `replicas > 1`, a shared storage volume is automatically created at `/mnt/shared-storage` for load balancing state.

### Node Placement

Control where Workbench server pods are scheduled:

```yaml
spec:
  workbench:
    # Node selector for server pods
    nodeSelector:
      node-type: posit-products

    # Tolerations for server pods
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "posit"
        effect: "NoSchedule"
```

### Environment Variables

Add custom environment variables to the Workbench server:

```yaml
spec:
  workbench:
    addEnv:
      R_LIBS_SITE: "/opt/R/libraries"
      MY_CUSTOM_VAR: "value"
```

---

## Authentication

Workbench integrates with Site-level authentication. Supported methods:

### OIDC Authentication

```yaml
spec:
  workbench:
    auth:
      type: "oidc"
      clientId: "workbench-client-id"
      issuer: "https://idp.example.com"

      # Claim mappings
      usernameClaim: "preferred_username"  # Optional

      # Request scopes (optional)
      scopes:
        - "openid"
        - "profile"
        - "email"
```

### SAML Authentication

```yaml
spec:
  workbench:
    auth:
      type: "saml"
      samlMetadataUrl: "https://idp.example.com/metadata"

      # Attribute mappings (optional if using a profile)
      samlIdPAttributeProfile: "azure"  # Use preset profile
      # Or specify custom attributes:
      samlUsernameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
      samlEmailAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
```

### Password Authentication

For development environments only:

```yaml
spec:
  workbench:
    auth:
      type: "password"
```

### User Provisioning

Control automatic user account creation:

```yaml
spec:
  workbench:
    # Automatically create user accounts on first login
    createUsersAutomatically: true

    # Groups with admin dashboard access
    adminGroups:
      - workbench-admin
      - platform-admins

    # Groups with superuser (full) admin access
    adminSuperuserGroups:
      - workbench-superadmin
```

### Custom Login Page

Customize the login page with HTML content:

```yaml
spec:
  workbench:
    authLoginPageHtml: |
      <div style="text-align: center; padding: 20px;">
        <h2>Welcome to Data Science Platform</h2>
        <p>Please log in with your corporate credentials.</p>
      </div>
```

The HTML content is mounted at `/etc/rstudio/login.html` and must be less than 64KB.

---

## Off-Host Execution / Kubernetes Launcher

Off-host execution runs user sessions as Kubernetes Jobs, providing isolation, resource management, and scalability. **This is enabled by default** in Team Operator.

### How It Works

1. User requests a session (RStudio, VS Code, Jupyter, etc.)
2. Workbench's Kubernetes Launcher creates a Kubernetes Job
3. The session runs in its own pod with configured resources
4. Session connects back to Workbench for proxying and management

### Session Images

Configure the container images available for sessions:

```yaml
spec:
  workbench:
    # Default session image
    defaultSessionImage: "ghcr.io/posit-dev/workbench-session:jammy-2024.12.0"

    # Additional session images users can select
    extraSessionImages:
      - "ghcr.io/posit-dev/workbench-session:gpu-2024.12.0"
      - "ghcr.io/posit-dev/workbench-session:ml-2024.12.0"
      - "custom-registry.io/custom-session:latest"
```

### Session Init Containers

Configure init containers that run before session containers:

```yaml
spec:
  workbench:
    # Init container image for sessions
    sessionInitContainerImageName: "busybox"
    sessionInitContainerImageTag: "latest"
```

### Resource Profiles

Resource profiles define CPU and memory allocations that users can select:

```yaml
spec:
  workbench:
    experimentalFeatures:
      resourceProfiles:
        default:
          name: "Small"
          cpus: "1"
          memMb: "2000"
        medium:
          name: "Medium"
          cpus: "2"
          memMb: "4000"
        large:
          name: "Large"
          cpus: "4"
          memMb: "8000"
        gpu:
          name: "GPU Enabled"
          cpus: "4"
          memMb: "16000"
          nvidiaGpus: "1"
          placementConstraints: "node-type:gpu"
```

**Resource Profile Fields:**

| Field | Description |
|-------|-------------|
| `name` | Display name in UI |
| `cpus` | CPU limit |
| `cpusRequest` | CPU request (defaults to ratio of limit) |
| `memMb` | Memory limit in MB |
| `memMbRequest` | Memory request (defaults to ratio of limit) |
| `nvidiaGpus` | NVIDIA GPU count |
| `amdGpus` | AMD GPU count |
| `placementConstraints` | Node selector as `key:value` pairs |

### Request Ratios

Control the ratio of requests to limits for session pods:

```yaml
spec:
  workbench:
    experimentalFeatures:
      # CPU requests = limits * 0.6 (default)
      cpuRequestRatio: "0.6"

      # Memory requests = limits * 0.8 (default)
      memoryRequestRatio: "0.8"
```

### Session Configuration Details

Sessions are configured via launcher templates. The operator manages:

- `job.tpl` - Kubernetes Job template
- `service.tpl` - Service template for session connectivity
- `rstudio-library-templates-data.tpl` - Configuration data injected into templates

---

## IDE Configuration

### RStudio IDE

RStudio is enabled by default. Configure via the Workbench spec:

```yaml
spec:
  workbench:
    experimentalFeatures:
      # First project template path
      firstProjectTemplatePath: "/opt/templates/default-project"

      # Session save behavior: "no", "ask", or "yes"
      sessionSaveActionDefault: "no"  # Recommended for Kubernetes
```

### VS Code / Code Server

```yaml
spec:
  workbench:
    # VS Code extensions to pre-install
    vsCodeExtensions:
      - "ms-python.python"
      - "quarto.quarto"
      - "posit.shiny"
      - "REditorSupport.r"

    # VS Code user settings (JSON)
    vsCodeUserSettings:
      editor.fontSize:
        raw: "14"
      editor.tabSize:
        raw: "2"

    # VS Code-specific settings
    vsCodeConfig:
      enabled: 1  # 1 = enabled (default)
      sessionTimeoutKillHours: 1

    experimentalFeatures:
      # Custom VS Code executable path
      vsCodePath: "/opt/code-server/bin/code-server"

      # Extensions directory for shared extensions
      vsCodeExtensionsDir: "/mnt/extensions/vscode"
```

### Positron IDE

Positron is Posit's next-generation IDE. Enable and configure:

```yaml
spec:
  workbench:
    positronConfig:
      enabled: 1
      exe: "/opt/positron/bin/positron"
      args: "--host=0.0.0.0"

      # Default session image for Positron
      defaultSessionContainerImage: "ghcr.io/posit-dev/positron-session:latest"

      # Additional Positron session images
      sessionContainerImages:
        - "ghcr.io/posit-dev/positron-session:gpu"

      # Session behavior
      sessionNoProfile: 1  # Skip .profile loading
      userDataDir: "/home/{user}/.positron"
      allowFileDownloads: 1
      allowFileUploads: 1
      sessionTimeoutKillHours: 24

      # Positron extensions
      extensions:
        - "posit.positron-r"
        - "posit.positron-python"

      # User settings (JSON)
      userSettings:
        editor.fontSize:
          raw: "14"
```

### Jupyter Notebooks and JupyterLab

```yaml
spec:
  workbench:
    jupyterConfig:
      # Enable Jupyter Notebook Classic
      notebooksEnabled: 1

      # Enable JupyterLab (default: enabled)
      labsEnabled: 1

      # Custom Jupyter executable
      jupyterExe: "/opt/python/bin/jupyter"

      # Version detection (default: "auto")
      labVersion: "auto"
      notebookVersion: "auto"

      # Idle kernel culling (minutes)
      sessionCullMinutes: 120

      # Shutdown after idle (minutes)
      sessionShutdownMinutes: 5

      # Default session image for Jupyter
      defaultSessionContainerImage: "ghcr.io/posit-dev/jupyter-session:latest"
```

---

## Data Integrations

### Databricks Integration

Connect to one or more Databricks workspaces:

```yaml
spec:
  workbench:
    databricks:
      production:
        name: "Production Workspace"
        url: "https://production.cloud.databricks.com"
        clientId: "databricks-app-client-id"
        tenantId: "azure-tenant-id"  # For Azure Databricks

      development:
        name: "Development Workspace"
        url: "https://dev.cloud.databricks.com"
        clientId: "databricks-dev-client-id"

    experimentalFeatures:
      # Force enable Databricks pane even without managed credentials
      databricksForceEnabled: true
```

**Note:** Databricks client secrets must be stored in the site secret vault with keys like `dev-client-secret-{clientId}`.

### Snowflake Integration

```yaml
spec:
  workbench:
    snowflake:
      accountId: "abc12345.us-east-1"
      clientId: "snowflake-oauth-client-id"
```

The Snowflake client secret must be stored in the site secret vault as `snowflake-client-secret`.

### DSN / ODBC Configuration

Mount ODBC data source configurations into sessions:

```yaml
spec:
  workbench:
    experimentalFeatures:
      # Key in the site secret containing odbc.ini content
      dsnSecret: "workbench-odbc-config"
```

The DSN file is mounted at `/etc/odbc.ini` in session pods.

**Example odbc.ini content:**

```ini
[PostgreSQL]
Driver = PostgreSQL
Server = postgres.example.com
Port = 5432
Database = analytics

[Snowflake]
Driver = Snowflake
Server = account.snowflakecomputing.com
Database = ANALYTICS
Schema = PUBLIC
```

---

## Session Customization

### Session Tolerations

Apply tolerations specifically to session pods (not the server):

```yaml
spec:
  workbench:
    # Tolerations for Workbench server pods
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "posit-products"
        effect: "NoSchedule"

    # Tolerations for session pods only
    sessionTolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "workbench-sessions"
        effect: "NoSchedule"
      - key: "nvidia.com/gpu"
        operator: "Exists"
        effect: "NoSchedule"
```

### Session Node Selector

The server-level `nodeSelector` is inherited by sessions. Sessions use placement constraints from resource profiles for additional targeting.

### Session Environment Variables

Inject environment variables into all sessions:

```yaml
spec:
  workbench:
    experimentalFeatures:
      sessionEnvVars:
        - name: "R_LIBS_USER"
          value: "~/R/library"
        - name: "DATABASE_URL"
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: url
```

### Session Service Account

Specify a custom service account for session pods:

```yaml
spec:
  workbench:
    experimentalFeatures:
      sessionServiceAccountName: "workbench-session-sa"
```

### Session Image Pull Policy

Control when session images are pulled:

```yaml
spec:
  workbench:
    experimentalFeatures:
      sessionImagePullPolicy: "Always"  # Always, IfNotPresent, Never
```

### Launcher Environment (PATH)

Customize the PATH for launcher sessions:

```yaml
spec:
  workbench:
    experimentalFeatures:
      launcherEnvPath: "/opt/R/4.3/bin:/opt/python/3.11/bin:/usr/local/bin:/usr/bin:/bin"
```

---

## Non-Root Execution Mode

Enable "maximally rootless" execution for enhanced security:

```yaml
spec:
  workbench:
    experimentalFeatures:
      nonRoot: true
```

When enabled:
- Workbench launcher runs with `unprivileged=1`
- Custom supervisord configuration is deployed
- Secure cookie key file is relocated to `/mnt/secure/rstudio/`
- Launcher configuration is managed via mounted ConfigMaps

**Requirements:**
- Compatible Workbench image version
- Proper file permissions on mounted volumes

**Limitations:**
- Some features requiring root privileges may not work
- Not all Workbench functionality has been tested in non-root mode

---

## Experimental Features

The `experimentalFeatures` section contains advanced options. These are subject to change:

```yaml
spec:
  workbench:
    experimentalFeatures:
      # Enable managed credential jobs
      enableManagedCredentialJobs: true

      # Non-root operation
      nonRoot: false

      # Privileged sessions (for Docker-in-Docker)
      privilegedSessions: false

      # Web server thread pool size (default: 16)
      wwwThreadPoolSize: 32

      # Session proxy timeout (default: 30 seconds)
      launcherSessionsProxyTimeoutSeconds: 60

      # Force admin UI even on Kubernetes
      forceAdminUiEnabled: true

      # Chronicle sidecar API key injection
      chronicleSidecarProductApiKeyEnabled: true
```

### Workbench API Settings

Enable the Workbench REST API:

```yaml
spec:
  workbench:
    apiSettings:
      workbenchApiEnabled: 1
      workbenchApiAdminEnabled: 1
      workbenchApiSuperAdminEnabled: 1
```

---

## Example Configurations

### Minimal Development Setup

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: dev
  namespace: posit-team
spec:
  domain: dev.example.com

  secret:
    type: "kubernetes"
    vaultName: "dev-secrets"

  workbench:
    image: "ghcr.io/posit-dev/workbench:jammy-2024.12.0"
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pw.lic
    auth:
      type: "password"
    createUsersAutomatically: true
```

### Production Multi-IDE Setup

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: production
  namespace: posit-team
spec:
  domain: posit.example.com

  secret:
    type: "aws"
    vaultName: "production-secrets"

  volumeSource:
    type: fsx-zfs
    volumeId: fsvol-abcdef123456
    dnsName: fs-abcdef123456.fsx.us-east-1.amazonaws.com

  workbench:
    image: "ghcr.io/posit-dev/workbench:jammy-2024.12.0"
    replicas: 3

    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pw.lic

    auth:
      type: "oidc"
      clientId: "workbench-prod"
      issuer: "https://idp.example.com"

    createUsersAutomatically: true
    adminGroups:
      - platform-admins
    adminSuperuserGroups:
      - workbench-superadmins

    defaultSessionImage: "ghcr.io/posit-dev/workbench-session:jammy-2024.12.0"
    extraSessionImages:
      - "ghcr.io/posit-dev/workbench-session:gpu-2024.12.0"

    nodeSelector:
      node-type: posit-products

    sessionTolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "workbench-sessions"
        effect: "NoSchedule"

    vsCodeExtensions:
      - "ms-python.python"
      - "quarto.quarto"

    positronConfig:
      enabled: 1

    databricks:
      workspace:
        name: "Analytics Workspace"
        url: "https://analytics.cloud.databricks.com"
        clientId: "databricks-client"

    experimentalFeatures:
      resourceProfiles:
        default:
          name: "Small (1 CPU, 2GB)"
          cpus: "1"
          memMb: "2000"
        medium:
          name: "Medium (2 CPU, 4GB)"
          cpus: "2"
          memMb: "4000"
        large:
          name: "Large (4 CPU, 8GB)"
          cpus: "4"
          memMb: "8000"
        gpu:
          name: "GPU (4 CPU, 16GB, 1 GPU)"
          cpus: "4"
          memMb: "16000"
          nvidiaGpus: "1"
```

### GPU-Enabled Data Science Platform

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: ml-platform
  namespace: posit-team
spec:
  domain: ml.example.com

  workbench:
    image: "ghcr.io/posit-dev/workbench:jammy-2024.12.0"
    replicas: 2

    defaultSessionImage: "ghcr.io/posit-dev/workbench-session:ml-2024.12.0"
    extraSessionImages:
      - "ghcr.io/posit-dev/workbench-session:gpu-pytorch"
      - "ghcr.io/posit-dev/workbench-session:gpu-tensorflow"

    sessionTolerations:
      - key: "nvidia.com/gpu"
        operator: "Exists"
        effect: "NoSchedule"

    experimentalFeatures:
      resourceProfiles:
        cpu-small:
          name: "CPU Small"
          cpus: "2"
          memMb: "4000"
        cpu-large:
          name: "CPU Large"
          cpus: "8"
          memMb: "32000"
        gpu-single:
          name: "Single GPU"
          cpus: "4"
          memMb: "32000"
          nvidiaGpus: "1"
          placementConstraints: "node-type:gpu"
        gpu-multi:
          name: "Multi GPU"
          cpus: "8"
          memMb: "64000"
          nvidiaGpus: "4"
          placementConstraints: "node-type:gpu-multi"
```

---

## Troubleshooting

### Common Issues

#### Sessions Not Starting

1. **Check launcher logs:**
   ```bash
   kubectl logs -n posit-team deploy/<site-name>-workbench | grep -i launcher
   ```

2. **Verify session service account exists:**
   ```bash
   kubectl get sa <site-name>-workbench-session -n posit-team
   ```

3. **Check for pending session jobs:**
   ```bash
   kubectl get jobs -n posit-team -l posit.team/component=workbench-session
   ```

4. **Verify session image is pullable:**
   ```bash
   kubectl run test --image=<session-image> --rm -it --command -- echo "Success"
   ```

#### Authentication Failures

1. **Check OIDC configuration:**
   - Verify issuer URL is accessible from the cluster
   - Confirm client ID matches IdP configuration
   - Check that redirect URIs are configured in IdP

2. **View authentication logs:**
   ```bash
   kubectl logs -n posit-team deploy/<site-name>-workbench | grep -i auth
   ```

3. **Verify secrets exist:**
   ```bash
   kubectl get secret <site-name>-workbench-config -n posit-team
   ```

#### Session Resource Issues

1. **Check resource profile configuration:**
   ```bash
   kubectl get configmap <site-name>-workbench -n posit-team -o yaml | grep -A 50 "launcher.kubernetes.resources.conf"
   ```

2. **Verify nodes have capacity:**
   ```bash
   kubectl describe nodes | grep -A 10 "Allocated resources"
   ```

3. **Check session pod events:**
   ```bash
   kubectl describe pod <session-pod-name> -n posit-team
   ```

#### Volume Mount Issues

1. **Verify PVC exists and is bound:**
   ```bash
   kubectl get pvc -n posit-team | grep workbench
   ```

2. **Check volume permissions in session:**
   ```bash
   kubectl exec -it <session-pod> -n posit-team -- ls -la /home
   ```

3. **Verify storage class supports RWX:**
   ```bash
   kubectl get storageclass <storage-class-name> -o yaml
   ```

### Useful Commands

```bash
# List all Workbench resources
kubectl get workbench -n posit-team

# Describe Workbench configuration
kubectl describe workbench <site-name> -n posit-team

# View Workbench ConfigMap
kubectl get configmap <site-name>-workbench -n posit-team -o yaml

# Check session template ConfigMap
kubectl get configmap <site-name>-workbench-templates -n posit-team -o yaml

# List active sessions
kubectl get jobs -n posit-team -l posit.team/component=workbench-session

# View session logs
kubectl logs job/<session-job-name> -n posit-team

# Force restart Workbench
kubectl rollout restart deploy/<site-name>-workbench -n posit-team
```

### Log Levels

Enable debug logging for troubleshooting:

```yaml
spec:
  debug: true
  logFormat: json  # Optional: use JSON for log aggregation
```

Debug logging increases verbosity for:
- Launcher operations
- Authentication flows
- Session lifecycle events
- Database operations

---

## Related Documentation

- [Site Management Guide](product-team-site-management.md)
- [Adding Config Options](adding-config-options.md) - For contributors extending Workbench configuration
- [Posit Workbench Admin Guide](https://docs.posit.co/ide/server-pro/)
- [Kubernetes Job Launcher Documentation](https://docs.posit.co/ide/server-pro/integration/launcher-kubernetes.html)
