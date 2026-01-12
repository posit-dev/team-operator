package core

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/api/templates"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/db"
	"github.com/rstudio/goex/ptr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasses,verbs=get;list;watch;create;update;patch;delete

func (r *ConnectReconciler) ReconcileConnect(ctx context.Context, req ctrl.Request, c *positcov1beta1.Connect) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-connect",
		"product", "connect",
	)

	// create database
	secretKey := "pub-db-password"

	schema := "connect"
	if c.Spec.DatabaseConfig.Schema != "" {
		schema = c.Spec.DatabaseConfig.Schema
	}

	instrumentationSchema := "instrumentation"
	if c.Spec.DatabaseConfig.InstrumentationSchema != "" {
		instrumentationSchema = c.Spec.DatabaseConfig.InstrumentationSchema
	}

	dbSchemas := []string{schema}
	if schema != instrumentationSchema {
		dbSchemas = append(dbSchemas, instrumentationSchema)
	}

	if err := db.EnsureDatabaseExists(ctx, r, req, c, c.Spec.DatabaseConfig, c.ComponentName(), "", dbSchemas, c.Spec.Secret, c.Spec.WorkloadSecret, c.Spec.MainDatabaseCredentialSecret, secretKey); err != nil {
		l.Error(err, "error creating database", "database", c.ComponentName())
		return ctrl.Result{}, err
	}

	// create managerial secret if secret type kubernetes
	// ... otherwise reference the presumed-already-existing secret
	if c.GetSecretType() == product.SiteSecretKubernetes {
		// NOTE: we do not retain this value locally. Instead we just reference the key in the Status
		if _, err := internal.EnsureProvisioningKey(ctx, c, r, req, c); err != nil {
			l.Error(err, "error ensuring that provisioning key exists")
			return ctrl.Result{}, err
		} else {
			l.Info("successfully created or retrieved provisioning key value")
		}

		// update status with key secret reference
		c.Status.KeySecretRef = corev1.SecretReference{
			Name:      c.KeySecretName(),
			Namespace: req.Namespace,
		}
		if err := r.Status().Update(ctx, c); err != nil {
			l.Error(err, "Error updating status")
			return ctrl.Result{}, err
		}
	}

	// TODO: at some point, postgres should probably be an option... (i.e. multi-tenant world?)
	if c.Spec.Config.Database != nil {
		c.Spec.Config.Database.Provider = "postgres"
	} else {
		c.Spec.Config.Database = &positcov1beta1.ConnectDatabaseConfig{
			Provider: "postgres",
		}
	}

	dbUrl := db.DatabaseUrl(c.Spec.DatabaseConfig.Host, c.ComponentName(), "", db.QueryParams(schema, c.Spec.DatabaseConfig.SslMode)).String()
	dbInstUrl := db.DatabaseUrl(c.Spec.DatabaseConfig.Host, c.ComponentName(), "", db.QueryParams(instrumentationSchema, c.Spec.DatabaseConfig.SslMode)).String()
	if c.Spec.Config.Postgres != nil {
		c.Spec.Config.Postgres.URL = dbUrl
		c.Spec.Config.Postgres.InstrumentationURL = dbInstUrl
	} else {
		c.Spec.Config.Postgres = &positcov1beta1.ConnectPostgresConfig{
			URL:                dbUrl,
			InstrumentationURL: dbInstUrl,
		}
	}

	// then create the service itself
	res, err := r.ensureDeployedService(ctx, req, c)
	if err != nil {
		l.Error(err, "error deploying service")
		return res, err
	}

	// TODO: should we watch for happy pods?

	// set to ready if it is not set yet...
	if !c.Status.Ready {
		c.Status.Ready = true
		if err := r.Status().Update(ctx, c); err != nil {
			l.Error(err, "Error setting ready status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ConnectReconciler) forwardMiddleware(c *positcov1beta1.Connect) string {
	return fmt.Sprintf("%s-forward", c.ComponentName())
}

func (r *ConnectReconciler) deployTraefikMiddlewares(ctx context.Context, req ctrl.Request, c *positcov1beta1.Connect) error {

	if err := internal.DeployTraefikForwardMiddleware(ctx, req, r, r.forwardMiddleware(c), c); err != nil {
		return err
	}

	return nil
}

var defaultConnectVolumeSize = resource.MustParse("2Gi")

const connectConfigShaKey = "connect.posit.team/configmap-sha"
const connectTemplateShaKey = "connect.posit.team/template-sha"

func (r *ConnectReconciler) ensureDeployedService(ctx context.Context, req ctrl.Request, c *positcov1beta1.Connect) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "deploy-service",
		"product", "connect",
	)
	ctx = logr.NewContext(ctx, l)

	// this object key is used by configmap, deployment, and service
	key := client.ObjectKey{
		Name:      c.ComponentName(),
		Namespace: req.Namespace,
	}

	// modifications to config based on parameter settings
	configCopy := c.Spec.Config.DeepCopy()

	// only set privileged if offHostExecution is _off_
	privileged := true
	if c.Spec.OffHostExecution {
		privileged = false
	}

	// SECRETS
	if c.GetSecretType() == product.SiteSecretAws {
		// deploy SecretProviderClass for app secrets
		spcNameKey := client.ObjectKey{Name: c.SecretProviderClassName(), Namespace: req.Namespace}

		allSecrets := map[string]string{
			"secret.key":      "pub-secret-key",
			"pub-db-password": "pub-db-password",
			"pub.lic":         "pub-license",
		}
		kubernetesSecrets := map[string]map[string]string{
			fmt.Sprintf("%s-secret-key", c.ComponentName()): {
				"secret.key": "secret.key",
			},
			fmt.Sprintf("%s-db", c.ComponentName()): {
				"password": "pub-db-password",
				"license":  "pub.lic",
			},
		}
		if configCopy.Server.EmailProvider == "SMTP" {
			allSecrets["smtp-host"] = "pub-smtp-host"
			allSecrets["smtp-password"] = "pub-smtp-password"
			allSecrets["smtp-port"] = "pub-smtp-port"
			allSecrets["smtp-user"] = "pub-smtp-user"
			kubernetesSecrets[fmt.Sprintf("%s-smtp", c.ComponentName())] = map[string]string{
				"smtp-host":     "smtp-host",
				"smtp-password": "smtp-password",
				"smtp-port":     "smtp-port",
				"smtp-user":     "smtp-user",
			}
		}
		if c.Spec.DsnSecret != "" {
			allSecrets["odbc.ini"] = c.Spec.DsnSecret
		}

		if c.Spec.Auth.Type == positcov1beta1.AuthTypeOidc {
			allSecrets["client-secret"] = "pub-client-secret"
		}

		if c.Spec.ChronicleSidecarProductApiKeyEnabled {
			allSecrets["pub-chronicle-api-key"] = "pub-chronicle-api-key"
			kubernetesSecrets[fmt.Sprintf("%s-chronicle", c.ComponentName())] = map[string]string{
				"pub-chronicle-api-key": "pub-chronicle-api-key",
			}
		}

		// TODO: should have a generic "secret vault" config...
		if targetSpc, err := product.GetSecretProviderClassForAllSecrets(
			c, c.SecretProviderClassName(),
			req.Namespace, c.Spec.Secret.VaultName,
			allSecrets,
			kubernetesSecrets,
		); err != nil {
			return ctrl.Result{}, err
		} else {
			existingSpc := &secretsstorev1.SecretProviderClass{}

			if err := internal.BasicCreateOrUpdate(ctx, r, l, spcNameKey, existingSpc, targetSpc); err != nil {
				l.Error(err, "error provisioning SecretProviderClass for secrets")
				return ctrl.Result{}, err
			}
		}
	}

	if c.Spec.Url != "" {
		// TODO: server address protocol should perhaps be configurable...?
		//       or we should figure out a way to do TLS in development...
		address := "https://" + c.Spec.Url
		if configCopy.Server != nil {
			configCopy.Server.Address = address
		} else {
			configCopy.Server = &positcov1beta1.ConnectServerConfig{
				Address: address,
			}
		}
	}

	if c.Spec.OffHostExecution {
		if configCopy.Python != nil {
			configCopy.Python.Enabled = true
		} else {
			configCopy.Python = &positcov1beta1.ConnectPythonConfig{
				Enabled: true,
			}
		}
		volName := ""
		if c.Spec.Volume.Create {
			volName = c.ComponentName()
		}
		configCopy.Launcher = &positcov1beta1.ConnectLauncherConfig{
			Enabled:                  true,
			Kubernetes:               true,
			ClusterDefinition:        []string{"/etc/rstudio-connect/runtime.yaml"},
			KubernetesNamespace:      req.Namespace,
			KubernetesProfilesConfig: "/etc/rstudio-connect/launcher/launcher.kubernetes.profiles.conf",
			DataDirPVCName:           volName,
			KubernetesUseTemplates:   true,
			ScratchPath:              "/var/lib/rstudio-connect-launcher",
		}
	}

	if c.Spec.Auth.Type == positcov1beta1.AuthTypeOidc {
		if configCopy.Authentication != nil {
			configCopy.Authentication.Provider = "OAuth2"
		} else {
			configCopy.Authentication = &positcov1beta1.ConnectAuthenticationConfig{
				Provider: "OAuth2",
			}
		}

		// NOTE: the volume mount will be created elsewhere... keys must match...
		configCopy.OAuth2 = &positcov1beta1.ConnectOAuth2Config{
			ClientId:             c.Spec.Auth.ClientId,
			ClientSecretFile:     "/etc/rstudio-connect/client-secret",
			OpenIDConnectIssuer:  c.Spec.Auth.Issuer,
			RequireUsernameClaim: true,
			// this default works ok...? and is sorta the default for okta... modified below
			UniqueIdClaim: "email",
			Logging:       c.Spec.Debug,
		}

		if c.Spec.Auth.Groups {
			configCopy.OAuth2.GroupsAutoProvision = true
			if len(c.Spec.Auth.Scopes) == 0 {
				configCopy.OAuth2.CustomScope = []string{"groups"}
			}
		}

		if len(c.Spec.Auth.Scopes) > 0 {
			configCopy.OAuth2.CustomScope = c.Spec.Auth.Scopes
		}

		// override claim mapping
		if c.Spec.Auth.UsernameClaim != "" {
			configCopy.OAuth2.UsernameClaim = c.Spec.Auth.UsernameClaim
		}
		if c.Spec.Auth.EmailClaim != "" {
			configCopy.OAuth2.EmailClaim = c.Spec.Auth.EmailClaim
			// use a different email claim for uniqueid if email is invalid
			configCopy.OAuth2.UniqueIdClaim = c.Spec.Auth.EmailClaim
		}
		if c.Spec.Auth.UniqueIdClaim != "" {
			configCopy.OAuth2.UniqueIdClaim = c.Spec.Auth.UniqueIdClaim
		}
		if c.Spec.Auth.DisableGroupsClaim {
			// Explicitly set GroupsClaim to empty string to override Connect's default
			configCopy.OAuth2.GroupsClaim = ptr.To("")
		} else if c.Spec.Auth.GroupsClaim != "" {
			configCopy.OAuth2.GroupsClaim = ptr.To(c.Spec.Auth.GroupsClaim)
		}
	} else if c.Spec.Auth.Type == positcov1beta1.AuthTypeSaml {
		if c.Spec.Auth.SamlMetadataUrl == "" {
			return ctrl.Result{}, fmt.Errorf("SAML authentication requires a metadata URL to be specified")
		}

		// Validate mutual exclusivity between IdPAttributeProfile and individual attribute mappings
		hasIndividualAttributes := c.Spec.Auth.SamlUsernameAttribute != "" ||
			c.Spec.Auth.SamlFirstNameAttribute != "" ||
			c.Spec.Auth.SamlLastNameAttribute != "" ||
			c.Spec.Auth.SamlEmailAttribute != ""

		if c.Spec.Auth.SamlIdPAttributeProfile != "" && hasIndividualAttributes {
			return ctrl.Result{}, fmt.Errorf("SAML IdPAttributeProfile cannot be specified together with individual SAML attribute mappings (UsernameAttribute, FirstNameAttribute, LastNameAttribute, EmailAttribute)")
		}

		if configCopy.Authentication != nil {
			configCopy.Authentication.Provider = "saml"
		} else {
			configCopy.Authentication = &positcov1beta1.ConnectAuthenticationConfig{
				Provider: "saml",
			}
		}

		configCopy.SAML = &positcov1beta1.ConnectSamlConfig{
			IdPMetaDataURL: c.Spec.Auth.SamlMetadataUrl,
		}

		// Set IdPAttributeProfile or individual attributes
		if c.Spec.Auth.SamlIdPAttributeProfile != "" {
			configCopy.SAML.IdPAttributeProfile = c.Spec.Auth.SamlIdPAttributeProfile
		} else if hasIndividualAttributes {
			configCopy.SAML.UsernameAttribute = c.Spec.Auth.SamlUsernameAttribute
			configCopy.SAML.FirstNameAttribute = c.Spec.Auth.SamlFirstNameAttribute
			configCopy.SAML.LastNameAttribute = c.Spec.Auth.SamlLastNameAttribute
			configCopy.SAML.EmailAttribute = c.Spec.Auth.SamlEmailAttribute
		} else {
			// Default to "default" profile if nothing is specified
			configCopy.SAML.IdPAttributeProfile = "default"
		}
	}

	// Set role mappings in Authorization config
	if len(c.Spec.Auth.ViewerRoleMapping) > 0 || len(c.Spec.Auth.PublisherRoleMapping) > 0 || len(c.Spec.Auth.AdministratorRoleMapping) > 0 {
		if configCopy.Authorization == nil {
			configCopy.Authorization = &positcov1beta1.ConnectAuthorizationConfig{}
		}
		// Enable UserRoleGroupMapping when role mappings are specified
		configCopy.Authorization.UserRoleGroupMapping = true
		if len(c.Spec.Auth.ViewerRoleMapping) > 0 {
			configCopy.Authorization.ViewerRoleMapping = c.Spec.Auth.ViewerRoleMapping
		}
		if len(c.Spec.Auth.PublisherRoleMapping) > 0 {
			configCopy.Authorization.PublisherRoleMapping = c.Spec.Auth.PublisherRoleMapping
		}
		if len(c.Spec.Auth.AdministratorRoleMapping) > 0 {
			configCopy.Authorization.AdministratorRoleMapping = c.Spec.Auth.AdministratorRoleMapping
		}
	}

	// set log level for other fields
	if c.Spec.Debug {
		if configCopy.TableauIntegration != nil {
			configCopy.TableauIntegration.Logging = c.Spec.Debug
		} else {
			configCopy.TableauIntegration = &positcov1beta1.ConnectTableauIntegrationConfig{
				Logging: true,
			}
		}

		if configCopy.Server != nil {
			configCopy.Server.ProxyHeaderLogging = c.Spec.Debug
		} else {
			configCopy.Server = &positcov1beta1.ConnectServerConfig{
				ProxyHeaderLogging: c.Spec.Debug,
			}
		}
	}

	// CONFIGMAP

	cmSha := ""
	if rawConfig, err := configCopy.GenerateGcfg(); err != nil {
		l.Error(err, "error generating gcfg values")
		return ctrl.Result{}, err
	} else {
		targetConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            c.ComponentName(),
				Namespace:       req.Namespace,
				Labels:          c.KubernetesLabels(),
				OwnerReferences: c.OwnerReferencesForChildren(),
			},
			Data: map[string]string{
				"rstudio-connect.gcfg": rawConfig,
			},
		}

		if tmpCmSha, err := product.ComputeSha256(targetConfigmap.Data); err != nil {
			l.Error(err, "Error computing sha256 for configmap")
			return ctrl.Result{}, err
		} else {
			cmSha = tmpCmSha
		}

		// add runtime configuration when using off-host execution
		if c.Spec.OffHostExecution {
			if runtimeYaml, err := c.DefaultRuntimeYAML(); err != nil {
				return ctrl.Result{}, err
			} else {
				targetConfigmap.Data["runtime.yaml"] = runtimeYaml
			}

			targetConfigmap.Data["launcher.kubernetes.profiles.conf"] = ""
		}

		existingConfigmap := &corev1.ConfigMap{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingConfigmap, targetConfigmap); err != nil {
			return ctrl.Result{}, err
		}
	}

	// TEMPLATES

	templateCmSha := ""
	if c.Spec.OffHostExecution {
		templateKey := client.ObjectKey{
			Name:      c.TemplateConfigMapName(),
			Namespace: req.Namespace,
		}

		targetTemplatesConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            c.TemplateConfigMapName(),
				Namespace:       req.Namespace,
				Labels:          c.KubernetesLabels(),
				OwnerReferences: c.OwnerReferencesForChildren(),
			},
			Data: map[string]string{
				"job.tpl":                            templates.DumpJobTpl(),
				"service.tpl":                        templates.DumpServiceTpl(),
				"rstudio-library-templates-data.tpl": c.SessionConfigTemplateData(ctx),
			},
		}

		if tmpTemplateCmSha, err := product.ComputeSha256(targetTemplatesConfigmap.Data); err != nil {
			l.Error(err, "Error computing sha256 for template configmap")
			return ctrl.Result{}, err
		} else {
			templateCmSha = tmpTemplateCmSha
		}

		existingTemplatesConfigmap := &corev1.ConfigMap{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, templateKey, existingTemplatesConfigmap, targetTemplatesConfigmap); err != nil {
			return ctrl.Result{}, err
		}

		// provision the SiteSession SecretProviderClass...
		if siteSpc, err := c.SiteSessionSecretProviderClass(ctx); err != nil {
			return ctrl.Result{}, err
		} else if siteSpc != nil {
			// no error... and not nil SecretProviderClass
			siteSpcKey := client.ObjectKey{Namespace: req.Namespace, Name: siteSpc.Name}
			if err := internal.BasicCreateOrUpdate(ctx, r, l, siteSpcKey, &secretsstorev1.SecretProviderClass{}, siteSpc); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// SERVICE ACCOUNT & RBAC

	// TODO: there may be a time that you want to just generate a service account, even if Connect is on-host execution...
	if c.Spec.OffHostExecution {
		if err := internal.GenerateRbac(ctx, r, req, c); err != nil {
			l.Error(err, "Error generating service account and rbac")
			return ctrl.Result{}, err
		}
	}

	// SESSION SERVICE ACCOUNT
	saAnnotations := internal.AddIamAnnotation(fmt.Sprintf("%s-ses", c.ShortName()), req.Namespace, c.SiteName(), map[string]string{}, c)
	saName := c.SessionServiceAccountName()
	saKey := client.ObjectKey{Name: saName, Namespace: req.Namespace}
	targetServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            saName,
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			Annotations:     saAnnotations,
			OwnerReferences: c.OwnerReferencesForChildren(),
		},
	}

	existingServiceAccount := &corev1.ServiceAccount{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, saKey, existingServiceAccount, targetServiceAccount); err != nil {
		return ctrl.Result{}, err
	}

	// VOLUME CLAIM

	// TODO: note that if you change Volume.Create to false, we will just ignore it forever...
	if c.Spec.Volume != nil && c.Spec.Volume.Create {
		if pvc, err := product.DefinePvc(c, req, c.ComponentName(), c.Spec.Volume, defaultConnectVolumeSize); err != nil {
			l.Error(err, "error defining PVC", "pvc", pvc.Name)
			return ctrl.Result{}, err
		} else {
			// TODO: handle volume size changes...?
			// TODO: beware... some updates are invalid and will cause crash-loop forever
			//   everything is immutable in a PVC except for the storage request
			if err := internal.PvcCreateOrUpdate(ctx, r, l, key, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
				return ctrl.Result{}, err
			} else {
				l.Info("successfully created or updated PVC", "pvc", pvc.Name)
			}
		}
	}
	// create the volumes that will be referenced by the VolumeFactory later...
	for _, v := range c.Spec.AdditionalVolumes {
		if v.Create {
			// create the PVC
			if pvc, err := product.DefinePvc(c, req, v.PvcName, &v, resource.MustParse("10Gi")); err != nil {
				l.Error(err, "error defining PVC", "pvc", pvc.Name)
				return ctrl.Result{}, err
			} else {
				localKey := client.ObjectKey{Name: pvc.Name, Namespace: req.Namespace}
				if err := internal.PvcCreateOrUpdate(ctx, r, l, localKey, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
					l.Error(err, "error creating or updating PVC", "pvc", pvc.Name)
					return ctrl.Result{}, err
				} else {
					l.Info("successfully created or updated PVC", "pvc", pvc.Name)
				}
			}
		}
	}

	// DEPLOYMENT

	var pullSecrets []corev1.LocalObjectReference
	for _, s := range c.Spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: s})
	}

	maybeServiceAccountName := ""
	if c.Spec.OffHostExecution {
		maybeServiceAccountName = c.ComponentName()
	}

	volumeFactory := c.CreateVolumeFactory(configCopy)
	secretVolumeFactory := c.CreateSecretVolumeFactory(configCopy)

	var chronicleSeededEnv []corev1.EnvVar
	if c.Spec.ChronicleSidecarProductApiKeyEnabled {
		chronicleSeededEnv = []corev1.EnvVar{
			{Name: "CHRONICLE_CONNECT_APIKEY", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-chronicle", c.ComponentName())},
					Key:                  "pub-chronicle-api-key",
					Optional:             ptr.To(true),
				},
			}},
		}
	}

	targetDeployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			OwnerReferences: c.OwnerReferencesForChildren(),
		},
		Spec: v1.DeploymentSpec{
			Replicas: ptr.To(int32(product.PassDefaultReplicas(c.Spec.Replicas, 1))),
			Selector: &metav1.LabelSelector{
				MatchLabels: c.SelectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: c.KubernetesLabels(),
					Annotations: map[string]string{
						// TODO: this is a hack to get config changes to trigger a new deployment (for now)
						//   In the future, we could use our own mechanism and decide whether to restart or SIGHUP the service...
						connectConfigShaKey:   cmSha,
						connectTemplateShaKey: templateCmSha,
					},
				},
				Spec: corev1.PodSpec{
					EnableServiceLinks: ptr.To(false),
					NodeSelector:       c.Spec.NodeSelector,
					ImagePullSecrets:   pullSecrets,
					ServiceAccountName: maybeServiceAccountName,
					// TODO: go back to automounting service token...
					AutomountServiceAccountToken: ptr.To(false),
					Containers: product.ConcatLists([]corev1.Container{
						{
							Name:            "connect",
							Image:           c.Spec.Image,
							ImagePullPolicy: c.Spec.ImagePullPolicy,
							Command:         []string{"tini", "--"},
							Args:            []string{"/usr/local/bin/startup.sh"},
							Env: product.ConcatLists(
								volumeFactory.EnvVars(),
								secretVolumeFactory.EnvVars(),
								product.StringMapToEnvVars(c.Spec.AddEnv),
								[]corev1.EnvVar{
									{
										Name:  "LAUNCHER_INSTANCE_ID",
										Value: c.ComponentName(),
									},
								},
							),
							Ports: []corev1.ContainerPort{
								internal.DefaultPortConnectHTTP.ContainerPort("http"),
								internal.DefaultPortConnectMetrics.ContainerPort("metrics"),
							},
							SecurityContext: &corev1.SecurityContext{
								//RunAsUser:                ptr.To(int64(0)),
								RunAsNonRoot:             ptr.To(false),
								AllowPrivilegeEscalation: ptr.To(true),
								Privileged:               ptr.To(privileged),
								Capabilities:             &corev1.Capabilities{
									// Drop: []corev1.Capability{"ALL"},
								},
								//SeccompProfile: &corev1.SeccompProfile{
								// Type: "RuntimeDefault",
								//},
							},
							VolumeMounts: product.ConcatLists(
								volumeFactory.VolumeMounts(),
								secretVolumeFactory.VolumeMounts(),
								c.TokenVolumeMounts(),
							),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									// TODO: resources
									//"cpu":               resource.Quantity{Format: "2000m"},
									//"memory":            resource.Quantity{Format: "3Gi"},
									//"ephemeral-storage": resource.Quantity{Format: "100Mi"},
								},
								Limits: corev1.ResourceList{
									// TODO: resources
									//"cpu":               resource.Quantity{Format: "6000m"},
									//"memory":            resource.Quantity{Format: "8Gi"},
									//"ephemeral-storage": resource.Quantity{Format: "200Mi"},
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
								FailureThreshold:              3,
								TerminationGracePeriodSeconds: nil,
							},
						},
					},
						product.ChronicleSidecar(c, chronicleSeededEnv),
					),
					Affinity: &corev1.Affinity{
						PodAntiAffinity: positcov1beta1.ComponentSpecPodAntiAffinity(c, req.Namespace),
					},
					SecurityContext: &corev1.PodSecurityContext{
						//FSGroup: ptr.To(int64(999)),
					},
					Volumes: product.ConcatLists(
						volumeFactory.Volumes(),
						secretVolumeFactory.Volumes(),
						c.TokenVolumes(),
					),
				},
			},
		},
	}

	if c.Spec.Sleep {
		targetDeployment.Spec.Template.Spec.Containers[0].Command = []string{"sleep"}
		targetDeployment.Spec.Template.Spec.Containers[0].Args = []string{"infinity"}
	}

	existingDeployment := &v1.Deployment{}

	// TODO: deployment will _definitely_ need custom CreateOrUpdate work at some point
	//   i.e. to handle version upgrades, etc. We could add an Updater() callback, or a
	//   CustomComparator... or just decide to inline the logic
	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingDeployment, targetDeployment); err != nil {
		return ctrl.Result{}, err
	}

	// SERVICE

	targetService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			OwnerReferences: c.OwnerReferencesForChildren(),
			Annotations:     internal.TraefikStickyServiceAnnotations(c),
		},
		Spec: corev1.ServiceSpec{
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
				{
					Name:     "metrics",
					Protocol: corev1.ProtocolTCP,
					Port:     3232,
					TargetPort: intstr.IntOrString{
						Type:   1,
						StrVal: "metrics",
					},
				},
			},
			Selector:                 c.KubernetesLabels(),
			Type:                     "ClusterIP",
			PublishNotReadyAddresses: false,
		},
	}

	existingService := &corev1.Service{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingService, targetService); err != nil {
		return ctrl.Result{}, err
	}

	// TRAEFIK MIDDLEWARES

	if err := r.deployTraefikMiddlewares(ctx, req, c); err != nil {
		l.Error(err, "Error deploying traefik middlewares")
		return ctrl.Result{}, err
	}

	// INGRESS

	annotations := map[string]string{}
	traefikMiddlewares := internal.BuildTraefikMiddlewareAnnotation(req.Namespace, r.forwardMiddleware(c))
	annotations[internal.TraefikMiddlewaresKey] = traefikMiddlewares

	// add spec annotations and append traefik middlewares
	for k, v := range c.Spec.IngressAnnotations {
		if k == internal.TraefikMiddlewaresKey {
			traefikMiddlewares = fmt.Sprintf("%s,%s", v, traefikMiddlewares)
			v = traefikMiddlewares
		}
		annotations[k] = v
	}

	targetIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			OwnerReferences: c.OwnerReferencesForChildren(),
			Annotations:     annotations,
		},
		Spec: networkingv1.IngressSpec{
			// IngressClass set below
			// TODO: TLS configuration, perhaps
			TLS: nil,
			Rules: []networkingv1.IngressRule{
				{
					Host: c.Spec.Url,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: c.ComponentName(),
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
		},
	}

	// only define the ingressClassName if it is specified on the site
	if c.Spec.IngressClass != "" {
		targetIngress.Spec.IngressClassName = &c.Spec.IngressClass
	}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, &networkingv1.Ingress{}, targetIngress); err != nil {
		return ctrl.Result{}, err
	}

	// POD DISRUPTION BUDGET
	if err := CreateOrUpdateDisruptionBudget(
		ctx, req, r, c, ptr.To(product.DetermineMinAvailableReplicas(c.Spec.Replicas)), nil,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ConnectReconciler) CleanupConnect(ctx context.Context, req ctrl.Request, c *positcov1beta1.Connect) (ctrl.Result, error) {
	if err := r.cleanupDeployedService(ctx, req, c); err != nil {
		return ctrl.Result{}, err
	}
	if err := internal.CleanupProvisioningKey(ctx, c, r, req); err != nil {
		return ctrl.Result{}, err
	}
	if err := db.CleanupDatabasePasswordSecret(ctx, r, req, c.ComponentName()); err != nil {
		return ctrl.Result{}, err
	}

	if err := db.CleanupDatabase(ctx, r, req, c.ComponentName()); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ConnectReconciler) cleanupDeployedService(ctx context.Context, req ctrl.Request, c *positcov1beta1.Connect) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-service",
		"product", "connect",
	)

	// we reuse this key, because everything is named the same...
	key := client.ObjectKey{
		Name:      c.ComponentName(),
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
