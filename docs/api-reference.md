---
title: API Reference
description: Complete CRD field reference for Team Operator resources (auto-generated from CRD schemas)
---

# Team Operator API Reference

This document is auto-generated from the CRD schemas. Last updated: 2026-04-13.

**API Group:** `core.posit.team`

## Table of Contents

- [Site](#site)
- [Connect](#connect)
- [Workbench](#workbench)
- [PackageManager](#packagemanager)
- [Chronicle](#chronicle)
- [PostgresDatabase](#postgresdatabase)
- [Flightdeck](#flightdeck)

---
## Site

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `Site`
**Plural:** `sites`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.awsAccountId` | `string` | No | AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations |
| `.spec.chronicle` | `object` | No | Chronicle contains Posit Chronicle configuration |
| `.spec.chronicle.addEnv` | `map[string]string` | No |  |
| `.spec.chronicle.agentImage` | `string` | No |  |
| `.spec.chronicle.enabled` | `bool` | No | Enabled controls whether Chronicle is running. Defaults to true. Setting to false suspends Chronicle: stops the StatefulSet and removes the service. Re-enabling restores full service. |
| `.spec.chronicle.image` | `string` | No |  |
| `.spec.chronicle.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.chronicle.nodeSelector` | `map[string]string` | No |  |
| `.spec.chronicle.s3Bucket` | `string` | No |  |
| `.spec.chronicle.teardown` | `bool` | No | Teardown permanently destroys all Chronicle resources. Only takes effect when Enabled is false. Re-enabling after teardown starts fresh. |
| `.spec.clusterDate` | `string` | No | ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations |
| `.spec.connect` | `object` | No | Connect contains Posit Connect configuration |
| `.spec.connect.addEnv` | `map[string]string` | No |  |
| `.spec.connect.additionalConfig` | `string` | No | AdditionalConfig allows appending arbitrary gcfg config content to the generated config. |
| `.spec.connect.additionalRuntimeImages` | `[]object` | No | AdditionalRuntimeImages specifies additional runtime images to append to the defaults for Connect off-host execution |
| `.spec.connect.auth` | `object` | No |  |
| `.spec.connect.baseDomain` | `string` | No | BaseDomain overrides site.Spec.Domain for this product's URL construction. When set, the product URL will be: domainPrefix.baseDomain |
| `.spec.connect.databaseSettings` | `object` | No |  |
| `.spec.connect.databricks` | `object` | No |  |
| `.spec.connect.domainPrefix` | `string` | No |  |
| `.spec.connect.enabled` | `bool` | No | Enabled controls whether Connect is running. Defaults to true. Setting to false suspends Connect: stops pods and removes ingress/service, but preserves PVC, database, and secrets so data is retained. Re-enabling restores full service without data loss. |
| `.spec.connect.experimentalFeatures` | `object` | No |  |
| `.spec.connect.gpuSettings` | `object` | No | GPUSettings allows configuring GPU resource requests and limits |
| `.spec.connect.image` | `string` | No |  |
| `.spec.connect.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.connect.license` | `object` | No |  |
| `.spec.connect.loggedInWarning` | `string` | No |  |
| `.spec.connect.nodeSelector` | `map[string]string` | No |  |
| `.spec.connect.publicWarning` | `string` | No |  |
| `.spec.connect.registerOnFirstLogin` | `bool` | No | RegisterOnFirstLogin controls whether new users are automatically registered when they first log in via OAuth2/OIDC. Only applies when auth type is "oidc". |
| `.spec.connect.replicas` | `int` | No |  |
| `.spec.connect.scheduleConcurrency` | `int` | No |  |
| `.spec.connect.sessionImage` | `string` | No |  |
| `.spec.connect.teardown` | `bool` | No | Teardown permanently destroys all Connect resources including the database, secrets, and persistent volume claim. Only takes effect when Enabled is false. Re-enabling after teardown starts fresh with a new empty database. |
| `.spec.connect.volume` | `object` | No | VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted) |
| `.spec.debug` | `bool` | No |  |
| `.spec.disablePrePullImages` | `bool` | No |  |
| `.spec.domain` | `string` | **Yes** | Domain is the core domain name associated with the Posit Team Site |
| `.spec.dropDatabaseOnTearDown` | `bool` | No |  |
| `.spec.efsEnabled` | `bool` | No | EFSEnabled indicates whether EFS is enabled for this site When true, network policies will allow workbench sessions to access EFS mount targets |
| `.spec.enableFqdnHealthChecks` | `bool` | No | EnableFQDNHealthChecks controls whether Grafana Alloy generates FQDN-based health check targets for this site's products. When false, only internal cluster health checks are generated. Defaults to true. |
| `.spec.extraSiteServiceAccounts` | `[]object` | No | ExtraSiteServiceAccounts will be prefixed by "<siteName>-" and created as service accounts in Kubernetes |
| `.spec.flightdeck` | `object` | No | Flightdeck contains Flightdeck configuration |
| `.spec.flightdeck.enabled` | `bool` | No | Enabled controls whether Flightdeck is deployed. Defaults to true if not specified. Set to false to explicitly disable Flightdeck deployment. |
| `.spec.flightdeck.featureEnabler` | `object` | No | FeatureEnabler controls which features are enabled in Flightdeck |
| `.spec.flightdeck.image` | `string` | No | Image is the container image for Flightdeck. Can be a tag (e.g., "v1.2.3") which will be combined with the default registry, or a full image path (e.g., "my-registry.io/flightdeck:v1.0.0"). Defaults to "docker.io/posit/ptd-flightdeck:latest" if not specified. |
| `.spec.flightdeck.imagePullPolicy` | `string` | No | ImagePullPolicy controls when the kubelet pulls the image |
| `.spec.flightdeck.logFormat` | `string` | No | LogFormat sets the log output format (text, json) |
| `.spec.flightdeck.logLevel` | `string` | No | LogLevel sets the logging verbosity (debug, info, warn, error) |
| `.spec.flightdeck.replicas` | `int` | No | Replicas is the number of Flightdeck pods to run |
| `.spec.imagePullSecrets` | `[]string` | No | ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets must already exist in the namespace in question. |
| `.spec.ingressAnnotations` | `map[string]string` | No | IngressAnnotations is a set of annotations to be applied to all ingress routes |
| `.spec.ingressClass` | `string` | No | IngressClass is the ingress class to be used when creating ingress routes |
| `.spec.keycloak` | `object` | No | Keycloak contains the Keycloak configuration details |
| `.spec.keycloak.enabled` | `bool` | No |  |
| `.spec.keycloak.image` | `string` | No |  |
| `.spec.keycloak.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.keycloak.nodeSelector` | `map[string]string` | No |  |
| `.spec.logFormat` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret` | `object` | No | MainDatabaseCredentialSecret configures the secret used for storing the main database credentials |
| `.spec.mainDatabaseCredentialSecret.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret.vaultName` | `string` | No |  |
| `.spec.networkTrust` | `int` | No |  |
| `.spec.packageManager` | `object` | No | PackageManager contains Posit Package Manager configuration |
| `.spec.packageManager.addEnv` | `map[string]string` | No |  |
| `.spec.packageManager.additionalConfig` | `string` | No | AdditionalConfig allows appending arbitrary gcfg config content to the generated config. |
| `.spec.packageManager.auth` | `object` | No | Auth configures OIDC authentication for Package Manager's web UI |
| `.spec.packageManager.azureFiles` | `object` | No | AzureFiles configures Azure Files integration for persistent storage |
| `.spec.packageManager.baseDomain` | `string` | No | BaseDomain overrides site.Spec.Domain for this product's URL construction. When set, the product URL will be: domainPrefix.baseDomain |
| `.spec.packageManager.domainPrefix` | `string` | No |  |
| `.spec.packageManager.enabled` | `bool` | No | Enabled controls whether Package Manager is running. Defaults to true. Setting to false suspends Package Manager: stops pods and removes ingress/service, but preserves PVC, database, and secrets so data is retained. Re-enabling restores full service without data loss. |
| `.spec.packageManager.gitSSHKeys` | `[]object` | No | GitSSHKeys defines SSH key configurations for Git authentication in Package Manager These SSH keys will be made available to Package Manager for Git Builders |
| `.spec.packageManager.image` | `string` | No |  |
| `.spec.packageManager.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.packageManager.license` | `object` | No |  |
| `.spec.packageManager.nodeSelector` | `map[string]string` | No |  |
| `.spec.packageManager.oidcClientSecretKey` | `string` | No | OIDCClientSecretKey is the key in the vault for the OIDC client secret |
| `.spec.packageManager.replicas` | `int` | No |  |
| `.spec.packageManager.s3Bucket` | `string` | No |  |
| `.spec.packageManager.teardown` | `bool` | No | Teardown permanently destroys all Package Manager resources including the database, secrets, and persistent volume claim. Only takes effect when Enabled is false. Re-enabling after teardown starts fresh with a new empty database. |
| `.spec.packageManager.volume` | `object` | No | VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted) |
| `.spec.packageManagerUrl` | `string` | No | PackageManagerUrl specifies the Package Manager URL for Workbench to use If empty, Workbench will use the local Package Manager URL by default |
| `.spec.secret` | `object` | No | Secret configures the secret management for this Site |
| `.spec.secret.type` | `string` | No |  |
| `.spec.secret.vaultName` | `string` | No |  |
| `.spec.secretType` | `string` | No | SecretType is the type of secret that we should use to store values (i.e. database passwords) *NOTE*: this field is deprecated and will be removed in the future |
| `.spec.sharedDirectory` | `string` | No | SharedDirectory is the name of a directory mounted into Workbench and Connect at /mnt/<sharedDirectory>. It should NOT contain any slashes. |
| `.spec.volumeSource` | `object` | No | VolumeSource is a definition of where volumes should be created from. Usually a site targets a single shared resource (i.e. FSx instance) to provision all of its shared data |
| `.spec.volumeSource.dnsName` | `string` | No |  |
| `.spec.volumeSource.type` | `string` | No |  |
| `.spec.volumeSource.volumeId` | `string` | No |  |
| `.spec.volumeSubdirJobOff` | `bool` | No | VolumeSubdirJobOff turns off the VolumeSubdir provisioning kubernetes job |
| `.spec.vpcCIDR` | `string` | No | VPCCIDR is the CIDR block for the VPC, used for EFS network policies |
| `.spec.workbench` | `object` | No | Workbench contains Posit Workbench configuration |
| `.spec.workbench.addEnv` | `map[string]string` | No |  |
| `.spec.workbench.additionalConfigs` | `map[string]string` | No | AdditionalConfigs allows appending arbitrary content to Workbench server config files. Keys are config file names (e.g., "rserver.conf", "launcher.conf"). |
| `.spec.workbench.additionalSessionConfigs` | `map[string]string` | No | AdditionalSessionConfigs allows appending arbitrary content to Workbench session config files. Keys are config file names (e.g., "rsession.conf", "repos.conf"). |
| `.spec.workbench.additionalVolumes` | `[]object` | No | AdditionalVolumes represents additional VolumeSpec's that can be defined for Workbench |
| `.spec.workbench.adminGroups` | `[]string` | No | AdminGroups specifies a list of groups that will have admin access to the Workbench administrative dashboard These groups will be joined into a comma-delimited string for the admin-group configuration If not specified, defaults to ["workbench-admin"] |
| `.spec.workbench.adminSuperuserGroups` | `[]string` | No | AdminSuperuserGroups specifies a list of groups that will have superuser access to the Workbench administrative dashboard These groups will be joined into a comma-delimited string for the admin-superuser-group configuration If not specified, no superuser groups will be configured |
| `.spec.workbench.apiSettings` | `object` | No |  |
| `.spec.workbench.auditedJobs` | `object` | No | AuditedJobs configures Workbench Audited Jobs for tracking execution details alongside job output, including digital signatures and environment data. Requires the Advanced product tier. See: https://docs.posit.co/ide/server-pro/admin/auditing_and_monitoring/audited_workbench_jobs.html |
| `.spec.workbench.auth` | `object` | No |  |
| `.spec.workbench.authLoginPageHtml` | `string` | No | Workbench Auth/Login Landing Page Customization HTML |
| `.spec.workbench.baseDomain` | `string` | No | BaseDomain overrides site.Spec.Domain for this product's URL construction. When set, the product URL will be: domainPrefix.baseDomain |
| `.spec.workbench.createUsersAutomatically` | `bool` | No |  |
| `.spec.workbench.databricks` | `map[string]object` | No |  |
| `.spec.workbench.defaultSessionImage` | `string` | No |  |
| `.spec.workbench.domainPrefix` | `string` | No |  |
| `.spec.workbench.enabled` | `bool` | No | Enabled controls whether Workbench is running. Defaults to true. Setting to false suspends Workbench: stops pods and removes ingress/service, but preserves PVC, database, and secrets so data is retained. Re-enabling restores full service without data loss. |
| `.spec.workbench.experimentalFeatures` | `object` | No | ExperimentalFeatures allows enabling miscellaneous experimental features for workbench |
| `.spec.workbench.extraSessionImages` | `[]string` | No |  |
| `.spec.workbench.image` | `string` | No |  |
| `.spec.workbench.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.workbench.jupyterConfig` | `object` | No | JupyterConfig contains Jupyter configuration for Workbench |
| `.spec.workbench.license` | `object` | No |  |
| `.spec.workbench.nodeSelector` | `map[string]string` | No | NodeSelector that is applied universally to server and sessions |
| `.spec.workbench.positronConfig` | `object` | No |  |
| `.spec.workbench.replicas` | `int` | No |  |
| `.spec.workbench.scim` | `object` | No | SCIM configures SCIM user provisioning for Workbench. Requires SSO (OIDC or SAML) to be configured. |
| `.spec.workbench.sessionConfig` | `object` | No | SessionConfig allows configuring Workbench session pods, including dynamic labels, annotations, tolerations, and other pod-level settings. |
| `.spec.workbench.sessionInitContainerImageName` | `string` | No | SessionInitContainerImageName specifies the init container image name for Workbench sessions |
| `.spec.workbench.sessionInitContainerImageTag` | `string` | No | SessionInitContainerImageTag specifies the init container image tag for Workbench sessions |
| `.spec.workbench.sessionTolerations` | `[]object` | No | SessionTolerations are tolerations applied only to session pods (not the main workbench server) |
| `.spec.workbench.snowflake` | `object` | No |  |
| `.spec.workbench.teardown` | `bool` | No | Teardown permanently destroys all Workbench resources including the database, secrets, and persistent volume claim. Only takes effect when Enabled is false. Re-enabling after teardown starts fresh with a new empty database. |
| `.spec.workbench.tolerations` | `[]object` | No | Tolerations that are applied universally to server and sessions |
| `.spec.workbench.volume` | `object` | No | VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted) |
| `.spec.workbench.vsCodeConfig` | `object` | No |  |
| `.spec.workbench.vsCodeExtensions` | `[]string` | No |  |
| `.spec.workbench.vsCodeUserSettings` | `map[string]string` | No |  |
| `.spec.workloadCompoundName` | `string` | No | WorkloadCompoundName is the name for the workload |
| `.spec.workloadSecret` | `object` | No | WorkloadSecret configures the managed persistent secret for the entire workload account |
| `.spec.workloadSecret.type` | `string` | No |  |
| `.spec.workloadSecret.vaultName` | `string` | No |  |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.chronicleReady` | `bool` | No | ChronicleReady indicates whether the Chronicle child resource is ready. |
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.connectReady` | `bool` | No | ConnectReady indicates whether the Connect child resource is ready. |
| `.status.flightdeckReady` | `bool` | No | FlightdeckReady indicates whether the Flightdeck child resource is ready. |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.packageManagerReady` | `bool` | No | PackageManagerReady indicates whether the PackageManager child resource is ready. |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |
| `.status.workbenchReady` | `bool` | No | WorkbenchReady indicates whether the Workbench child resource is ready. |

---
## Connect

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `Connect`
**Plural:** `connects`
**Short Names:** `con`, `cons`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.addEnv` | `map[string]string` | No | AddEnv adds arbitrary environment variables to the container env |
| `.spec.additionalRuntimeImages` | `[]object` | No | AdditionalRuntimeImages specifies additional runtime images to append to the defaults for Connect off-host execution. These are added after the built-in default images. |
| `.spec.additionalVolumes` | `[]object` | No | AdditionalVolumes represents additional VolumeSpec's that can be defined |
| `.spec.auth` | `object` | No |  |
| `.spec.auth.administratorRoleMapping` | `[]string` | No |  |
| `.spec.auth.clientId` | `string` | No |  |
| `.spec.auth.disableGroupsClaim` | `bool` | No |  |
| `.spec.auth.emailClaim` | `string` | No |  |
| `.spec.auth.groups` | `bool` | No |  |
| `.spec.auth.groupsClaim` | `string` | No |  |
| `.spec.auth.issuer` | `string` | No |  |
| `.spec.auth.publisherRoleMapping` | `[]string` | No |  |
| `.spec.auth.samlEmailAttribute` | `string` | No |  |
| `.spec.auth.samlFirstNameAttribute` | `string` | No |  |
| `.spec.auth.samlIdPAttributeProfile` | `string` | No | SAML-specific attribute mappings (mutually exclusive with SamlIdPAttributeProfile) |
| `.spec.auth.samlLastNameAttribute` | `string` | No |  |
| `.spec.auth.samlMetadataUrl` | `string` | No |  |
| `.spec.auth.samlUsernameAttribute` | `string` | No |  |
| `.spec.auth.scopes` | `[]string` | No |  |
| `.spec.auth.type` | `string` | No |  |
| `.spec.auth.uniqueIdClaim` | `string` | No |  |
| `.spec.auth.usernameClaim` | `string` | No |  |
| `.spec.auth.viewerRoleMapping` | `[]string` | No |  |
| `.spec.awsAccountId` | `string` | No | AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations |
| `.spec.chronicleImage` | `string` | No | ChronicleAgentImage is the image used for the Chronicle Agent |
| `.spec.chronicleSidecarProductApiKeyEnabled` | `bool` | No | ChronicleSidecarProductApiKeyEnabled assumes the api key for this product has been added to a secret and injects the secret as an environment variable to the sidecar. **EXPERIMENTAL** |
| `.spec.clusterDate` | `string` | No | ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations |
| `.spec.config` | `object` | No |  |
| `.spec.config.Applications` | `object` | No |  |
| `.spec.config.Authentication` | `object` | No |  |
| `.spec.config.Authorization` | `object` | No |  |
| `.spec.config.Database` | `object` | No |  |
| `.spec.config.Http` | `object` | No |  |
| `.spec.config.Launcher` | `object` | No |  |
| `.spec.config.Logging` | `object` | No |  |
| `.spec.config.Metrics` | `object` | No |  |
| `.spec.config.OAuth2` | `object` | No |  |
| `.spec.config.Postgres` | `object` | No |  |
| `.spec.config.Python` | `object` | No |  |
| `.spec.config.Quarto` | `object` | No |  |
| `.spec.config.R` | `object` | No |  |
| `.spec.config.RPackageRepositories` | `map[string]object` | No | exclude this from default JSON marshalling... that way we can handle directly and unwrap the keys at the top level of the JSON output see the GenerateGcfg method for our custom handling |
| `.spec.config.SAML` | `object` | No |  |
| `.spec.config.Scheduler` | `object` | No |  |
| `.spec.config.Server` | `object` | No |  |
| `.spec.config.TableauIntegration` | `object` | No |  |
| `.spec.config.additionalConfig` | `string` | No | AdditionalConfig allows appending arbitrary gcfg config content not covered by typed fields. The value is appended verbatim after the generated config. gcfg parsing naturally handles conflicts: list values are combined, scalar values use the last occurrence. |
| `.spec.databaseConfig` | `object` | No |  |
| `.spec.databaseConfig.dropOnTeardown` | `bool` | No |  |
| `.spec.databaseConfig.host` | `string` | No |  |
| `.spec.databaseConfig.instrumentationSchema` | `string` | No |  |
| `.spec.databaseConfig.schema` | `string` | No |  |
| `.spec.databaseConfig.sslMode` | `string` | No |  |
| `.spec.debug` | `bool` | No | Debug sets whether to enable debug settings. This setting overrides specific "Config.Logging" sections globally |
| `.spec.dsnSecret` | `string` | No | DsnSecret is the name of the secret that contains the DSN to include with all Connect sessions |
| `.spec.image` | `string` | No |  |
| `.spec.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.imagePullSecrets` | `[]string` | No | ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets must already exist in the namespace in question. |
| `.spec.ingressAnnotations` | `map[string]string` | No | IngressAnnotations is a set of annotations to be applied to all ingress routes |
| `.spec.ingressClass` | `string` | No | IngressClass is the ingress class to be used when creating ingress routes |
| `.spec.license` | `object` | No |  |
| `.spec.license.existingSecretKey` | `string` | No |  |
| `.spec.license.existingSecretName` | `string` | No |  |
| `.spec.license.key` | `string` | No |  |
| `.spec.license.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret` | `object` | No | MainDatabaseCredentialSecret configures the secret used for storing the main database credentials |
| `.spec.mainDatabaseCredentialSecret.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret.vaultName` | `string` | No |  |
| `.spec.nodeSelector` | `map[string]string` | No |  |
| `.spec.offHostExecution` | `bool` | No |  |
| `.spec.registerOnFirstLogin` | `bool` | No | RegisterOnFirstLogin controls whether new users are automatically registered when they first log in via OAuth2/OIDC. Only applies when auth type is "oidc". |
| `.spec.replicas` | `int` | No |  |
| `.spec.secret` | `object` | No | Secret configures the secret management for this Connect |
| `.spec.secret.type` | `string` | No |  |
| `.spec.secret.vaultName` | `string` | No |  |
| `.spec.secretType` | `string` | No |  |
| `.spec.sessionConfig` | `object` | No | SessionConfig houses all session configuration |
| `.spec.sessionConfig.job` | `object` | No | JobConfig is the configuration for session jobs |
| `.spec.sessionConfig.pod` | `object` | No | PodConfig is the configuration for session pods |
| `.spec.sessionConfig.service` | `object` | No | ServiceConfig is the configuration for session service definition |
| `.spec.sessionImage` | `string` | No |  |
| `.spec.sleep` | `bool` | No | Sleep puts the service to sleep... so you can debug a crash looping container / etc. It is an ugly escape hatch, but can also be useful on occasion |
| `.spec.suspended` | `bool` | No | Suspended indicates Connect should not run serving resources (Deployment, Service, Ingress) but should preserve data resources (PVC, database, secrets). Set by the Site controller. |
| `.spec.url` | `string` | No |  |
| `.spec.volume` | `object` | No | VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted) |
| `.spec.volume.accessModes` | `[]string` | No | AccessModes is the access mode for the created PVC. Only used if Create is true |
| `.spec.volume.create` | `bool` | No | Create determines whether the PVC should be created or not |
| `.spec.volume.mountPath` | `string` | No | MountPath is not always used. It is only used for volumes that are configurable... (i.e. additionalVolumes) |
| `.spec.volume.pvcName` | `string` | No | PvcName is used only if Create is false. The idea is that the PVC has already been created. Only used if Create is false |
| `.spec.volume.readOnly` | `bool` | No | ReadOnly defaults to false and determines whether the volume is mounted ReadOnly |
| `.spec.volume.size` | `string` | No | Size is the size of the PVC that is being created. Only used if Create is true |
| `.spec.volume.storageClassName` | `string` | No | StorageClassName identifies the StorageClassName created by the PV. Only used if Create is true |
| `.spec.volume.volumeName` | `string` | No | VolumeName is the name of the PV that will be referenced by the created PVC. Only used if Create is true |
| `.spec.workloadCompoundName` | `string` | No | WorkloadCompoundName is the name for the workload |
| `.spec.workloadSecret` | `object` | No | WorkloadSecret configures the managed persistent secret for the entire workload account |
| `.spec.workloadSecret.type` | `string` | No |  |
| `.spec.workloadSecret.vaultName` | `string` | No |  |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.keySecretRef` | `object` | No | SecretReference represents a Secret Reference. It has enough information to retrieve secret in any namespace |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.ready` | `bool` | No |  |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |

---
## Workbench

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `Workbench`
**Plural:** `workbenches`
**Short Names:** `wb`, `wbs`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.addEnv` | `map[string]string` | No | AddEnv adds arbitrary environment variables to the container env |
| `.spec.additionalVolumes` | `[]object` | No | AdditionalVolumes represents additional VolumeSpec's that can be defined |
| `.spec.auth` | `object` | No |  |
| `.spec.auth.administratorRoleMapping` | `[]string` | No |  |
| `.spec.auth.clientId` | `string` | No |  |
| `.spec.auth.disableGroupsClaim` | `bool` | No |  |
| `.spec.auth.emailClaim` | `string` | No |  |
| `.spec.auth.groups` | `bool` | No |  |
| `.spec.auth.groupsClaim` | `string` | No |  |
| `.spec.auth.issuer` | `string` | No |  |
| `.spec.auth.publisherRoleMapping` | `[]string` | No |  |
| `.spec.auth.samlEmailAttribute` | `string` | No |  |
| `.spec.auth.samlFirstNameAttribute` | `string` | No |  |
| `.spec.auth.samlIdPAttributeProfile` | `string` | No | SAML-specific attribute mappings (mutually exclusive with SamlIdPAttributeProfile) |
| `.spec.auth.samlLastNameAttribute` | `string` | No |  |
| `.spec.auth.samlMetadataUrl` | `string` | No |  |
| `.spec.auth.samlUsernameAttribute` | `string` | No |  |
| `.spec.auth.scopes` | `[]string` | No |  |
| `.spec.auth.type` | `string` | No |  |
| `.spec.auth.uniqueIdClaim` | `string` | No |  |
| `.spec.auth.usernameClaim` | `string` | No |  |
| `.spec.auth.viewerRoleMapping` | `[]string` | No |  |
| `.spec.authLoginPageHtml` | `string` | No | AuthLoginPageHtml is the custom HTML content to be displayed on the Workbench login page. This content will be mounted at /etc/rstudio/login.html in the Workbench pod. The HTML content must be valid and complete HTML and less than 65,536 bytes (64KB) in size. Empty or whitespace-only content will be ignored. See: https://docs.posit.co/ide/server-pro/admin/authenticating_users/customizing_signin.html |
| `.spec.awsAccountId` | `string` | No | AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations |
| `.spec.chronicleImage` | `string` | No | ChronicleAgentImage is the image used for the Chronicle Agent |
| `.spec.chronicleSidecarProductApiKeyEnabled` | `bool` | No | ChronicleSidecarProductApiKeyEnabled assumes the api key for this product has been added to a secret and injects the secret as an environment variable to the sidecar. **EXPERIMENTAL** |
| `.spec.clusterDate` | `string` | No | ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations |
| `.spec.config` | `object` | No | WorkbenchConfig is a "top-level" configuration object. It has "child-structs" which have different config formats, and the `GenerateConfigmap` method generates a map[string]string which can be used to create a configmap with the contents |
| `.spec.config.supervisord-ini-config` | `object` | No | SupervisordIniConfig allows customization of the startup of the product... it is currently only enabled and utilized when workbench.Spec.NonRoot is enabled |
| `.spec.config.workbench-dcf-config` | `object` | No |  |
| `.spec.config.workbench-ini-config` | `object` | No |  |
| `.spec.config.workbench-profiles-config` | `object` | No |  |
| `.spec.config.workbench-session-ini-config` | `object` | No |  |
| `.spec.config.workbench-session-json-config` | `object` | No |  |
| `.spec.config.workbench-session-newline-config` | `object` | No |  |
| `.spec.databaseConfig` | `object` | No |  |
| `.spec.databaseConfig.dropOnTeardown` | `bool` | No |  |
| `.spec.databaseConfig.host` | `string` | No |  |
| `.spec.databaseConfig.instrumentationSchema` | `string` | No |  |
| `.spec.databaseConfig.schema` | `string` | No |  |
| `.spec.databaseConfig.sslMode` | `string` | No |  |
| `.spec.dsnSecret` | `string` | No | DsnSecret is the name of the secret that contains the DSN to include with all Workbench sessions |
| `.spec.image` | `string` | No |  |
| `.spec.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.imagePullSecrets` | `[]string` | No | ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets must already exist in the namespace in question. |
| `.spec.ingressAnnotations` | `map[string]string` | No | IngressAnnotations is a set of annotations to be applied to all ingress routes |
| `.spec.ingressClass` | `string` | No | IngressClass is the ingress class to be used when creating ingress routes |
| `.spec.license` | `object` | No |  |
| `.spec.license.existingSecretKey` | `string` | No |  |
| `.spec.license.existingSecretName` | `string` | No |  |
| `.spec.license.key` | `string` | No |  |
| `.spec.license.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret` | `object` | No | MainDatabaseCredentialSecret configures the secret used for storing the main database credentials |
| `.spec.mainDatabaseCredentialSecret.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret.vaultName` | `string` | No |  |
| `.spec.nodeSelector` | `map[string]string` | No |  |
| `.spec.nonRoot` | `bool` | No | NonRoot is a flag that enables rootless execution for workbench (or as much as is currently possible...) |
| `.spec.offHostExecution` | `bool` | No |  |
| `.spec.parentUrl` | `string` | No |  |
| `.spec.replicas` | `int` | No |  |
| `.spec.scim` | `object` | No | SCIM configures SCIM user provisioning. |
| `.spec.scim.enabled` | `bool` | **Yes** | Enabled controls whether SCIM provisioning is active. |
| `.spec.scim.tokenSecretName` | `string` | No | TokenSecretName is the name of a pre-existing Kubernetes Secret in the same namespace that contains the SCIM bearer token. The secret must have a key named "token". If not specified and Enabled is true, the operator generates a random token and stores it in a Secret named "<workbench-name>-scim-token". |
| `.spec.secret` | `object` | No | Secret configures the secret management for this Workbench |
| `.spec.secret.type` | `string` | No |  |
| `.spec.secret.vaultName` | `string` | No |  |
| `.spec.secretConfig` | `object` | No | WorkbenchSecretConfig is a "top-level" configuration object. It has "child-structs" which have different config formats, and the `GenerateSecretData` method generates a map[string]string which can be used to create a secret with the contents |
| `.spec.secretConfig.workbench-secret-ini-config` | `object` | No |  |
| `.spec.secretType` | `string` | No |  |
| `.spec.sessionConfig` | `object` | No | SessionConfig houses all session configuration |
| `.spec.sessionConfig.job` | `object` | No | JobConfig is the configuration for session jobs |
| `.spec.sessionConfig.pod` | `object` | No | PodConfig is the configuration for session pods |
| `.spec.sessionConfig.service` | `object` | No | ServiceConfig is the configuration for session service definition |
| `.spec.sleep` | `bool` | No | Sleep puts the service to sleep... so you can debug a crash looping container / etc. It is an ugly escape hatch, but can also be useful on occasion |
| `.spec.snowflake` | `object` | No |  |
| `.spec.snowflake.accountId` | `string` | No |  |
| `.spec.snowflake.clientId` | `string` | No |  |
| `.spec.suspended` | `bool` | No | Suspended indicates Workbench should not run serving resources (Deployment, Service, Ingress) but should preserve data resources (PVC, database, secrets). Set by the Site controller. |
| `.spec.tolerations` | `[]object` | No |  |
| `.spec.url` | `string` | No |  |
| `.spec.volume` | `object` | No | VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted) |
| `.spec.volume.accessModes` | `[]string` | No | AccessModes is the access mode for the created PVC. Only used if Create is true |
| `.spec.volume.create` | `bool` | No | Create determines whether the PVC should be created or not |
| `.spec.volume.mountPath` | `string` | No | MountPath is not always used. It is only used for volumes that are configurable... (i.e. additionalVolumes) |
| `.spec.volume.pvcName` | `string` | No | PvcName is used only if Create is false. The idea is that the PVC has already been created. Only used if Create is false |
| `.spec.volume.readOnly` | `bool` | No | ReadOnly defaults to false and determines whether the volume is mounted ReadOnly |
| `.spec.volume.size` | `string` | No | Size is the size of the PVC that is being created. Only used if Create is true |
| `.spec.volume.storageClassName` | `string` | No | StorageClassName identifies the StorageClassName created by the PV. Only used if Create is true |
| `.spec.volume.volumeName` | `string` | No | VolumeName is the name of the PV that will be referenced by the created PVC. Only used if Create is true |
| `.spec.workloadCompoundName` | `string` | No | WorkloadCompoundName is the name for the workload |
| `.spec.workloadSecret` | `object` | No | WorkloadSecret configures the managed persistent secret for the entire workload account |
| `.spec.workloadSecret.type` | `string` | No |  |
| `.spec.workloadSecret.vaultName` | `string` | No |  |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.keySecretRef` | `object` | No | SecretReference represents a Secret Reference. It has enough information to retrieve secret in any namespace |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.ready` | `bool` | No |  |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |

---
## PackageManager

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `PackageManager`
**Plural:** `packagemanagers`
**Short Names:** `pm`, `pms`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.addEnv` | `map[string]string` | No | AddEnv adds arbitrary environment variables to the container env |
| `.spec.awsAccountId` | `string` | No | AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations |
| `.spec.azureFiles` | `object` | No | AzureFiles configures Azure Files integration for persistent storage |
| `.spec.azureFiles.shareSizeGiB` | `int` | No | ShareSizeGiB is the size of the Azure File Share to create |
| `.spec.azureFiles.storageClassName` | `string` | No | StorageClassName is the name of the Kubernetes StorageClass that uses the Azure Files CSI driver |
| `.spec.chronicleImage` | `string` | No | ChronicleAgentImage is the image used for the Chronicle Agent |
| `.spec.clusterDate` | `string` | No | ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations |
| `.spec.config` | `object` | No |  |
| `.spec.config.Authentication` | `object` | No |  |
| `.spec.config.CRAN` | `object` | No | PackageManagerCRANConfig is deprecated TODO: deprecated! We will remove this soon! |
| `.spec.config.Database` | `object` | No |  |
| `.spec.config.Debug` | `object` | No |  |
| `.spec.config.Git` | `object` | No |  |
| `.spec.config.Http` | `object` | No |  |
| `.spec.config.Metrics` | `object` | No |  |
| `.spec.config.OpenIDConnect` | `object` | No |  |
| `.spec.config.Postgres` | `object` | No |  |
| `.spec.config.Repos` | `object` | No |  |
| `.spec.config.S3Storage` | `object` | No |  |
| `.spec.config.Server` | `object` | No |  |
| `.spec.config.Storage` | `object` | No |  |
| `.spec.config.additionalConfig` | `string` | No | AdditionalConfig allows appending arbitrary gcfg config content not covered by typed fields. The value is appended verbatim after the generated config. gcfg parsing naturally handles conflicts: list values are combined, scalar values use the last occurrence. |
| `.spec.databaseConfig` | `object` | No |  |
| `.spec.databaseConfig.dropOnTeardown` | `bool` | No |  |
| `.spec.databaseConfig.host` | `string` | No |  |
| `.spec.databaseConfig.instrumentationSchema` | `string` | No |  |
| `.spec.databaseConfig.schema` | `string` | No |  |
| `.spec.databaseConfig.sslMode` | `string` | No |  |
| `.spec.gitSSHKeys` | `[]object` | No | GitSSHKeys defines SSH key configurations for Git authentication This is used for mounting SSH keys but not included in the .gcfg file |
| `.spec.image` | `string` | No |  |
| `.spec.imagePullPolicy` | `string` | No | PullPolicy describes a policy for if/when to pull a container image |
| `.spec.imagePullSecrets` | `[]string` | No | ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets must already exist in the namespace in question. |
| `.spec.ingressAnnotations` | `map[string]string` | No | IngressAnnotations is a set of annotations to be applied to all ingress routes |
| `.spec.ingressClass` | `string` | No | IngressClass is the ingress class to be used when creating ingress routes |
| `.spec.license` | `object` | No |  |
| `.spec.license.existingSecretKey` | `string` | No |  |
| `.spec.license.existingSecretName` | `string` | No |  |
| `.spec.license.key` | `string` | No |  |
| `.spec.license.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret` | `object` | No | MainDatabaseCredentialSecret configures the secret used for storing the main database credentials |
| `.spec.mainDatabaseCredentialSecret.type` | `string` | No |  |
| `.spec.mainDatabaseCredentialSecret.vaultName` | `string` | No |  |
| `.spec.nodeSelector` | `map[string]string` | No |  |
| `.spec.oidcClientSecretKey` | `string` | No | OIDCClientSecretKey is the key name in the vault for the OIDC client secret. When set, the client secret will be mounted at /etc/rstudio-pm/oidc-client-secret |
| `.spec.replicas` | `int` | No |  |
| `.spec.secret` | `object` | No | Secret configures the secret management for this PackageManager |
| `.spec.secret.type` | `string` | No |  |
| `.spec.secret.vaultName` | `string` | No |  |
| `.spec.secretType` | `string` | No |  |
| `.spec.sleep` | `bool` | No | Sleep puts the service to sleep... so you can debug a crash looping container / etc. It is an ugly escape hatch, but can also be useful on occasion |
| `.spec.suspended` | `bool` | No | Suspended indicates Package Manager should not run serving resources (Deployment, Service, Ingress) but should preserve data resources (PVC, database, secrets). Set by the Site controller. |
| `.spec.url` | `string` | No |  |
| `.spec.volume` | `object` | No | VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted) |
| `.spec.volume.accessModes` | `[]string` | No | AccessModes is the access mode for the created PVC. Only used if Create is true |
| `.spec.volume.create` | `bool` | No | Create determines whether the PVC should be created or not |
| `.spec.volume.mountPath` | `string` | No | MountPath is not always used. It is only used for volumes that are configurable... (i.e. additionalVolumes) |
| `.spec.volume.pvcName` | `string` | No | PvcName is used only if Create is false. The idea is that the PVC has already been created. Only used if Create is false |
| `.spec.volume.readOnly` | `bool` | No | ReadOnly defaults to false and determines whether the volume is mounted ReadOnly |
| `.spec.volume.size` | `string` | No | Size is the size of the PVC that is being created. Only used if Create is true |
| `.spec.volume.storageClassName` | `string` | No | StorageClassName identifies the StorageClassName created by the PV. Only used if Create is true |
| `.spec.volume.volumeName` | `string` | No | VolumeName is the name of the PV that will be referenced by the created PVC. Only used if Create is true |
| `.spec.workloadCompoundName` | `string` | No | WorkloadCompoundName is the name for the workload |
| `.spec.workloadSecret` | `object` | No | WorkloadSecret configures the managed persistent secret for the entire workload account |
| `.spec.workloadSecret.type` | `string` | No |  |
| `.spec.workloadSecret.vaultName` | `string` | No |  |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.keySecretRef` | `object` | No | SecretReference represents a Secret Reference. It has enough information to retrieve secret in any namespace |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.ready` | `bool` | No |  |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |

---
## Chronicle

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `Chronicle`
**Plural:** `chronicles`
**Short Names:** `pcr`, `chr`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.addEnv` | `map[string]string` | No | AddEnv adds arbitrary environment variables to the container env |
| `.spec.awsAccountId` | `string` | No | AwsAccountId is the account Id for this AWS Account. It is used to create EKS-to-IAM annotations |
| `.spec.clusterDate` | `string` | No | ClusterDate is the date id (YYYYmmdd) for the cluster. It is used to create EKS-to-IAM annotations |
| `.spec.config` | `object` | No |  |
| `.spec.config.Http` | `object` | No |  |
| `.spec.config.LocalStorage` | `object` | No |  |
| `.spec.config.Logging` | `object` | No |  |
| `.spec.config.Metrics` | `object` | No |  |
| `.spec.config.Profiling` | `object` | No |  |
| `.spec.config.S3Storage` | `object` | No |  |
| `.spec.image` | `string` | No |  |
| `.spec.imagePullSecrets` | `[]string` | No | ImagePullSecrets is a set of image pull secrets to use for all image pulls. These names / secrets must already exist in the namespace in question. |
| `.spec.nodeSelector` | `map[string]string` | No |  |
| `.spec.suspended` | `bool` | No | Suspended indicates Chronicle should not run serving resources (StatefulSet, Service) but should preserve configuration. Set by the Site controller. |
| `.spec.workloadCompoundName` | `string` | No | WorkloadCompoundName is the name for the workload |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.ready` | `bool` | No |  |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |

---
## PostgresDatabase

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `PostgresDatabase`
**Plural:** `postgresdatabases`
**Short Names:** `pgdb`, `pgdbs`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.extensions` | `[]string` | No |  |
| `.spec.mainDbCredentialSecret` | `object` | No | MainDatabaseCredentialSecret configures the secret used for storing the main database credentials |
| `.spec.mainDbCredentialSecret.type` | `string` | No |  |
| `.spec.mainDbCredentialSecret.vaultName` | `string` | No |  |
| `.spec.schemas` | `[]string` | No |  |
| `.spec.secret` | `object` | No | Secret is configuration to use for retrieving the password |
| `.spec.secret.type` | `string` | No |  |
| `.spec.secret.vaultName` | `string` | No |  |
| `.spec.secretPasswordKey` | `string` | **Yes** | SecretPasswordKey is the password key to use (within the SecretVault) |
| `.spec.secretVault` | `string` | **Yes** | SecretVault is the secretId to use for retrieving the password |
| `.spec.teardown` | `object` | No |  |
| `.spec.teardown.drop` | `bool` | No |  |
| `.spec.url` | `string` | **Yes** |  |
| `.spec.workloadSecret` | `object` | No | WorkloadSecret configures the managed persistent secret for the entire workload account |
| `.spec.workloadSecret.type` | `string` | No |  |
| `.spec.workloadSecret.vaultName` | `string` | No |  |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |

---
## Flightdeck

**API Group/Version:** `core.posit.team/v1beta1`
**Kind:** `Flightdeck`
**Plural:** `flightdecks`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.awsAccountId` | `string` | No | AwsAccountId is the AWS account ID (used for EKS-to-IAM annotations) |
| `.spec.clusterDate` | `string` | No | ClusterDate is the cluster date ID (used for EKS-to-IAM annotations) |
| `.spec.domain` | `string` | No | Domain is the domain name for Flightdeck ingress |
| `.spec.featureEnabler` | `object` | No | FeatureEnabler controls which features are enabled in Flightdeck |
| `.spec.featureEnabler.showAcademy` | `bool` | No | ShowAcademy enables the academy page |
| `.spec.featureEnabler.showConfig` | `bool` | No | ShowConfig enables the configuration page |
| `.spec.image` | `string` | No | Image is the container image for Flightdeck |
| `.spec.imagePullPolicy` | `string` | No | ImagePullPolicy controls when the kubelet pulls the image |
| `.spec.imagePullSecrets` | `[]string` | No | ImagePullSecrets are references to secrets for pulling images |
| `.spec.ingressAnnotations` | `map[string]string` | No | IngressAnnotations are annotations to apply to the ingress |
| `.spec.ingressClass` | `string` | No | IngressClass is the ingress class to use |
| `.spec.logFormat` | `string` | No | LogFormat sets the log output format (text, json) |
| `.spec.logLevel` | `string` | No | LogLevel sets the logging verbosity (debug, info, warn, error) |
| `.spec.port` | `int` | No | Port is the port that the container will listen on |
| `.spec.replicas` | `int` | No | Replicas is the number of Flightdeck pods to run |
| `.spec.siteName` | `string` | No | SiteName is the name of the Site that owns this Flightdeck instance |
| `.spec.workloadCompoundName` | `string` | No | WorkloadCompoundName is the workload name |

### Status Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.status.conditions` | `[]object` | No | Conditions represent the latest available observations of the resource's current state. |
| `.status.observedGeneration` | `int64` | No | ObservedGeneration is the most recent generation observed for this resource. It corresponds to the resource's generation, which is updated on mutation by the API Server. |
| `.status.ready` | `bool` | No | Ready indicates whether the Flightdeck deployment is ready |
| `.status.version` | `string` | No | Version is the version of the product image being deployed. |

---
