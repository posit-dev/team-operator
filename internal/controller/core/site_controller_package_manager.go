package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SiteReconciler) reconcilePackageManager(
	ctx context.Context,
	req controllerruntime.Request,
	site *v1beta1.Site,
	dbHost string,
	sslMode string,
	packageManagerUrl string,
) error {

	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-package-manager",
	)

	packageManagerDebugLogConfig := ""
	if site.Spec.Debug {
		packageManagerDebugLogConfig = "verbose"
	}

	packageManagerAccessLogFormat := v1beta1.PackageManagerAccessLogFormatCommon

	if site.Spec.LogFormat == product.LogFormatJson {
		// TODO: packageManagerAccessLogFormat does not support JSON yet...
	}

	pm := &v1beta1.PackageManager{
		ObjectMeta: v1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, pm, site, func() error {
		pm.Labels = map[string]string{
			v1beta1.ManagedByLabelKey: v1beta1.ManagedByLabelValue,
		}
		// Suspended is intentionally absent: CreateOrUpdate does a full spec
		// replacement (regular Update, not SSA), so any prior Suspended=true is
		// cleared when Package Manager is re-enabled.
		pm.Spec = v1beta1.PackageManagerSpec{
			AwsAccountId:         site.Spec.AwsAccountId,
			ClusterDate:          site.Spec.ClusterDate,
			WorkloadCompoundName: site.Spec.WorkloadCompoundName,
			License:              site.Spec.PackageManager.License,
			Config: &v1beta1.PackageManagerConfig{
				Server: &v1beta1.PackageManagerServerConfig{
					Address:         "",
					RVersion:        []string{"/opt/R/default"},
					LauncherDir:     "/var/lib/rstudio-pm/launcher_internal",
					AccessLog:       "", // TODO: need STDOUT/STDERR options for Package Manager
					AccessLogFormat: packageManagerAccessLogFormat,
				},
				Http: &v1beta1.PackageManagerHttpConfig{
					Listen: ":4242",
				},
				Git: &v1beta1.PackageManagerGitConfig{
					AllowUnsandboxedGitBuilds: true,
				},
				Metrics: &v1beta1.PackageManagerMetricsConfig{
					Enabled: true,
				},
				Debug: &v1beta1.PackageManagerDebugConfig{
					Log: packageManagerDebugLogConfig,
				},
				Repos: &v1beta1.PackageManagerReposConfig{
					PyPI:         "pypi",
					CRAN:         "cran",
					Bioconductor: "bioconductor",
				},
			},
			Volume:     site.Spec.PackageManager.Volume,
			SecretType: site.Spec.Secret.Type,
			Url:        packageManagerUrl,
			DatabaseConfig: v1beta1.PostgresDatabaseConfig{
				Host:           dbHost,
				DropOnTeardown: site.Spec.DropDatabaseOnTeardown,
				SslMode:        sslMode,
			},
			MainDatabaseCredentialSecret: site.Spec.MainDatabaseCredentialSecret,
			IngressClass:                 site.Spec.IngressClass,
			IngressAnnotations:           site.Spec.IngressAnnotations,
			Image:                        site.Spec.PackageManager.Image,
			ImagePullPolicy:              site.Spec.PackageManager.ImagePullPolicy,
			ImagePullSecrets:             site.Spec.ImagePullSecrets,
			ChronicleAgentImage:          site.Spec.Chronicle.AgentImage,
			NodeSelector:                 site.Spec.PackageManager.NodeSelector,
			AddEnv:                       site.Spec.PackageManager.AddEnv,
			Secret:                       site.Spec.Secret,
			WorkloadSecret:               site.Spec.WorkloadSecret,
			Replicas:                     product.PassDefaultReplicas(site.Spec.PackageManager.Replicas, 1),
			GitSSHKeys:                   site.Spec.PackageManager.GitSSHKeys,
			AzureFiles:                   site.Spec.PackageManager.AzureFiles,
		}

		if site.Spec.PackageManager.S3Bucket != "" {
			if pm.Spec.Config.Storage == nil {
				pm.Spec.Config.Storage = &v1beta1.PackageManagerStorageConfig{}
			}
			pm.Spec.Config.Storage.Default = "S3"

			if pm.Spec.Config.S3Storage == nil {
				pm.Spec.Config.S3Storage = &v1beta1.PackageManagerS3StorageConfig{}
			}
			pm.Spec.Config.S3Storage.Bucket = site.Spec.PackageManager.S3Bucket
			pm.Spec.Config.S3Storage.Prefix = site.Name + "/ppm-v0"
			pm.Spec.Config.S3Storage.Region = product.GetAWSRegion()
		}

		// Propagate additional config from Site to PackageManager
		pm.Spec.Config.AdditionalConfig = site.Spec.PackageManager.AdditionalConfig

		// Propagate OIDC authentication configuration
		if site.Spec.PackageManager.Auth != nil && site.Spec.PackageManager.Auth.Type == v1beta1.AuthTypeOidc {
			pm.Spec.Config.OpenIDConnect = &v1beta1.PackageManagerOIDCConfig{
				ClientId:         site.Spec.PackageManager.Auth.ClientId,
				ClientSecretFile: "/etc/rstudio-pm/oidc-client-secret",
				Issuer:           site.Spec.PackageManager.Auth.Issuer,
				RequireLogin:     true,
			}
			// PPM requires at least one of Scope, RoleClaim, or GroupToScopeMapping.
			// Default to repos:read:* only when no alternative is configured via AdditionalConfig.
			if site.Spec.PackageManager.AdditionalConfig == "" ||
				(!containsGcfgKey(site.Spec.PackageManager.AdditionalConfig, "OpenIDConnect", "Scope") &&
					!containsGcfgKey(site.Spec.PackageManager.AdditionalConfig, "OpenIDConnect", "RoleClaim") &&
					!containsGcfgKey(site.Spec.PackageManager.AdditionalConfig, "OpenIDConnect", "GroupToScopeMapping")) {
				pm.Spec.Config.OpenIDConnect.Scope = "repos:read:*"
			}
			if site.Spec.PackageManager.Auth.GroupsClaim != "" {
				pm.Spec.Config.OpenIDConnect.GroupsClaim = site.Spec.PackageManager.Auth.GroupsClaim
			}
			// Propagate the OIDC client secret key so the volume factory can mount it
			pm.Spec.OIDCClientSecretKey = site.Spec.PackageManager.OIDCClientSecretKey
		}

		// Auto-configure Identity Federation entries based on product flags
		var idfEntries []v1beta1.PackageManagerIdentityFederationConfig
		if site.Spec.OIDCIssuerURL != "" {
			audience := site.Spec.OIDCAudience
			if audience == "" {
				l.V(0).Info("OIDCAudience is not set; Identity Federation entries will have an empty audience and PPM auth projected SA tokens will not work. Set spec.oidcAudience (e.g. 'sts.amazonaws.com' for EKS)")
			}
			if site.Spec.Connect.AuthenticatedRepos {
				idfEntries = append(idfEntries, v1beta1.PackageManagerIdentityFederationConfig{
					Name:          "connect",
					Issuer:        site.Spec.OIDCIssuerURL,
					Audience:      audience,
					Subject:       fmt.Sprintf("system:serviceaccount:%s:%s-connect", req.Namespace, req.Name),
					Scope:         "repos:read:*",
					UniqueIdClaim: "sub",
					UsernameClaim: "sub",
				})
			}
			if site.Spec.Workbench.AuthenticatedRepos {
				idfEntries = append(idfEntries, v1beta1.PackageManagerIdentityFederationConfig{
					Name:          "workbench",
					Issuer:        site.Spec.OIDCIssuerURL,
					Audience:      audience,
					Subject:       fmt.Sprintf("system:serviceaccount:%s:%s-workbench", req.Namespace, req.Name),
					Scope:         "repos:read:*",
					UniqueIdClaim: "sub",
					UsernameClaim: "sub",
				})
			}
		} else if site.Spec.Connect.AuthenticatedRepos || site.Spec.Workbench.AuthenticatedRepos {
			l.Info("AuthenticatedRepos is enabled but OIDCIssuerURL is empty; Identity Federation will not be configured")
		}
		if len(idfEntries) > 0 {
			pm.Spec.Config.IdentityFederation = idfEntries
		}

		return nil
	}); err != nil {
		l.Error(err, "error creating package manager instance")
		return err
	}

	return nil
}

