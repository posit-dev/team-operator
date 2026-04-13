---
title: Workbench Configuration
description: Configuration of Posit Workbench in Team Operator including authentication, off-host execution, and IDE settings
---

# Workbench Configuration Guide

Posit Workbench is an interactive development environment for data science teams, supporting RStudio, VS Code, Positron, and Jupyter in a single platform. When you enable Workbench in a Site resource, the operator handles the Kubernetes deployment, session job management, ingress, storage, and configuration — you specify intent in the Site spec and the operator reconciles the rest.

The most common things you will configure here are: the container image and replica count, a license, an authentication method, and the default session image for off-host execution. For production deployments you will also want to set resource profiles and, if applicable, data integrations such as Databricks or Snowflake.

## Overview

In Team Operator, Workbench runs on Kubernetes with off-host execution enabled by default. User sessions run as separate Kubernetes Jobs rather than on the Workbench server pod itself, providing resource isolation and scalability across the cluster.

When configured via a Site resource, Workbench does the following:
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
8. [SCIM User Provisioning](#scim-user-provisioning)
9. [Audited Jobs](#audited-jobs)
10. [Additional Configuration Files](#additional-configuration-files)
11. [Pod Disruption Budgets](#pod-disruption-budgets)
12. [Experimental Features](#experimental-features)
13. [Example Configurations](#example-configurations)
14. [Troubleshooting](#troubleshooting)

---

## Basic Configuration

This section covers the foundational fields every Workbench deployment needs: lifecycle controls, the server image, licensing, storage, and node placement.

### Enabling/Disabling Workbench

Workbench can be suspended or permanently torn down using the `enabled` and `teardown` fields.

#### Suspending Workbench (non-destructive)

Setting `enabled: false` suspends Workbench: the Deployment, Service, and Ingress are removed, but the PVC, database, and secrets are preserved. Re-enabling restores full service with all existing data intact.

```yaml
spec:
  workbench:
    enabled: false   # suspend — data is preserved
```

**When to use `enabled: false`:**

- Customer does not have a Workbench license yet — deploy the site without Workbench and enable it once a license is purchased
- Temporarily pause Workbench during a maintenance window or cost-saving period
- Stop Workbench while retaining all user home directories and configuration for a possible return

**Re-enabling Workbench** after a suspend is as simple as removing the field or setting it back to `true`:

```yaml
spec:
  workbench:
    enabled: true   # or omit the field entirely — defaults to true
```

#### Tearing down Workbench (destructive)

To permanently destroy all Workbench resources — including the database, secrets, and PVC — set both `enabled: false` and `teardown: true`:

```yaml
spec:
  workbench:
    enabled: false
    teardown: true   # DESTRUCTIVE: deletes database, secrets, and PVC
```

**This is irreversible.** Re-enabling Workbench after a teardown starts completely fresh with a new empty database and no prior user home directories or configuration.

**When to use `teardown: true`:**

- Permanently decommissioning Workbench with no intent to restore data
- Reclaiming cluster storage after migrating to a different Workbench instance
- Explicitly wiping Workbench to start fresh

> **Note:** `teardown: true` has no effect while `enabled` is `true` or unset. You must set `enabled: false` first.

---

### Image and Resources

The `image` field controls which Workbench server version runs. Setting `replicas` above 1 enables load balancing; the operator automatically provisions a shared storage volume at `/mnt/shared-storage` when you do.

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

Workbench requires a valid license to start. Store the license file or key in a Kubernetes Secret and reference it here. The operator mounts the secret into the Workbench pod at startup.

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

Workbench stores user home directories on a persistent volume. For multi-replica deployments, the storage class must support `ReadWriteMany` access (for example, EFS on AWS or Azure NetApp Files on Azure). You can also mount additional volumes into every session pod via `additionalVolumes`.

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

Use `nodeSelector` and `tolerations` to constrain which nodes the Workbench server pod runs on. These settings apply to the server pod; session pods inherit the node selector and can be further targeted via resource profile placement constraints.

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

Use `addEnv` to inject environment variables into the Workbench server container. These are set on the server pod itself; for variables needed inside user sessions, see the `sessionEnvVars` option under [Session Customization](#session-customization).

```yaml
spec:
  workbench:
    addEnv:
      R_LIBS_SITE: "/opt/R/libraries"
      MY_CUSTOM_VAR: "value"
```

---

## Authentication

Workbench integrates with Site-level authentication. For production deployments, OIDC is the recommended approach as it supports group-based access control and integrates with most enterprise identity providers. SAML is available for environments where OIDC is not supported. Password authentication is suitable only for local development.

Supported methods:

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

Control whether Workbench automatically creates local user accounts on first login, and which IdP groups receive admin access.

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

You can inject custom HTML into the Workbench login page — useful for adding branding, legal notices, or usage guidelines. The HTML is mounted at `/etc/rstudio/login.html` and must be under 64KB.

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

Off-host execution is one of the most important aspects of running Workbench on Kubernetes. Rather than running user sessions directly on the server pod, the Kubernetes Launcher creates a dedicated Job for each session. This gives each user their own isolated compute environment with configurable CPU, memory, and GPU resources.

Off-host execution runs user sessions as Kubernetes Jobs, providing isolation, resource management, and scalability. This is enabled by default in Team Operator.

### How It Works

1. User requests a session (RStudio, VS Code, Jupyter, etc.)
2. Workbench's Kubernetes Launcher creates a Kubernetes Job
3. The session runs in its own pod with configured resources
4. Session connects back to Workbench for proxying and management

### Session Images

The `defaultSessionImage` is the container image used for new sessions. You can offer additional images via `extraSessionImages`; users will see them as selectable options in the session launch dialog.

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

Resource profiles let users choose their compute allocation when launching a session. Define as many profiles as you need; the `default` key specifies what users see if they do not make a selection. Profiles support CPU, memory, NVIDIA and AMD GPUs, and placement constraints for targeting specific node types.

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

Kubernetes distinguishes between resource requests (used for scheduling) and limits (enforced at runtime). By default the operator sets requests as a fraction of limits, which allows the scheduler to bin-pack sessions efficiently without over-provisioning. Adjust these ratios if your workloads have bursty or consistent resource usage patterns.

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

Sessions are configured via launcher templates. The operator manages these files:

- `job.tpl` - Kubernetes Job template
- `service.tpl` - Service template for session connectivity
- `rstudio-library-templates-data.tpl` - Configuration data injected into templates

---

## IDE Configuration

Workbench supports four IDEs: RStudio (the default), VS Code, Positron, and Jupyter. All run as off-host sessions via the Kubernetes Launcher. Each IDE has its own configuration section below; most production deployments enable at least RStudio and one of the Python-focused IDEs.

### RStudio IDE

RStudio is enabled by default. You can customize project templates and session save behavior for a better Kubernetes experience.

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

VS Code runs via Code Server. You can pre-install extensions, set default user settings, and configure a shared extensions directory — useful when multiple users benefit from the same tooling without downloading it on every session start.

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

Positron is Posit's next-generation IDE for data science, built on a VS Code foundation with deep R and Python integration. Enable it alongside other IDEs; users can choose which to launch at session start.

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

Configure Jupyter Notebook Classic and JupyterLab, including idle kernel culling to reclaim cluster resources when sessions are left idle.

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

These integrations surface managed credentials inside Workbench sessions, so users authenticate to external data platforms through the IDE rather than managing credentials manually.

### Databricks Integration

Connect to one or more Databricks workspaces. The operator injects OAuth credentials into sessions so users can authenticate without managing secrets directly. Client secrets must be stored in the site secret vault.

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

Configure OAuth-based Snowflake connectivity. The client secret is read from the site secret vault under the key `snowflake-client-secret`.

```yaml
spec:
  workbench:
    snowflake:
      accountId: "abc12345.us-east-1"
      clientId: "snowflake-oauth-client-id"
```

The Snowflake client secret must be stored in the site secret vault as `snowflake-client-secret`.

### DSN / ODBC Configuration

Mount an `odbc.ini` file into every session pod to make ODBC data sources available to R and Python without users needing to configure connections themselves. Store the file contents as a key in your site secret vault and reference it here.

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

These settings control the Kubernetes configuration of session pods specifically, separate from the Workbench server pod. They are useful when your cluster has dedicated node pools for interactive sessions or when sessions need access to resources (such as GPUs or shared data) that the server itself does not.

### Session Tolerations

The server-level `tolerations` apply to the Workbench server pod. Use `sessionTolerations` to allow session pods to be scheduled on nodes with specific taints — for example, GPU nodes or dedicated session compute nodes.

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

The server-level `nodeSelector` is inherited by session pods. For more granular targeting — for example, routing GPU sessions to GPU nodes — use `placementConstraints` within individual resource profiles instead.

### Session Environment Variables

Inject environment variables into all session pods. You can reference Kubernetes Secrets or ConfigMaps, making this a good way to provide database URLs, API keys, or shared configuration without hard-coding values into session images.

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

By default, session pods use the service account created by the operator. Override this to grant sessions access to AWS IAM roles, Kubernetes RBAC permissions, or other cluster resources that require a specific service account.

```yaml
spec:
  workbench:
    experimentalFeatures:
      sessionServiceAccountName: "workbench-session-sa"
```

### Session Image Pull Policy

Controls when Kubernetes pulls the session container image. Use `Always` when iterating on custom session images; use `IfNotPresent` in production to avoid unnecessary pulls on every session start.

```yaml
spec:
  workbench:
    experimentalFeatures:
      sessionImagePullPolicy: "Always"  # Always, IfNotPresent, Never
```

### Launcher Environment (PATH)

Override the PATH variable for launcher sessions. This is useful when R or Python installations live outside the default system paths and need to be discoverable by the launcher.

```yaml
spec:
  workbench:
    experimentalFeatures:
      launcherEnvPath: "/opt/R/4.3/bin:/opt/python/3.11/bin:/usr/local/bin:/usr/bin:/bin"
```

### Session Config (Advanced)

For fine-grained control over session pod configuration — labels, annotations, volumes, security contexts, sidecar containers, and dynamic labeling — use the `sessionConfig` block. This is the same structure used by Connect's session configuration.

```yaml
spec:
  workbench:
    sessionConfig:
      # Service configuration for session networking
      service:
        labels:
          custom-label: value

      # Pod configuration for session jobs
      pod:
        annotations:
          prometheus.io/scrape: "true"

        labels:
          team: data-science

        # Dynamic labels generated from runtime session fields (requires template v2.5.0+)
        dynamicLabels:
          - field: "user"
            labelKey: "posit.team/session-user"
          - field: "args"
            match: "--r-version=([0-9.]+)"
            trimPrefix: "--r-version="
            labelPrefix: "posit.team/r-version-"
            labelValue: "true"

        # Additional volumes
        volumes:
          - name: shared-data
            persistentVolumeClaim:
              claimName: shared-data-pvc
        volumeMounts:
          - name: shared-data
            mountPath: /mnt/shared

        # Node selection and tolerations
        nodeSelector:
          workload: workbench-sessions
        tolerations:
          - key: "dedicated"
            operator: "Equal"
            value: "workbench-sessions"
            effect: "NoSchedule"

      # Job-level labels
      job:
        labels:
          job-type: workbench-session
```

**Dynamic Labels** allow the launcher to stamp session pods with labels derived from runtime context (the username, R version, session arguments, etc.). Rules use either a direct field-to-label mapping (`labelKey`) or a regex pattern (`match` + `labelPrefix`). At most 20 rules and 200 total regex-matched labels are applied per session; excess matches are dropped and a `posit.team/dynamic-label-cap-reached` annotation is set on the pod.

---

## SCIM User Provisioning

SCIM (System for Cross-domain Identity Management) allows your IdP to automatically provision, update, and deprovision user accounts in Workbench. This requires SSO (OIDC or SAML) to be configured first.

```yaml
spec:
  workbench:
    scim:
      # Enable SCIM user provisioning
      enabled: true

      # Optional: name of an existing Kubernetes Secret containing the SCIM bearer token.
      # The secret must have a key named "token".
      # If omitted, the operator generates a random token stored as "<workbench-name>-scim-token".
      tokenSecretName: "my-scim-token"
```

When `enabled: true` and no `tokenSecretName` is provided, the operator creates a Kubernetes Secret named `<workbench-name>-scim-token` containing the generated bearer token. Configure your IdP to send SCIM requests to `https://<workbench-url>/scim/v2/` with that token as the `Authorization: Bearer` header.

**Requirements:**
- OIDC or SAML authentication must be configured
- `createUsersAutomatically` should be `false` when SCIM is managing user provisioning

---

## Audited Jobs

Audited Jobs records digital signatures and execution details alongside job output, giving you a tamper-evident audit trail for Workbench sessions. This requires the Advanced product tier.

```yaml
spec:
  workbench:
    auditedJobs:
      # Enable audited jobs (0=disabled, 1=enabled)
      enabled: 1

      # Directory for audit data (must be on a persistent volume)
      storagePath: "/mnt/audit-data"

      # RSA private key path for signing job records
      privateKeyPath: "/etc/rstudio/audit-key.pem"

      # RSA public key path(s) for verification (comma-separated for multiple keys)
      publicKeyPaths: "/etc/rstudio/audit-key-pub.pem"

      # Maximum number of audit log entries to retain
      logLimit: 10000

      # Days before completed audited jobs are deleted
      deletionExpiry: 90
```

See the [Posit Workbench Audited Jobs documentation](https://docs.posit.co/ide/server-pro/admin/auditing_and_monitoring/audited_workbench_jobs.html) for full details on key generation and verification.

---

## Additional Configuration Files

For settings not directly exposed as typed fields, you can append raw configuration content to the Workbench server config files (`rserver.conf`, `launcher.conf`, etc.) and session config files (`rsession.conf`, `repos.conf`, etc.).

### Server Configuration

```yaml
spec:
  workbench:
    # Keys are config file names; values are raw config content appended to each file.
    additionalConfigs:
      rserver.conf: |
        [rsession]
        session-timeout-minutes=240
      launcher.conf: |
        [launcher]
        address=localhost
```

### Session Configuration

```yaml
spec:
  workbench:
    # Appended to session-level config files (rsession.conf, repos.conf, etc.)
    additionalSessionConfigs:
      rsession.conf: |
        r-cran-repos=https://cran.rstudio.com/
      repos.conf: |
        CRAN=https://packagemanager.example.com/cran/latest
```

Both `additionalConfigs` and `additionalSessionConfigs` append content after all operator-managed settings. If a key already exists in the generated file, appending a duplicate section will override earlier values (gcfg last-wins semantics).

---

## Pod Disruption Budgets

The operator automatically creates two PodDisruptionBudgets for every Workbench deployment:

**Server PDB** (`<site>-workbench`): Protects the Workbench server pod(s) during cluster maintenance. When `replicas > 1`, `minAvailable` is set to 1. When `replicas = 1`, `minAvailable` is 0 (allows voluntary disruption of a single-replica deployment).

**Session PDB** (`<site>-workbench-sessions`): Protects all active Workbench session pods with `maxUnavailable: 0`, preventing sessions from being evicted during node drains or cluster upgrades. This ensures users do not lose active R or Python sessions unexpectedly.

These PDBs are managed automatically and do not require any configuration.

---

## Non-Root Execution Mode

Some organizations require that all containers run without root privileges. Enabling non-root mode configures Workbench and its launcher to operate within these constraints. Review the requirements and limitations below before enabling this in production.

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

The `experimentalFeatures` section groups advanced and in-progress options. These settings are subject to change between operator versions. Use them when you need capabilities not yet promoted to top-level fields, but be aware they may be renamed or restructured in future releases.

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

Enable the Workbench REST API for programmatic access to sessions, users, and job management. The admin and super-admin endpoints expose privileged operations and should only be enabled for trusted automation.

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
   - Verify the issuer URL is accessible from the cluster
   - Confirm the client ID matches IdP configuration
   - Verify redirect URIs are configured in the IdP

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

1. **Verify the PVC exists and is bound:**
   ```bash
   kubectl get pvc -n posit-team | grep workbench
   ```

2. **Check volume permissions in the session:**
   ```bash
   kubectl exec -it <session-pod> -n posit-team -- ls -la /home
   ```

3. **Verify the storage class supports RWX:**
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
