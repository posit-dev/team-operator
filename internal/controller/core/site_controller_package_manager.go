package core

import (
	"context"
	"fmt"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
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
				Issuer:       site.Spec.PackageManager.Auth.Issuer,
				RequireLogin: true,
				Scope:        "repos:read:*",
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
				audience = "sts.amazonaws.com"
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
