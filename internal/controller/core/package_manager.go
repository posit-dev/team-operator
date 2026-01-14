package core

import (
	"context"
	"fmt"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/db"
	"github.com/rstudio/goex/ptr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasses,verbs=get;list;watch;create;update;patch;delete

func (r *PackageManagerReconciler) CleanupPackageManager(ctx context.Context, req ctrl.Request, pm *positcov1beta1.PackageManager) (ctrl.Result, error) {
	if err := r.cleanupDeployedService(ctx, req, pm); err != nil {
		return ctrl.Result{}, err
	}
	if err := internal.CleanupProvisioningKey(ctx, pm, r, req); err != nil {
		return ctrl.Result{}, err
	}
	if err := db.CleanupDatabasePasswordSecret(ctx, r, req, pm.ComponentName()); err != nil {
		return ctrl.Result{}, err
	}
	if err := db.CleanupDatabase(ctx, r, req, pm.ComponentName()); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *PackageManagerReconciler) cleanupDeployedService(ctx context.Context, req ctrl.Request, pm *positcov1beta1.PackageManager) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-service",
		"product", "package-manager",
	)

	// we reuse this key, because everything is named the same...
	key := client.ObjectKey{
		Name:      pm.ComponentName(),
		Namespace: req.Namespace,
	}

	// INGRESS

	existingIngress := &networkingv1.Ingress{}
	if err := internal.BasicDelete(ctx, r, l, key, existingIngress); err != nil {
		return err
	}

	// SERVICE

	existingService := &corev1.Service{}
	if err := internal.BasicDelete(ctx, r, l, key, existingService); err != nil {
		return err
	}

	// DEPLOYMENT

	existingDeployment := &v1.Deployment{}
	if err := internal.BasicDelete(ctx, r, l, key, existingDeployment); err != nil {
		return err
	}

	// VOLUME
	// NOTE: we delete the volume universally, even if create was false...
	//   this ensures that we have the resource completely removed, whether it
	//   was created, forgotten, or never created.

	existingPvc := &corev1.PersistentVolumeClaim{}
	if err := internal.BasicDelete(ctx, r, l, key, existingPvc); err != nil {
		return err
	}

	// SERVICE ACCOUNT

	existingServiceAccount := &corev1.ServiceAccount{}
	if err := internal.BasicDelete(ctx, r, l, key, existingServiceAccount); err != nil {
		return err
	}

	// CONFIGMAP

	existingConfigmap := &corev1.ConfigMap{}
	if err := internal.BasicDelete(ctx, r, l, key, existingConfigmap); err != nil {
		return err
	}

	return nil
}

const packageManagerConfigShaKey = "package-manager.posit.team/configmap-sha"

