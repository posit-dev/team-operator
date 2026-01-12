package core

import (
	"context"
	"fmt"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *SiteReconciler) reconcileConnect(
	ctx context.Context,
	req controllerruntime.Request,
	site *v1beta1.Site,
	dbHost string,
	sslMode string,
	volumeName string,
	storageClassName string,
	additionalVolumes []product.VolumeSpec,
	packageManagerRepoUrl string,
	connectUrl string,
) error {

	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-connect",
	)

	connectDebugLog := false
	if site.Spec.Debug {
		connectDebugLog = true
	}

	connectLogFormat := v1beta1.ConnectServiceLogFormatText
	connectAccessLogFormat := v1beta1.ConnectAccessLogFormatCommon

	if site.Spec.LogFormat == product.LogFormatJson {
		connectLogFormat = v1beta1.ConnectServiceLogFormatJson
		connectAccessLogFormat = v1beta1.ConnectAccessLogFormatJson
	}

	targetConnect := v1beta1.Connect{
		ObjectMeta: v1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				v1beta1.ManagedByLabelKey: v1beta1.ManagedByLabelValue,
			},
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1beta1.ConnectSpec{
			AwsAccountId:         site.Spec.AwsAccountId,
			ClusterDate:          site.Spec.ClusterDate,
			WorkloadCompoundName: site.Spec.WorkloadCompoundName,
			License:              site.Spec.Connect.License,
			SessionConfig: &product.SessionConfig{
				Pod: &product.PodConfig{
					ImagePullPolicy:    site.Spec.Connect.ImagePullPolicy,
					ImagePullSecrets:   product.MakePullSecrets(site.Spec.ImagePullSecrets),
					ServiceAccountName: fmt.Sprintf("%s-connect-session", req.Name),
				},
			},
			Config: v1beta1.ConnectConfig{
				Applications: &v1beta1.ConnectApplicationsConfig{
					BundleRetentionLimit:     2,
					PythonEnvironmentReaping: true,
					OAuthIntegrationsEnabled: true,
					ScheduleConcurrency:      2,
				},
				Server: &v1beta1.ConnectServerConfig{
					// This will be filled in by the controller... see "Url" below
					Address:                "",
					FrameOptionsDashboard:  "NONE",
					FrameOptionsContent:    "NONE",
					DefaultContentListView: v1beta1.ContentListViewCompact,
					LoggedInWarning:        site.Spec.Connect.LoggedInWarning,
					PublicWarning:          site.Spec.Connect.PublicWarning,
					HideEmailAddresses:     true,
				},
				Http: &v1beta1.ConnectHttpConfig{
					ForceSecure: true,
					Listen:      ":3939",
				},
				Metrics: &v1beta1.ConnectMetricsConfig{
					PrometheusListen: ":3232",
				},
				Logging: &v1beta1.ConnectLoggingConfig{
					ServiceLog:       "STDERR",
					ServiceLogLevel:  "INFO",
					ServiceLogFormat: connectLogFormat,
					AccessLog:        "STDERR",
					AccessLogFormat:  connectAccessLogFormat,
					AuditLog:         "",
					AuditLogFormat:   connectLogFormat,
				},
				Authorization: &v1beta1.ConnectAuthorizationConfig{
					ViewersCanOnlySeeThemselves: false,
					DefaultUserRole:             v1beta1.ConnectPublisherRole,
					PublishersCanManageVanities: true,
				},
				R: &v1beta1.ConnectRConfig{
					Enabled:                         true,
					PositPackageManagerURLRewriting: v1beta1.ConnectPackageManagerUrlRewriteForceBinary,
				},
				Quarto: &v1beta1.ConnectQuartoConfig{
					Enabled: true,
				},
				Scheduler: &v1beta1.ConnectSchedulerConfig{
					MaxMemoryRequest: 8 * 1024 * 1024 * 1024,
					MaxMemoryLimit:   8 * 1024 * 1024 * 1024,
					MaxCPURequest:    4,
					MaxCPULimit:      4,
				},
				RPackageRepository: map[string]v1beta1.RPackageRepositoryConfig{
					"CRAN": {
						Url: packageManagerRepoUrl,
					},
					"RSPM": {
						Url: packageManagerRepoUrl,
					},
				},
			},
			Volume:     site.Spec.Connect.Volume,
			SecretType: site.Spec.Secret.Type,
			Url:        connectUrl,
			DatabaseConfig: v1beta1.PostgresDatabaseConfig{
				Host:                  dbHost,
				DropOnTeardown:        site.Spec.DropDatabaseOnTeardown,
				SslMode:               sslMode,
				Schema:                "",
				InstrumentationSchema: "",
			},
			MainDatabaseCredentialSecret: site.Spec.MainDatabaseCredentialSecret,
			IngressClass:                 site.Spec.IngressClass,
			IngressAnnotations:           site.Spec.IngressAnnotations,
			Image:                        site.Spec.Connect.Image,
			SessionImage:                 site.Spec.Connect.SessionImage,
			ImagePullPolicy:              site.Spec.Connect.ImagePullPolicy,
			ImagePullSecrets:             site.Spec.ImagePullSecrets,
			ChronicleAgentImage:          site.Spec.Chronicle.AgentImage,
			AdditionalVolumes:            additionalVolumes,
			NodeSelector:                 site.Spec.Connect.NodeSelector,
			AddEnv:                       site.Spec.Connect.AddEnv,
			// default to true...
			OffHostExecution: true,
			Auth:             site.Spec.Connect.Auth,
			Secret:           site.Spec.Secret,
			WorkloadSecret:   site.Spec.WorkloadSecret,
			Debug:            connectDebugLog,
			Replicas:         product.PassDefaultReplicas(site.Spec.Connect.Replicas, 1),
		},
	}

	if site.Spec.Connect.GPUSettings != nil {
		if site.Spec.Connect.GPUSettings.NvidiaGPULimit > 0 {
			targetConnect.Spec.Config.Scheduler.NvidiaGPULimit = site.Spec.Connect.GPUSettings.NvidiaGPULimit
		}
		if site.Spec.Connect.GPUSettings.MaxNvidiaGPULimit > 0 {
			targetConnect.Spec.Config.Scheduler.MaxNvidiaGPULimit = site.Spec.Connect.GPUSettings.MaxNvidiaGPULimit
		}
		if site.Spec.Connect.GPUSettings.AMDGPULimit > 0 {
			targetConnect.Spec.Config.Scheduler.AMDGPULimit = site.Spec.Connect.GPUSettings.AMDGPULimit
		}
		if site.Spec.Connect.GPUSettings.MaxAMDGPULimit > 0 {
			targetConnect.Spec.Config.Scheduler.MaxAMDGPULimit = site.Spec.Connect.GPUSettings.MaxAMDGPULimit
		}
	}

	// Apply database settings if configured
	if site.Spec.Connect.DatabaseSettings != nil {
		targetConnect.Spec.DatabaseConfig.Schema = site.Spec.Connect.DatabaseSettings.Schema
		targetConnect.Spec.DatabaseConfig.InstrumentationSchema = site.Spec.Connect.DatabaseSettings.InstrumentationSchema
	}

	// Apply ScheduleConcurrency if configured
	if site.Spec.Connect.ScheduleConcurrency >= 0 {
		targetConnect.Spec.Config.Applications.ScheduleConcurrency = site.Spec.Connect.ScheduleConcurrency
	}

	if site.Spec.Connect.ExperimentalFeatures != nil {
		if site.Spec.Connect.ExperimentalFeatures.MailSender != "" {
			// enable SMTP!
			targetConnect.Spec.Config.Server.EmailProvider = "SMTP"
			targetConnect.Spec.Config.Server.SenderEmail = site.Spec.Connect.ExperimentalFeatures.MailSender
			targetConnect.Spec.Config.Server.SenderEmailDisplayName = site.Spec.Connect.ExperimentalFeatures.MailDisplayName
			targetConnect.Spec.Config.Server.EmailTo = site.Spec.Connect.ExperimentalFeatures.MailTarget
		}
		targetConnect.Spec.DsnSecret = site.Spec.Connect.ExperimentalFeatures.DsnSecret

		// set session service account, env vars, and image pull policy
		if targetConnect.Spec.SessionConfig != nil {
			if targetConnect.Spec.SessionConfig.Pod != nil {
				targetConnect.Spec.SessionConfig.Pod.Env = site.Spec.Connect.ExperimentalFeatures.SessionEnvVars
				targetConnect.Spec.SessionConfig.Pod.ImagePullPolicy = site.Spec.Connect.ExperimentalFeatures.SessionImagePullPolicy
				targetConnect.Spec.SessionConfig.Pod.ServiceAccountName = site.Spec.Connect.ExperimentalFeatures.SessionServiceAccountName
			} else {
				targetConnect.Spec.SessionConfig.Pod = &product.PodConfig{
					Env:                site.Spec.Connect.ExperimentalFeatures.SessionEnvVars,
					ImagePullPolicy:    site.Spec.Connect.ExperimentalFeatures.SessionImagePullPolicy,
					ServiceAccountName: site.Spec.Connect.ExperimentalFeatures.SessionServiceAccountName,
				}
			}
		} else {
			targetConnect.Spec.SessionConfig = &product.SessionConfig{
				Pod: &product.PodConfig{
					Env:                site.Spec.Connect.ExperimentalFeatures.SessionEnvVars,
					ImagePullPolicy:    site.Spec.Connect.ExperimentalFeatures.SessionImagePullPolicy,
					ServiceAccountName: site.Spec.Connect.ExperimentalFeatures.SessionServiceAccountName,
				},
			}
		}

		if site.Spec.Connect.ExperimentalFeatures.ChronicleSidecarProductApiKeyEnabled {
			targetConnect.Spec.ChronicleSidecarProductApiKeyEnabled = true
		}
	}

	// if volumeSource.type is set, then force volume creation for Connect
	if site.Spec.VolumeSource.Type != v1beta1.VolumeSourceTypeNone {
		if targetConnect.Spec.Volume == nil {
			targetConnect.Spec.Volume = &product.VolumeSpec{}
		}
		targetConnect.Spec.Volume.Create = true
		targetConnect.Spec.Volume.AccessModes = []string{"ReadWriteMany"}
		targetConnect.Spec.Volume.VolumeName = volumeName
		if site.Spec.VolumeSource.Type == v1beta1.VolumeSourceTypeAzureNetApp {
			targetConnect.Spec.Volume.Size = "50Gi" // netapp operator has 50Gi minimum for pvc
		}
		targetConnect.Spec.Volume.StorageClassName = storageClassName
	}

	existingConnect := v1beta1.Connect{}

	connectKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, connectKey, &existingConnect, &targetConnect); err != nil {
		l.Error(err, "error creating connect instance")
		return err
	}
	return nil
}
