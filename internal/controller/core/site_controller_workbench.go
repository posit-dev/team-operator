package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/rstudio/goex/ptr"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func (r *SiteReconciler) reconcileWorkbench(
	ctx context.Context,
	req controllerruntime.Request,
	site *v1beta1.Site,
	dbHost string,
	sslMode string,
	volumeName string,
	storageClassName string,
	additionalVolumes []product.VolumeSpec,
	packageManagerRepoUrl string,
	workbenchUrl string,
) error {

	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-workbench",
	)

	workbenchServerImage := site.Spec.Workbench.Image

	defaultSessionImage := site.Spec.Workbench.DefaultSessionImage
	if defaultSessionImage == "" {
		defaultSessionImage = workbenchServerImage
	}

	workbenchLogLevel := v1beta1.WorkbenchLogLevelInfo
	if site.Spec.Debug {
		workbenchLogLevel = v1beta1.WorkbenchLogLevelDebug
	}

	workbenchLogFormat := v1beta1.WorkbenchLogFormatPretty
	if site.Spec.LogFormat == product.LogFormatJson {
		workbenchLogFormat = v1beta1.WorkbenchLogFormatJson
	}

	adminGroup := "workbench-admin"
	if len(site.Spec.Workbench.AdminGroups) > 0 {
		adminGroup = strings.Join(site.Spec.Workbench.AdminGroups, ",")
	}

	adminSuperuserGroup := ""
	if len(site.Spec.Workbench.AdminSuperuserGroups) > 0 {
		adminSuperuserGroup = strings.Join(site.Spec.Workbench.AdminSuperuserGroups, ",")
	}

	threadPoolSize := 16
	proxyMaxWaitSecs := 30
	vsCodeArgs := "--host=0.0.0.0"
	resourceProfiles := defaultWorkbenchResourceProfiles()
	if site.Spec.Workbench.ExperimentalFeatures != nil {
		if site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize != nil {
			threadPoolSize = *site.Spec.Workbench.ExperimentalFeatures.WwwThreadPoolSize
		}

		if site.Spec.Workbench.ExperimentalFeatures.LauncherSessionsProxyTimeoutSeconds != nil {
			proxyMaxWaitSecs = *site.Spec.Workbench.ExperimentalFeatures.LauncherSessionsProxyTimeoutSeconds
		}

		if site.Spec.Workbench.ExperimentalFeatures.VsCodeExtensionsDir != "" {
			vsCodeArgs += " --extensions-dir=" + site.Spec.Workbench.ExperimentalFeatures.VsCodeExtensionsDir
		}

		if len(site.Spec.Workbench.ExperimentalFeatures.ResourceProfiles) > 0 {
			resourceProfiles = site.Spec.Workbench.ExperimentalFeatures.ResourceProfiles
		}
	}

	targetWorkbench := &v1beta1.Workbench{
		ObjectMeta: v1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				v1beta1.ManagedByLabelKey: LabelManagedByValue,
			},
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1beta1.WorkbenchSpec{
			AwsAccountId:         site.Spec.AwsAccountId,
			ClusterDate:          site.Spec.ClusterDate,
			WorkloadCompoundName: site.Spec.WorkloadCompoundName,
			OffHostExecution:     true,
			Snowflake:            site.Spec.Workbench.Snowflake,
			Config: v1beta1.WorkbenchConfig{
				WorkbenchIniConfig: v1beta1.WorkbenchIniConfig{
					Launcher: &v1beta1.WorkbenchLauncherConfig{
						Server: &v1beta1.WorkbenchLauncherServerConfig{
							Address:              "127.0.0.1",
							Port:                 "5559",
							ServerUser:           "rstudio-server",
							AdminGroup:           "rstudio-server",
							AuthorizationEnabled: 1,
							ThreadPoolSize:       4,
						},
						Cluster: &v1beta1.WorkbenchLauncherClusterConfig{
							Name: "Kubernetes",
							Type: "Kubernetes",
						},
					},
					VsCode: &v1beta1.WorkbenchVsCodeConfig{
						Enabled:                 1,
						Args:                    vsCodeArgs,
						SessionTimeoutKillHours: site.Spec.Workbench.VSCodeSettings.SessionTimeoutKillHours,
					},
					Logging: &v1beta1.WorkbenchLoggingConfig{
						All: &v1beta1.WorkbenchLoggingSection{
							LogLevel:         workbenchLogLevel,
							LoggerType:       v1beta1.WorkbenchLoggerTypeStdErr,
							LogMessageFormat: workbenchLogFormat,
						},
					},
					Jupyter: &v1beta1.WorkbenchJupyterConfig{
						NotebooksEnabled: 1,
						LabsEnabled:      1,
					},
					RServer: &v1beta1.WorkbenchRServerConfig{
						LoadBalancingEnabled:                   1,
						ServerSharedStoragePath:                "/mnt/shared-storage",
						ServerHealthCheckEnabled:               1,
						AuthPamSessionsEnabled:                 1,
						AdminEnabled:                           1,
						AdminGroup:                             adminGroup,
						AdminSuperuserGroup:                    adminSuperuserGroup,
						WwwPort:                                8787,
						ServerProjectSharing:                   0,
						LauncherAddress:                        "127.0.0.1",
						LauncherPort:                           5559,
						LauncherSessionsEnabled:                1,
						WwwFrameOrigin:                         fmt.Sprintf("https://%s", site.Spec.Domain),
						UserProvisioningEnabled:                1,
						UserHomedirPath:                        "/home",
						WwwThreadPoolSize:                      threadPoolSize,
						LauncherSessionsProxyTimeoutSeconds:    proxyMaxWaitSecs,
						WorkbenchApiEnabled:                    site.Spec.Workbench.ApiSettings.WorkbenchApiEnabled,
						WorkbenchApiAdminEnabled:               site.Spec.Workbench.ApiSettings.WorkbenchApiAdminEnabled,
						WorkbenchApiSuperAdminEnabled:          site.Spec.Workbench.ApiSettings.WorkbenchApiSuperAdminEnabled,
						LauncherSessionsAutoUpdate:             1,
						LauncherSessionsInitContainerImageName: site.Spec.Workbench.SessionInitContainerImageName,
						LauncherSessionsInitContainerImageTag:  site.Spec.Workbench.SessionInitContainerImageTag,
					},

					// NOTE: this gets overwritten later when we configure off-host execution (adhoc in the workbench controller)
					LauncherKubernetes: &v1beta1.WorkbenchLauncherKubernetesConfig{},

					Resources: resourceProfiles,
				},
				WorkbenchSessionIniConfig: v1beta1.WorkbenchSessionIniConfig{
					RSession: &v1beta1.WorkbenchRSessionConfig{
						// TODO: need TLS to be configurable... for plaintext sites...
						DefaultRSConnectServer: "https://" + prefixDomain(site.Spec.Connect.DomainPrefix, site.Spec.Domain, v1beta1.SiteSubDomain),
						CopilotEnabled:         1,
					},
					// TODO: configure the expected package manager repositories...?
					Repos: &v1beta1.WorkbenchRepoConfig{
						// don't want two definitions here... they make things slow!
						// TODO: also worth noting that OS is hard-coded here
						CRAN: packageManagerRepoUrl,
					},
					Positron: &v1beta1.WorkbenchPositronConfig{
						Enabled:                      site.Spec.Workbench.PositronSettings.Enabled,
						Exe:                          site.Spec.Workbench.PositronSettings.Exe,
						Args:                         site.Spec.Workbench.PositronSettings.Args,
						DefaultSessionContainerImage: site.Spec.Workbench.PositronSettings.DefaultSessionContainerImage,
						SessionContainerImages:       site.Spec.Workbench.PositronSettings.SessionContainerImages,
						PositronSessionPath:          site.Spec.Workbench.PositronSettings.PositronSessionPath,
						SessionNoProfile:             site.Spec.Workbench.PositronSettings.SessionNoProfile,
						UserDataDir:                  site.Spec.Workbench.PositronSettings.UserDataDir,
						AllowFileDownloads:           site.Spec.Workbench.PositronSettings.AllowFileDownloads,
						AllowFileUploads:             site.Spec.Workbench.PositronSettings.AllowFileUploads,
						SessionTimeoutKillHours:      site.Spec.Workbench.PositronSettings.SessionTimeoutKillHours,
					},
				},
				WorkbenchSessionNewlineConfig: v1beta1.WorkbenchSessionNewlineConfig{
					VsCodeExtensionsConf:   site.Spec.Workbench.VsCodeExtensions,
					PositronExtensionsConf: site.Spec.Workbench.PositronSettings.Extensions,
				},
				WorkbenchSessionJsonConfig: v1beta1.WorkbenchSessionJsonConfig{
					VSCodeUserSettingsJson:   site.Spec.Workbench.VsCodeUserSettings,
					PositronUserSettingsJson: site.Spec.Workbench.PositronSettings.UserSettings,
				},
				WorkbenchProfilesConfig: v1beta1.WorkbenchProfilesConfig{
					LauncherKubernetesProfiles: map[string]v1beta1.WorkbenchLauncherKubernetesProfilesConfigSection{
						"*": {
							// TODO: allow configuring...
							ContainerImages:       product.ConcatLists([]string{defaultSessionImage}, site.Spec.Workbench.ExtraSessionImages),
							DefaultContainerImage: defaultSessionImage,
							AllowUnknownImages:    1,
							MemoryRequestRatio:    getMemoryRequestRatio(site.Spec.Workbench.ExperimentalFeatures),
							CpuRequestRatio:       getCpuRequestRatio(site.Spec.Workbench.ExperimentalFeatures),
							ResourceProfiles:      getResourceProfileKeys(resourceProfiles),
						},
					},
				},
			},
			SecretConfig: v1beta1.WorkbenchSecretConfig{
				WorkbenchSecretIniConfig: v1beta1.WorkbenchSecretIniConfig{
					Databricks: map[string]*v1beta1.WorkbenchDatabricksConfig{},
				},
			},
			SessionConfig: &product.SessionConfig{
				Pod: &product.PodConfig{
					NodeSelector:       site.Spec.Workbench.NodeSelector,
					Tolerations:        site.Spec.Workbench.SessionTolerations,
					ImagePullPolicy:    site.Spec.Workbench.ImagePullPolicy,
					ImagePullSecrets:   product.MakePullSecrets(site.Spec.ImagePullSecrets),
					ServiceAccountName: fmt.Sprintf("%s-workbench-session", req.Name),
				},
			},
			License:    site.Spec.Workbench.License,
			Volume:     site.Spec.Workbench.Volume,
			SecretType: site.Spec.Secret.Type,
			Url:        workbenchUrl,
			ParentUrl:  site.Spec.Domain,
			DatabaseConfig: v1beta1.PostgresDatabaseConfig{
				Host:           dbHost,
				DropOnTeardown: site.Spec.DropDatabaseOnTeardown,
				SslMode:        sslMode,
			},
			MainDatabaseCredentialSecret: site.Spec.MainDatabaseCredentialSecret,
			IngressClass:                 site.Spec.IngressClass,
			IngressAnnotations:           site.Spec.IngressAnnotations,
			Image:                        workbenchServerImage,
			ImagePullPolicy:              site.Spec.Workbench.ImagePullPolicy,
			ChronicleAgentImage:          site.Spec.Chronicle.AgentImage,
			AdditionalVolumes:            additionalVolumes,
			ImagePullSecrets:             site.Spec.ImagePullSecrets,
			NodeSelector:                 site.Spec.Workbench.NodeSelector,
			Tolerations:                  site.Spec.Workbench.Tolerations,
			AddEnv:                       site.Spec.Workbench.AddEnv,
			Auth:                         site.Spec.Workbench.Auth,
			Secret:                       site.Spec.Secret,
			WorkloadSecret:               site.Spec.WorkloadSecret,
			Replicas:                     product.PassDefaultReplicas(site.Spec.Workbench.Replicas, 1),
		},
	}
	// potentially enable experimental features
	if site.Spec.Workbench.ExperimentalFeatures != nil {
		// TODO: we could really benefit from a "nil-safe" way to set a value in a struct... filling in defaults along the way
		if site.Spec.Workbench.ExperimentalFeatures.EnableManagedCredentialJobs {
			targetWorkbench.Spec.Config.RSession.ManagedCredentialsInJobsEnabled = 1
		}

		// the default behavior in workbench is "ask," but we relax that to "no" as it can be costly in disk and time
		if site.Spec.Workbench.ExperimentalFeatures.SessionSaveActionDefault == v1beta1.SessionSaveActionEmpty {
			targetWorkbench.Spec.Config.RSession.SessionSaveActionDefault = v1beta1.SessionSaveActionNone
		} else {
			targetWorkbench.Spec.Config.RSession.SessionSaveActionDefault = site.Spec.Workbench.ExperimentalFeatures.SessionSaveActionDefault
		}

		if site.Spec.Workbench.ExperimentalFeatures.NonRoot {
			targetWorkbench.Spec.NonRoot = true
		}
		if site.Spec.Workbench.ExperimentalFeatures.FirstProjectTemplatePath != "" {
			if targetWorkbench.Spec.Config.RSession != nil {
				targetWorkbench.Spec.Config.RSession.SessionFirstProjectTemplatePath = site.Spec.Workbench.ExperimentalFeatures.FirstProjectTemplatePath
			} else {
				targetWorkbench.Spec.Config.RSession = &v1beta1.WorkbenchRSessionConfig{
					SessionFirstProjectTemplatePath: site.Spec.Workbench.ExperimentalFeatures.FirstProjectTemplatePath,
				}
			}
		}
		if site.Spec.Workbench.ExperimentalFeatures.SessionServiceAccountName != "" {
			saName := site.Spec.Workbench.ExperimentalFeatures.SessionServiceAccountName
			if targetWorkbench.Spec.SessionConfig == nil {
				targetWorkbench.Spec.SessionConfig = &product.SessionConfig{
					Pod: &product.PodConfig{
						ServiceAccountName: saName,
					},
				}
			} else if targetWorkbench.Spec.SessionConfig.Pod == nil {
				targetWorkbench.Spec.SessionConfig.Pod = &product.PodConfig{
					ServiceAccountName: saName,
				}
			} else {
				targetWorkbench.Spec.SessionConfig.Pod.ServiceAccountName = saName
			}
		}
		if site.Spec.Workbench.ExperimentalFeatures.PrivilegedSessions {
			if targetWorkbench.Spec.SessionConfig != nil {
				if targetWorkbench.Spec.SessionConfig.Pod != nil {
					targetWorkbench.Spec.SessionConfig.Pod.ContainerSecurityContext = v12.SecurityContext{
						Privileged: ptr.To(true),
					}
				} else {
					targetWorkbench.Spec.SessionConfig.Pod = &product.PodConfig{
						ContainerSecurityContext: v12.SecurityContext{
							Privileged: ptr.To(true),
						},
					}
				}
			} else {
				targetWorkbench.Spec.SessionConfig = &product.SessionConfig{
					Pod: &product.PodConfig{
						ContainerSecurityContext: v12.SecurityContext{
							Privileged: ptr.To(true),
						},
					},
				}
			}

		}

		if site.Spec.Workbench.ExperimentalFeatures.VsCodePath != "" {
			if targetWorkbench.Spec.Config.WorkbenchIniConfig.VsCode == nil {
				targetWorkbench.Spec.Config.WorkbenchIniConfig.VsCode = &v1beta1.WorkbenchVsCodeConfig{}
			}
			targetWorkbench.Spec.Config.WorkbenchIniConfig.VsCode.Exe = site.Spec.Workbench.ExperimentalFeatures.VsCodePath
		}

		targetWorkbench.Spec.DsnSecret = site.Spec.Workbench.ExperimentalFeatures.DsnSecret

		// set session env vars and image pull policy
		if targetWorkbench.Spec.SessionConfig != nil {
			if targetWorkbench.Spec.SessionConfig.Pod != nil {
				targetWorkbench.Spec.SessionConfig.Pod.Env = site.Spec.Workbench.ExperimentalFeatures.SessionEnvVars
				targetWorkbench.Spec.SessionConfig.Pod.ImagePullPolicy = site.Spec.Workbench.ExperimentalFeatures.SessionImagePullPolicy
			} else {
				targetWorkbench.Spec.SessionConfig.Pod = &product.PodConfig{
					Env:             site.Spec.Workbench.ExperimentalFeatures.SessionEnvVars,
					ImagePullPolicy: site.Spec.Workbench.ExperimentalFeatures.SessionImagePullPolicy,
				}
			}
		} else {
			targetWorkbench.Spec.SessionConfig = &product.SessionConfig{
				Pod: &product.PodConfig{
					Env:             site.Spec.Workbench.ExperimentalFeatures.SessionEnvVars,
					ImagePullPolicy: site.Spec.Workbench.ExperimentalFeatures.SessionImagePullPolicy,
				},
			}
		}

		if site.Spec.Workbench.ExperimentalFeatures.LauncherEnvPath != "" {
			targetWorkbench.Spec.Config.WorkbenchDcfConfig = v1beta1.WorkbenchDcfConfig{
				LauncherEnv: &v1beta1.WorkbenchLauncherEnvConfig{
					JobType: "session",
					Environment: map[string]string{
						"PATH": site.Spec.Workbench.ExperimentalFeatures.LauncherEnvPath,
					},
				},
			}
		}

		if site.Spec.Workbench.ExperimentalFeatures.ChronicleSidecarProductApiKeyEnabled {
			targetWorkbench.Spec.ChronicleSidecarProductApiKeyEnabled = true
		}

		if site.Spec.Workbench.ExperimentalFeatures.ForceAdminUiEnabled {
			targetWorkbench.Spec.Config.RServer.ForceAdminUiEnabled = 1
		}
	}

	// set user provisioning
	if site.Spec.Workbench.CreateUsersAutomatically {
		targetWorkbench.Spec.Config.RServer.UserProvisioningRegisterOnFirstLogin = 1
	}

	// set databricks config if it exists
	databricksEnabled := false
	for k, v := range site.Spec.Workbench.Databricks {
		databricksEnabled = true
		targetWorkbench.Spec.SecretConfig.Databricks[k] = &v1beta1.WorkbenchDatabricksConfig{
			Name:     v.Name,
			Url:      v.Url,
			ClientId: v.ClientId,
		}
	}
	if site.Spec.Workbench.ExperimentalFeatures != nil && site.Spec.Workbench.ExperimentalFeatures.DatabricksForceEnabled {
		databricksEnabled = true
	}
	if databricksEnabled {
		if targetWorkbench.Spec.Config.WorkbenchIniConfig.RServer == nil {
			targetWorkbench.Spec.Config.WorkbenchIniConfig.RServer = &v1beta1.WorkbenchRServerConfig{}
		}
		targetWorkbench.Spec.Config.WorkbenchIniConfig.RServer.DatabricksEnabled = 1
	}

	// Apply Site-level Jupyter configuration if provided
	if site.Spec.Workbench.JupyterConfig != nil {
		targetWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter = site.Spec.Workbench.JupyterConfig
	}

	// if landing/auth page is customized
	if site.Spec.Workbench.AuthLoginPageHtml != "" {
		targetWorkbench.Spec.AuthLoginPageHtml = site.Spec.Workbench.AuthLoginPageHtml
	}

	// if volumeSource.type is set, then force volume creation for Workbench
	if site.Spec.VolumeSource.Type != v1beta1.VolumeSourceTypeNone {
		if targetWorkbench.Spec.Volume == nil {
			targetWorkbench.Spec.Volume = &product.VolumeSpec{}
		}
		targetWorkbench.Spec.Volume.Create = true
		targetWorkbench.Spec.Volume.AccessModes = []string{"ReadWriteMany"}
		targetWorkbench.Spec.Volume.VolumeName = volumeName
		if site.Spec.VolumeSource.Type == v1beta1.VolumeSourceTypeAzureNetApp {
			targetWorkbench.Spec.Volume.Size = "50Gi" // netapp operator has 50Gi minimum for pvc
		}
		targetWorkbench.Spec.Volume.StorageClassName = storageClassName
	}

	workbench := &v1beta1.Workbench{
		ObjectMeta: v1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, workbench, site, func() error {
		workbench.Labels = targetWorkbench.Labels
		workbench.Spec = targetWorkbench.Spec
		return nil
	}); err != nil {
		l.Error(err, "error creating workbench instance")
		return err
	}

	return nil
}