// disablePackageManager suspends Package Manager by marking the existing PackageManager CR with Suspended=true.
// The Package Manager controller then removes serving resources (Deployment/Service/Ingress) while
// preserving data resources (PVC, database, secrets).
//
// If no PackageManager CR exists yet (Package Manager was never enabled), this is a no-op.
// When Package Manager is re-enabled, reconcilePackageManager overwrites Suspended back to nil and
// performs a full reconcile.
func (r *SiteReconciler) disablePackageManager(ctx context.Context, req controllerruntime.Request, l logr.Logger) error {
	l = l.WithValues("event", "disable-package-manager")

	pm := &v1beta1.PackageManager{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, pm); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("PackageManager CR not found, nothing to suspend")
			return nil
		}
		return err
	}

	if pm.Spec.Suspended != nil && *pm.Spec.Suspended {
		l.Info("PackageManager already suspended")
		return nil
	}

	patch := client.MergeFrom(pm.DeepCopy())
	suspended := true
	pm.Spec.Suspended = &suspended
	if err := r.Patch(ctx, pm, patch); err != nil {
		l.Error(err, "error suspending PackageManager CR")
		return err
	}

	l.Info("PackageManager CR suspended")
	return nil
}

// cleanupPackageManager deletes the PackageManager CR when teardown=true.
//
// WARNING: This is a DESTRUCTIVE operation. Deleting the PackageManager CR triggers the PackageManager
// finalizer which permanently destroys:
//   - The Package Manager database and all its data
//   - All secrets (database credentials, provisioning keys, etc.)
//   - Persistent volumes and claims
//   - All deployed Kubernetes resources
//
// This is triggered by Site.Spec.PackageManager.Teardown=true (when Enabled=false).
// Re-enabling Package Manager after teardown will start fresh with a new database.
func (r *SiteReconciler) cleanupPackageManager(ctx context.Context, req controllerruntime.Request, l logr.Logger) error {
	l = l.WithValues("event", "cleanup-package-manager")

	pmKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicDelete(ctx, r, l, pmKey, &v1beta1.PackageManager{}); err != nil {
		return err
	}

	return nil
}

// containsGcfgKey checks whether a raw gcfg config string contains a key assignment
// (e.g. "Scope = ...") within the given section (e.g. "OpenIDConnect").
func containsGcfgKey(config, section, key string) bool {
	inSection := false
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip gcfg comment lines (';' and '#' are valid comment prefixes)
		if strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			// Strip whitespace inside brackets so "[ OpenIDConnect ]" matches "OpenIDConnect"
			inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			inner = strings.TrimSpace(inner)
			inSection = strings.EqualFold(inner, section)
			continue
		}
		if inSection && len(trimmed) >= len(key) &&
			strings.EqualFold(trimmed[:len(key)], key) {
			rest := trimmed[len(key):]
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '=' {
				if strings.Contains(trimmed, "=") {
					return true
				}
			}
		}
	}
	return false
}