func (r *PackageManagerReconciler) ReconcilePackageManager(ctx context.Context, req ctrl.Request, pm *positcov1beta1.PackageManager) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-package-manager-service",
		"product", "package-manager",
	)

	// create database
	secretKey := "pkg-db-password"
	if err := db.EnsureDatabaseExists(ctx, r, req, pm, pm.Spec.DatabaseConfig, pm.ComponentName(), "", []string{"pm", "metrics"}, pm.Spec.Secret, pm.Spec.WorkloadSecret, pm.Spec.MainDatabaseCredentialSecret, secretKey); err != nil {
		l.Error(err, "error creating database", "database", pm.ComponentName())
		return ctrl.Result{}, err
	}

	// create managerial secret

	// TODO: we should probably initialize this higher up the stack so that we can
	//   use it when we are provisioning Workbench, a PM provisioning operator, etc.
	//   For now, we just use it to give to Package Manager
	if _, err := internal.EnsureProvisioningKey(ctx, pm, r, req, pm); err != nil {
		l.Error(err, "error ensuring that provisioning key exists")
		return ctrl.Result{}, err
	} else {
		l.Info("successfully created or retrieved provisioning key value")
	}

	pm.Status.KeySecretRef = corev1.SecretReference{
		Name:      pm.KeySecretName(),
		Namespace: req.Namespace,
	}
	if err := r.Status().Update(ctx, pm); err != nil {
		l.Error(err, "Error updating status")
		return ctrl.Result{}, err
	}

	// TODO: at some point, postgres should probably be an option... (i.e. multi-tenant world?)
	if pm.Spec.Config.Database == nil {
		pm.Spec.Config.Database = &positcov1beta1.PackageManagerDatabaseConfig{}
	}
	pm.Spec.Config.Database.Provider = "postgres"

	if pm.Spec.Config.Postgres == nil {
		pm.Spec.Config.Postgres = &positcov1beta1.PackageManagerPostgresConfig{}
	}
	pm.Spec.Config.Postgres.URL = db.DatabaseUrl(pm.Spec.DatabaseConfig.Host, pm.ComponentName(), "", db.QueryParams("pm", pm.Spec.DatabaseConfig.SslMode)).String()
	pm.Spec.Config.Postgres.UsageDataURL = db.DatabaseUrl(pm.Spec.DatabaseConfig.Host, pm.ComponentName(), "", db.QueryParams("metrics", pm.Spec.DatabaseConfig.SslMode)).String()

	// Handle Azure Files configuration
	if pm.Spec.AzureFiles != nil {
		// Set Server.DataDir to the mount path when Azure Files is used
		if pm.Spec.Config.Server == nil {
			pm.Spec.Config.Server = &positcov1beta1.PackageManagerServerConfig{}
		}
		pm.Spec.Config.Server.DataDir = "/mnt/azure-files"

		// Set Storage type configuration
		if pm.Spec.Config.Storage == nil {
			pm.Spec.Config.Storage = &positcov1beta1.PackageManagerStorageConfig{}
		}
		pm.Spec.Config.Storage.Default = "file"

		if err := r.createAzureFilesStoragePVC(ctx, pm); err != nil {
			l.Error(err, "error creating Azure Files PVC")
			return ctrl.Result{}, err
		}
	}

	// then create the service itself
	res, err := r.ensureDeployedService(ctx, req, pm)
	if err != nil {
		l.Error(err, "error deploying service")
		return res, err
	}

	// TODO: should we watch for happy pods?

	// set to ready if it is not set yet...
	if !pm.Status.Ready {
		pm.Status.Ready = true
		if err := r.Status().Update(ctx, pm); err != nil {
			l.Error(err, "Error setting ready status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

var defaultPmVolumeSize = resource.MustParse("2Gi")

// createAzureFilesStoragePVC creates a PVC that uses the Azure Files CSI StorageClass
func (r *PackageManagerReconciler) createAzureFilesStoragePVC(ctx context.Context, pm *positcov1beta1.PackageManager) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "create-azure-files-pvc",
		"product", "package-manager",
	)

	if pm.Spec.AzureFiles == nil || pm.Spec.AzureFiles.StorageClassName == "" || pm.Spec.AzureFiles.ShareSizeGiB < 100 {
		return fmt.Errorf("Invalid AzureFiles configuration. Missing StorageClassName or invalid ShareSizeGiB (minimum 100 GiB).")
	}

	pvcName := fmt.Sprintf("%s-azure-files", pm.ComponentName())

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pvcName,
			Namespace:       pm.Namespace,
			Labels:          pm.KubernetesLabels(),
			OwnerReferences: pm.OwnerReferencesForChildren(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &pm.Spec.AzureFiles.StorageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", pm.Spec.AzureFiles.ShareSizeGiB)),
				},
			},
		},
	}

	// Apply the PVC (dynamic provisioning will create the PV)
	if err := r.Create(ctx, pvc); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			l.Error(err, "Failed to create Azure Files PersistentVolumeClaim", "pvc", pvcName)
			return err
		}

		// PVC already exists, update it
		existingPVC := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: pm.Namespace}, existingPVC); err != nil {
			l.Error(err, "Failed to get existing Azure Files PersistentVolumeClaim", "pvc", pvcName)
			return err
		}

		if existingPVC.Spec.StorageClassName == nil || *existingPVC.Spec.StorageClassName != pm.Spec.AzureFiles.StorageClassName {
			existingPVC.Spec.StorageClassName = &pm.Spec.AzureFiles.StorageClassName
			if err := r.Update(ctx, existingPVC); err != nil {
				l.Error(err, "Failed to update Azure Files PersistentVolumeClaim", "pvc", pvcName)
				return err
			}
		}
	}

	return nil
}