func defaultWorkbenchResourceProfiles() map[string]*v1beta1.WorkbenchLauncherKubnernetesResourcesConfigSection {
	return map[string]*v1beta1.WorkbenchLauncherKubnernetesResourcesConfigSection{
		"default": {
			Name:  "Small",
			Cpus:  "1",
			MemMb: "2000",
		},
		"medium": {
			Name:  "Medium",
			Cpus:  "2",
			MemMb: "4000",
		},
		"zz-large": {
			Name:  "Large",
			Cpus:  "4",
			MemMb: "8000",
		},
	}
}

// getResourceProfileKeys extracts the keys from a resource profiles map
func getResourceProfileKeys(resourceProfiles map[string]*v1beta1.WorkbenchLauncherKubnernetesResourcesConfigSection) []string {
	keys := make([]string, 0, len(resourceProfiles))
	for key := range resourceProfiles {
		keys = append(keys, key)
	}
	return keys
}

// getCpuRequestRatio returns the configured CPU request ratio, with kubebuilder default fallback
func getCpuRequestRatio(experimentalFeatures *v1beta1.InternalWorkbenchExperimentalFeatures) string {
	if experimentalFeatures != nil && experimentalFeatures.CpuRequestRatio != "" {
		return experimentalFeatures.CpuRequestRatio
	}
	return "0.6" // Default when experimentalFeatures is nil or field is empty (kubebuilder sets this for new resources)
}

// getMemoryRequestRatio returns the configured memory request ratio, with kubebuilder default fallback
func getMemoryRequestRatio(experimentalFeatures *v1beta1.InternalWorkbenchExperimentalFeatures) string {
	if experimentalFeatures != nil && experimentalFeatures.MemoryRequestRatio != "" {
		return experimentalFeatures.MemoryRequestRatio
	}
	return "0.8" // Default when experimentalFeatures is nil or field is empty (kubebuilder sets this for new resources)
}