func (r *PackageManagerReconciler) ensureDeployedService(ctx context.Context, req ctrl.Request, pm *positcov1beta1.PackageManager) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "deploy-service",
		"product", "package-manager",
	)

	// SECRETS

	if pm.Spec.Secret.Type == product.SiteSecretAws {
		// deploy SecretProviderClass for app secrets
		// Build the secret refs map
		secretRefs := map[string]string{
			"pkg.lic":  "pkg-license",
			"key":      "pkg-secret-key",
			"password": "pkg-db-password",
		}

		if targetSpc, err := product.GetSecretProviderClassForAllSecrets(
			pm, pm.SecretProviderClassName(),
			req.Namespace, pm.Spec.Secret.VaultName,
			secretRefs,
			map[string]map[string]string{
				pm.KeySecretName(): {
					"key": "key",
				},
				fmt.Sprintf("%s-db", pm.ComponentName()): {
					"password": "password",
					"license":  "pkg.lic",
				},
			},
		); err != nil {
			return ctrl.Result{}, err
		} else {
			spc := &secretstorev1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pm.SecretProviderClassName(),
					Namespace: req.Namespace,
				},
			}
			if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, spc, pm, func() error {
				spc.Labels = targetSpc.Labels
				spc.Spec = targetSpc.Spec
				return nil
			}); err != nil {
				l.Error(err, "error provisioning SecretProviderClass for secrets")
				return ctrl.Result{}, err
			}
		}

		// Deploy separate SecretProviderClass for SSH keys if configured
		if len(pm.Spec.GitSSHKeys) > 0 {
			// Build SSH secret refs map
			sshSecretRefs := map[string]string{}
			for _, sshKey := range pm.Spec.GitSSHKeys {
				// Only add AWS Secrets Manager SSH keys
				if sshKey.SecretRef.Source == "aws-secrets-manager" {
					// Map the mount name to the actual field name in the vault
					// sshKey.Name is used as the mount point name
					// sshKey.SecretRef.Name is the actual field name in the AWS secret
					sshSecretRefs[sshKey.Name] = sshKey.SecretRef.Name
				}
			}

			if len(sshSecretRefs) > 0 {
				// Construct SSH vault name: {workloadCompoundName}-{siteName}-ssh-ppm-keys.posit.team
				sshVaultName := fmt.Sprintf("%s-%s-ssh-ppm-keys.posit.team", pm.Spec.WorkloadCompoundName, pm.SiteName())
				sshSpcName := fmt.Sprintf("%s-ssh-secrets", pm.ComponentName())

				if targetSshSpc, err := product.GetSecretProviderClassForAllSecrets(
					pm, sshSpcName,
					req.Namespace, sshVaultName,
					sshSecretRefs,
					nil, // No K8s secrets needed - mounting directly from CSI
				); err != nil {
					return ctrl.Result{}, err
				} else {
					sshSpc := &secretstorev1.SecretProviderClass{
						ObjectMeta: metav1.ObjectMeta{
							Name:      sshSpcName,
							Namespace: req.Namespace,
						},
					}
					if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, sshSpc, pm, func() error {
						sshSpc.Labels = targetSshSpc.Labels
						sshSpc.Spec = targetSshSpc.Spec
						return nil
					}); err != nil {
						l.Error(err, "error provisioning SecretProviderClass for SSH secrets")
						return ctrl.Result{}, err
					}
				}
			}
		}
	}

	// modifications to config based on parameter settings
	configCopy := pm.Spec.Config.DeepCopy()

	if pm.Spec.Url != "" {
		// TODO: server address protocol should perhaps be configurable...?
		//       or we should figure out a way to do TLS in development...
		configCopy.Server.Address = "https://" + pm.Spec.Url
	}

	// CONFIGMAP

	cmSha := ""
	if rawConfig, err := configCopy.GenerateGcfg(); err != nil {
		l.Error(err, "error generating gcfg values")
		return ctrl.Result{}, err
	} else {
		cmData := map[string]string{
			"rstudio-pm.gcfg": rawConfig,
		}

		if tmpCmSha, err := product.ComputeSha256(cmData); err != nil {
			l.Error(err, "error computing sha256 for configmap")
			return ctrl.Result{}, err
		} else {
			cmSha = tmpCmSha
		}

		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pm.ComponentName(),
				Namespace: req.Namespace,
			},
		}
		if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, configmap, pm, func() error {
			configmap.Labels = pm.KubernetesLabels()
			configmap.Data = cmData
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	// NOTE: The role used for ppm includes the site name (pm.Name) because there are
	// separate policies defined for each site so that a single bucket may be used across
	// multiple sites without risking Security Breach.
	roleArn := "arn:aws:iam::" + pm.GetAwsAccountId() + ":role/" +
		pm.ShortName() +
		"." + pm.GetClusterDate() +
		"." + pm.Name + // <--- this is the segment that is different
		"." + pm.WorkloadCompoundName() +
		".posit.team"

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pm.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, serviceAccount, pm, func() error {
		serviceAccount.Labels = pm.KubernetesLabels()
		serviceAccount.Annotations = map[string]string{"eks.amazonaws.com/role-arn": roleArn}
		// TODO: we should specify secrets here for "minimal access"
		serviceAccount.Secrets = nil
		serviceAccount.ImagePullSecrets = nil
		serviceAccount.AutomountServiceAccountToken = ptr.To(true)
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}

	// VOLUME CLAIM

	// TODO: note that if you change Volume.Create to false, we will just ignore it forever... (and not clean up)
	if pm.Spec.Volume != nil && pm.Spec.Volume.Create {
		if pvc, err := product.DefinePvc(pm, req, pm.ComponentName(), pm.Spec.Volume, defaultPmVolumeSize); err != nil {
			l.Error(err, "error defining PVC", "pvc", pvc.Name)
			return ctrl.Result{}, err
		} else {
			if err := internal.PvcCreateOrUpdate(ctx, r, l, req.NamespacedName, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
				l.Error(err, "error creating or updating PVC", "pvc", pvc.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// DEPLOYMENT

	var pullSecrets []corev1.LocalObjectReference
	for _, s := range pm.Spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: s})
	}

	secretVolumeFactory := pm.CreateSecretVolumeFactory()

	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pm.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	// TODO: deployment will _definitely_ need custom CreateOrUpdate work at some point
	//   i.e. to handle version upgrades, etc. We could add an Updater() callback, or a
	//   CustomComparator... or just decide to inline the logic
	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, deployment, pm, func() error {
		deployment.Labels = pm.KubernetesLabels()
		deployment.Spec = v1.DeploymentSpec{
			Replicas: ptr.To(int32(product.PassDefaultReplicas(pm.Spec.Replicas, 1))),
			Selector: &metav1.LabelSelector{
				MatchLabels: pm.SelectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: pm.KubernetesLabels(),
					Annotations: map[string]string{
						// TODO: this is a hack to get config changes to trigger a new deployment (for now)
						//   In the future, we could use our own mechanism and decide whether to restart or SIGHUP the service...
						packageManagerConfigShaKey: cmSha,
					},
				},
				Spec: corev1.PodSpec{
					EnableServiceLinks: ptr.To(false),
					NodeSelector:       pm.Spec.NodeSelector,
					ImagePullSecrets:   pullSecrets,
					ServiceAccountName: pm.ComponentName(),
					Containers: product.ConcatLists([]corev1.Container{
						{
							Name:            "rspm",
							Image:           pm.Spec.Image,
							ImagePullPolicy: pm.Spec.ImagePullPolicy,
							Command:         []string{"tini", "--"},
							Args:            []string{"/usr/local/bin/startup.sh"},
							Env: product.ConcatLists(
								secretVolumeFactory.EnvVars(),
								product.StringMapToEnvVars(pm.Spec.AddEnv),
								[]corev1.EnvVar{},
							),
							Ports: []corev1.ContainerPort{
								internal.DefaultPortPackageManagerHTTP.ContainerPort("http"),
								internal.DefaultPortPackageManagerMetrics.ContainerPort("metrics"),
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                ptr.To(int64(999)),
								RunAsNonRoot:             ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: "RuntimeDefault",
								},
							},
							VolumeMounts: product.ConcatLists([]corev1.VolumeMount{
								{
									Name:      "config-volume",
									ReadOnly:  true,
									MountPath: "/etc/rstudio-pm/rstudio-pm.gcfg",
									SubPath:   "rstudio-pm.gcfg",
								},
							},
								func() []corev1.VolumeMount {
									if pm.Spec.AzureFiles != nil {
										return []corev1.VolumeMount{
											{
												Name:      "azure-files-volume",
												MountPath: "/mnt/azure-files",
												ReadOnly:  false,
											},
										}
									}
									return []corev1.VolumeMount{}
								}(),
								secretVolumeFactory.VolumeMounts(),
							),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:              resource.MustParse("100m"),
									corev1.ResourceMemory:           resource.MustParse("2Gi"),
									corev1.ResourceEphemeralStorage: resource.MustParse("500Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:              resource.MustParse("2000m"),
									corev1.ResourceMemory:           resource.MustParse("4Gi"),
									corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/__ping__",
										Port: intstr.IntOrString{Type: intstr.String, StrVal: "http"},
									},
								},
								InitialDelaySeconds:           3,
								TimeoutSeconds:                1,
								PeriodSeconds:                 3,
								SuccessThreshold:              1,
								FailureThreshold:              2,
								TerminationGracePeriodSeconds: nil,
							},
						},
					},
					),
					Affinity: &corev1.Affinity{
						PodAntiAffinity: positcov1beta1.ComponentSpecPodAntiAffinity(pm, req.Namespace),
					},
					Volumes: product.ConcatLists([]corev1.Volume{
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: pm.ComponentName(),
									},
									Items: []corev1.KeyToPath{
										{Key: "rstudio-pm.gcfg", Path: "rstudio-pm.gcfg"},
									},
									DefaultMode: nil,
								},
							},
						},
					},
						func() []corev1.Volume {
							if pm.Spec.AzureFiles != nil {
								return []corev1.Volume{
									{
										Name: "azure-files-volume",
										VolumeSource: corev1.VolumeSource{
											PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
												ClaimName: fmt.Sprintf("%s-azure-files", pm.ComponentName()),
												ReadOnly:  false,
											},
										},
									},
								}
							}
							return []corev1.Volume{}
						}(),
						secretVolumeFactory.Volumes(),
					),
				},
			},
		}
		if pm.Spec.Sleep {
			deployment.Spec.Template.Spec.Containers[0].Command = []string{"sleep"}
			deployment.Spec.Template.Spec.Containers[0].Args = []string{"infinity"}
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}

	// SERVICE
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pm.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, service, pm, func() error {
		service.Labels = pm.KubernetesLabels()
		service.Spec = corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   1,
						StrVal: "http",
					},
				},
			},
			Selector:                 pm.KubernetesLabels(),
			Type:                     "ClusterIP",
			PublishNotReadyAddresses: false,
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}

	// INGRESS

	ing_annotations := map[string]string{}
	for k, v := range pm.Spec.IngressAnnotations {
		ing_annotations[k] = v
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pm.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, ingress, pm, func() error {
		ingress.Labels = pm.KubernetesLabels()
		ingress.Annotations = ing_annotations
		ingress.Spec = networkingv1.IngressSpec{
			// IngressClass set below
			// TODO: TLS configuration, perhaps
			TLS: nil,
			Rules: []networkingv1.IngressRule{
				{
					Host: pm.Spec.Url,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: pm.ComponentName(),
											Port: networkingv1.ServiceBackendPort{
												Name: "http",
											},
										},
										Resource: nil,
									},
								},
							},
						},
					},
				},
			},
		}
		// only define the ingressClassName if it is specified on the site
		if pm.Spec.IngressClass != "" {
			ingress.Spec.IngressClassName = &pm.Spec.IngressClass
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}

	// POD DISRUPTION BUDGET
	if err := CreateOrUpdateDisruptionBudget(
		ctx, req, r.Client, r.Scheme, pm, pm, ptr.To(product.DetermineMinAvailableReplicas(pm.Spec.Replicas)), nil,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
