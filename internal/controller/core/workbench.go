package core

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/api/templates"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/db"
	"github.com/rstudio/goex/ptr"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=secrets-store.csi.x-k8s.io,resources=secretproviderclasses,verbs=get;list;watch;create;update;patch;delete

var dbHostRegexp = regexp.MustCompile(`(:\/\/)?(?P<host>[a-zA-Z0-9\.\-]+)(?P<port>:[0-9]+)?`)
var portRegexp = regexp.MustCompile(`[0-9]+`)
var invalidCharacters = regexp.MustCompile("[^a-z0-9]") // do not glob, lest we lose uniqueness

var azureDatabricksRegexp = regexp.MustCompile("azuredatabricks\\.net")

// FetchAndSetClientSecretForAzureDatabricks will check to see whether AzureDatabricks is in use... if it is,
// it will fetch the secret from the secret manager and modify the Spec in-place...
func (r *WorkbenchReconciler) FetchAndSetClientSecretForAzureDatabricks(ctx context.Context, req ctrl.Request, w *positcov1beta1.Workbench) error {
	l := r.GetLogger(ctx)

	if w.Spec.SecretConfig.WorkbenchSecretIniConfig.Databricks != nil {
		for _, v := range w.Spec.SecretConfig.WorkbenchSecretIniConfig.Databricks {
			if azureDatabricksRegexp.MatchString(v.Url) {

				// matched azure url... fetch and set the client secret

				// TODO: ideally this secret would not be read by the operator...
				//   but that means we need a way to "mount" the secret by env var / etc.
				clientSecretName := fmt.Sprintf("dev-client-secret-%s", v.ClientId)
				if cs, err := product.FetchSecret(ctx, r, req, w.Spec.Secret.Type, w.Spec.Secret.VaultName, clientSecretName); err != nil {
					l.Error(err, "error fetching client secret for databricks azure")
					return err
				} else {
					v.ClientSecret = cs
				}
			}
		}
	}
	return nil
}

func (r *WorkbenchReconciler) ReconcileWorkbench(ctx context.Context, req ctrl.Request, w *positcov1beta1.Workbench) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-workbench",
		"product", "workbench",
	)

	// TODO: should do formal spec validation / correction...

	// check for deprecated databricks location (we did not remove this yet for backwards compat and to allow an upgrade path)
	// basically... the operator should only issue this error when the site has not yet reconciled
	if w.Spec.Config.Databricks != nil && len(w.Spec.Config.Databricks) > 0 {
		err := errors.New("the Databricks configuration should be in SecretConfig, not Config")
		l.Error(err, "invalid workbench specification")
		return ctrl.Result{}, err
	}

	// create database
	secretKey := "dev-db-password"
	if err := db.EnsureDatabaseExists(ctx, r, req, w, w.Spec.DatabaseConfig, w.ComponentName(), "", []string{}, w.Spec.Secret, w.Spec.WorkloadSecret, w.Spec.MainDatabaseCredentialSecret, secretKey); err != nil {
		l.Error(err, "error creating database", "database", w.ComponentName())
		return ctrl.Result{}, err
	}

	// NOTE: we do not retain this value locally. Instead, we just reference the key in the Status
	// TODO: we probably do not need to create this... it goes in a provisioning secret intentionally now...?
	if _, err := internal.EnsureWorkbenchSecretKey(ctx, w, r, req, w); err != nil {
		l.Error(err, "error ensuring that provisioning key exists")
		return ctrl.Result{}, err
	} else {
		l.Info("successfully created or retrieved provisioning key value")
	}

	// update the status with the secret reference
	w.Status.KeySecretRef = corev1.SecretReference{
		Name:      w.KeySecretName(),
		Namespace: req.Namespace,
	}
	if err := r.Status().Update(ctx, w); err != nil {
		l.Error(err, "Error updating status")
		return ctrl.Result{}, err
	}

	// define database stuff
	matches := dbHostRegexp.FindStringSubmatch(w.Spec.DatabaseConfig.Host)
	hostIndex := dbHostRegexp.SubexpIndex("host")
	portIndex := dbHostRegexp.SubexpIndex("port")
	justHost := matches[hostIndex]
	justPort := portRegexp.FindString(matches[portIndex])

	dbName := invalidCharacters.ReplaceAllString(w.ComponentName(), "_")
	w.Spec.SecretConfig.Database = &positcov1beta1.WorkbenchDatabaseConfig{
		Provider: positcov1beta1.WorkbenchDatabaseProviderPostgres,
		Database: dbName,
		Port:     justPort,
		Host:     justHost,
		Username: dbName,
		// FYI: Password is set via env var in the CreateSecretVolumeFactory
	}

	// fetch azure secret, if databricks is involved
	if err := r.FetchAndSetClientSecretForAzureDatabricks(ctx, req, w); err != nil {
		l.Error(err, "error fetching client secret for databricks azure. Not fatal")
	}

	// now create the service itself
	res, err := r.ensureDeployedService(ctx, req, w)
	if err != nil {
		l.Error(err, "error deploying service")
		return res, err
	}

	// TODO: should we watch for happy pods?

	// set to ready if it is not set yet...
	if !w.Status.Ready {
		w.Status.Ready = true
		if err := r.Status().Update(ctx, w); err != nil {
			l.Error(err, "Error updating status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

var defaultWorkbenchVolumeSize = resource.MustParse("2Gi")

func (r *WorkbenchReconciler) CspMiddleware(w *positcov1beta1.Workbench) string {
	return fmt.Sprintf("%s-csp", w.ComponentName())
}

func (r *WorkbenchReconciler) ForwardMiddleware(w *positcov1beta1.Workbench) string {
	return fmt.Sprintf("%s-forward", w.ComponentName())
}

func (r *WorkbenchReconciler) HeadersMiddleware(w *positcov1beta1.Workbench) string {
	return fmt.Sprintf("%s-headers", w.ComponentName())
}

func (r *WorkbenchReconciler) deployTraefikMiddlewares(ctx context.Context, req ctrl.Request, w *positcov1beta1.Workbench) error {
	l := r.GetLogger(ctx).WithValues(
		"function", "deployTraefikMiddlewares",
	)

	if err := internal.DeployTraefikForwardMiddleware(ctx, req, r, r.ForwardMiddleware(w), w); err != nil {
		return err
	}

	cspMiddleware := &v1alpha1.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:            r.CspMiddleware(w),
			Namespace:       req.Namespace,
			Labels:          w.KubernetesLabels(),
			OwnerReferences: w.OwnerReferencesForChildren(),
		},
		Spec: v1alpha1.MiddlewareSpec{
			Headers: &dynamic.Headers{
				// allow the product to be iframed within the parent
				// TODO: there is a risk of overriding other CSPs here...
				//  this also gets simpler if we use the same domain...
				ContentSecurityPolicy: fmt.Sprintf("frame-ancestors %s 'self';", w.Spec.ParentUrl),
			},
		},
	}

	cspMiddlewareKey := client.ObjectKey{Name: r.CspMiddleware(w), Namespace: req.Namespace}

	l.Info("CREATING CSP traefik middleware...")
	if err := internal.BasicCreateOrUpdate(ctx, r, l, cspMiddlewareKey, &v1alpha1.Middleware{}, cspMiddleware); err != nil {
		l.Error(err, "Error creating or updating CSP Middleware")
		return err
	}
	l.Info("DONE creating CSP traefik middleware...")

	headersMiddleware := &v1alpha1.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:            r.HeadersMiddleware(w),
			Namespace:       req.Namespace,
			Labels:          w.KubernetesLabels(),
			OwnerReferences: w.OwnerReferencesForChildren(),
		},
		Spec: v1alpha1.MiddlewareSpec{
			Headers: &dynamic.Headers{
				CustomRequestHeaders: map[string]string{
					"X-Rstudio-Request": fmt.Sprintf("https://%s", w.Spec.Url), // setting this prevents Workbench from including port :443 in its redirect URI in OIDC flows
				},
			},
		},
	}

	headersMiddlewareKey := client.ObjectKey{Name: r.HeadersMiddleware(w), Namespace: req.Namespace}

	l.Info("CREATING HEADERS traefik middleware...")
	if err := internal.BasicCreateOrUpdate(ctx, r, l, headersMiddlewareKey, &v1alpha1.Middleware{}, headersMiddleware); err != nil {
		l.Error(err, "Error creating or updating HEADERS Middleware")
		return err
	}
	l.Info("DONE creating Headers traefik middleware...")

	return nil
}

const workbenchConfigShaKey = "workbench.posit.team/configmap-sha"
const workbenchSessionShaKey = "workbench.posit.team/session-sha"
const workbenchSecretShaKey = "workbench.posit.team/secret-sha"
const workbenchTemplateShaKey = "workbench.posit.team/template-sha"

func (r *WorkbenchReconciler) ensureDeployedService(ctx context.Context, req ctrl.Request, w *positcov1beta1.Workbench) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "ensure-service",
		"product", "workbench",
	)
	// this object key is used a good bit...
	key := client.ObjectKey{
		Name:      w.ComponentName(),
		Namespace: req.Namespace,
	}

	// SECRETS
	if w.GetSecretType() == product.SiteSecretAws {
		// deploy SecretProviderClass for app secrets
		spcNameKey := client.ObjectKey{Name: w.SecretProviderClassName(), Namespace: req.Namespace}
		secretName := fmt.Sprintf("%s-secret", w.ComponentName())

		allSecrets := map[string]string{
			"dev.lic":               "dev-license",
			"admin_token":           "dev-admin-token",
			"user_token":            "dev-user-token",
			"dev-db-password":       "dev-db-password",
			"dev-chronicle-api-key": "dev-chronicle-api-key",
		}
		kubernetesSecrets := map[string]map[string]string{
			secretName: {
				"dev-db-password": "dev-db-password",
			},
		}

		// conditional secrets

		// snowflake
		if w.Spec.Snowflake.ClientId != "" && w.Spec.Snowflake.AccountId != "" {
			allSecrets["snowflake-client-secret"] = "snowflake-client-secret"
			kubernetesSecrets[secretName]["snowflake-client-secret"] = "snowflake-client-secret"
		}

		// oauth
		if w.Spec.Auth.Type == positcov1beta1.AuthTypeOidc {
			allSecrets["client-secret"] = "dev-client-secret"
			kubernetesSecrets[fmt.Sprintf("%s-client-secret", w.ComponentName())] = map[string]string{
				"client-secret": "client-secret",
			}
		}

		// dsn
		if w.Spec.DsnSecret != "" {
			allSecrets["odbc.ini"] = w.Spec.DsnSecret
		}

		// also ensure there is a kubernetes secret to back the admin token if chronicle needs it.
		if w.Spec.ChronicleSidecarProductApiKeyEnabled {
			kubernetesSecrets[secretName]["dev-chronicle-api-key"] = "dev-chronicle-api-key"
		}

		if targetSpc, err := product.GetSecretProviderClassForAllSecrets(
			w, w.SecretProviderClassName(),
			req.Namespace, w.Spec.Secret.VaultName,
			allSecrets,
			kubernetesSecrets,
		); err != nil {
			return ctrl.Result{}, err
		} else {
			existingSpc := &secretstorev1.SecretProviderClass{}

			if err := internal.BasicCreateOrUpdate(ctx, r, l, spcNameKey, existingSpc, targetSpc); err != nil {
				l.Error(err, "error provisioning SecretProviderClass for secrets")
				return ctrl.Result{}, err
			}
		}
	}

	// modifications to config based on parameter settings
	configCopy := w.Spec.Config.DeepCopy()
	secretConfigCopy := w.Spec.SecretConfig.DeepCopy()

	// default settings (NOTE: we ignore what the spec puts in here...)
	if configCopy.RServer == nil {
		configCopy.RServer = &positcov1beta1.WorkbenchRServerConfig{}
	}
	configCopy.RServer.MetricsEnabled = 1
	configCopy.RServer.MetricsPort = int(internal.DefaultPortWorkbenchMetrics)

	if w.Spec.OffHostExecution {
		// config changes for off-host execution...
		// TODO: this overwrites whatever is set in the spec... should we converge conflicts?
		configCopy.WorkbenchIniConfig.LauncherKubernetes = &positcov1beta1.WorkbenchLauncherKubernetesConfig{
			KubernetesNamespace: req.Namespace,
			UseTemplating:       1,
			JobExpiryHours:      1,
		}

		// set the server address to a backend path... TODO: use TLS at some point (or make it configurable)
		svcUrl := w.ServiceUrl("http://", req.Namespace)
		if configCopy.WorkbenchSessionIniConfig.WorkbenchNss == nil {
			configCopy.WorkbenchSessionIniConfig.WorkbenchNss = &positcov1beta1.WorkbenchNssConfig{
				ServerAddress: svcUrl,
			}
		} else {
			configCopy.WorkbenchSessionIniConfig.WorkbenchNss.ServerAddress = svcUrl
		}

		configCopy.WorkbenchIniConfig.RServer.LauncherSessionsCallbackAddress = svcUrl
	} else {
		configCopy.WorkbenchIniConfig.RServer.LauncherSessionsCallbackAddress = "http://localhost:" + strconv.Itoa(int(internal.DefaultPortWorkbenchHTTP))
		if configCopy.WorkbenchIniConfig.LauncherLocal == nil {
			configCopy.WorkbenchIniConfig.LauncherLocal = &positcov1beta1.WorkbenchLauncherLocalConfig{
				Unprivileged: 1,
			}
		} else {
			configCopy.WorkbenchIniConfig.LauncherLocal.Unprivileged = 1
		}
	}

	loginData, err := configCopy.GenerateLoginConfigmapData(ctx)
	if err != nil {
		l.Error(err, "Error generating login configmap contents")

		return ctrl.Result{}, err
	}

	if err := internal.BasicCreateOrUpdate(
		ctx,
		r,
		l,
		client.ObjectKey{
			Name:      w.LoginConfigmapName(),
			Namespace: req.Namespace,
		},
		&corev1.ConfigMap{},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            w.LoginConfigmapName(),
				Namespace:       req.Namespace,
				Labels:          w.KubernetesLabels(),
				OwnerReferences: w.OwnerReferencesForChildren(),
			},
			Data: loginData,
		},
	); err != nil {
		l.Error(err, "Error creating or updating login configmap")

		return ctrl.Result{}, err
	}

	// Create or update HTML ConfigMap for login page if HTML content is provided
	authLoginPageHtmlConfigmapKey := client.ObjectKey{
		Name:      w.AuthLoginPageHtmlConfigmapName(),
		Namespace: req.Namespace,
	}

	if w.Spec.AuthLoginPageHtml != "" {
		// Validate HTML size
		if len(w.Spec.AuthLoginPageHtml) > positcov1beta1.MaxLoginPageHtmlSize {
			err := fmt.Errorf("authLoginPageHtml content (%d bytes) exceeds maximum size of %d bytes (64KB)",
				len(w.Spec.AuthLoginPageHtml), positcov1beta1.MaxLoginPageHtmlSize)
			l.Error(err, "HTML content too large")
			return ctrl.Result{}, err
		}

		authLoginPageHtmlData := map[string]string{
			"login.html": w.Spec.AuthLoginPageHtml,
		}

		if err := internal.BasicCreateOrUpdate(
			ctx,
			r,
			l,
			authLoginPageHtmlConfigmapKey,
			&corev1.ConfigMap{},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            w.AuthLoginPageHtmlConfigmapName(),
					Namespace:       req.Namespace,
					Labels:          w.KubernetesLabels(),
					OwnerReferences: w.OwnerReferencesForChildren(),
				},
				Data: authLoginPageHtmlData,
			},
		); err != nil {
			l.Error(err, "Error creating or updating HTML configmap")
			return ctrl.Result{}, err
		}
	} else {
		// Cleanup ConfigMap if HTML content is removed from spec
		existingConfigmap := &corev1.ConfigMap{}
		if err := internal.BasicDelete(ctx, r, l, authLoginPageHtmlConfigmapKey, existingConfigmap); err != nil {
			if !kerrors.IsNotFound(err) {
				l.Error(err, "Error cleaning up HTML configmap")
				return ctrl.Result{}, err
			}
		}
	}

	if w.Spec.Auth.Type == positcov1beta1.AuthTypeOidc {
		// set up configuration for oauth
		configCopy.WorkbenchIniConfig.RServer.AuthOpenid = 1
		configCopy.WorkbenchIniConfig.RServer.AuthOpenidIssuer = w.Spec.Auth.Issuer
		secretConfigCopy.OpenidClientSecret = &positcov1beta1.WorkbenchOpenidClientSecret{
			ClientId: w.Spec.Auth.ClientId,
		}

		if w.Spec.Auth.UsernameClaim != "" {
			configCopy.WorkbenchIniConfig.RServer.AuthOpenidUsernameClaim = w.Spec.Auth.UsernameClaim
		}

		if len(w.Spec.Auth.Scopes) > 0 {
			configCopy.WorkbenchIniConfig.RServer.AuthOpenidScopes = w.Spec.Auth.Scopes
		}

		// fetch client secret directly... until we can configure this directly via an env var
		if clientSecret, err := product.FetchSecret(ctx, r, req, w.Spec.Secret.Type, w.Spec.Secret.VaultName, "dev-client-secret"); err != nil {
			return ctrl.Result{}, err
		} else {
			secretConfigCopy.OpenidClientSecret.ClientSecret = clientSecret
		}
	} else if w.Spec.Auth.Type == positcov1beta1.AuthTypeSaml {
		if w.Spec.Auth.SamlMetadataUrl == "" {
			return ctrl.Result{}, fmt.Errorf("SAML authentication requires a metadata URL to be specified")
		}

		if configCopy.WorkbenchIniConfig.RServer == nil {
			configCopy.WorkbenchIniConfig.RServer = &positcov1beta1.WorkbenchRServerConfig{}
		}

		configCopy.WorkbenchIniConfig.RServer.AuthSaml = 1
		configCopy.WorkbenchIniConfig.RServer.AuthSamlMetadataUrl = w.Spec.Auth.SamlMetadataUrl

		if w.Spec.Auth.UsernameClaim != "" {
			configCopy.WorkbenchIniConfig.RServer.AuthSamlSpAttributeUsername = w.Spec.Auth.UsernameClaim
		}
	}

	// SUPERVISOR CONFIGMAP
	supervisorKey := client.ObjectKey{
		Name:      w.SupervisorConfigmapName(),
		Namespace: req.Namespace,
	}
	if w.Spec.NonRoot {
		// set config changes
		// if these are nil... launcher is not in a functional state
		if configCopy.WorkbenchIniConfig.Launcher != nil && configCopy.WorkbenchIniConfig.Launcher.Server != nil {
			configCopy.WorkbenchIniConfig.Launcher.Server.Unprivileged = 1
			// this config file defaults to being generated in a root-only location
			configCopy.WorkbenchIniConfig.Launcher.Server.SecureCookieKeyFile = "/mnt/secure/rstudio/secure-cookie-key"
			configCopy.WorkbenchIniConfig.RServer.SecureCookieKeyFile = "/mnt/secure/rstudio/secure-cookie-key"
		}
		if err := w.InitializeNonRootSupervisorConfig(ctx, configCopy); err != nil {
			l.Error(err, "Error updating config for supervisor")
			return ctrl.Result{}, err
		}
		// NOTE: volumes are defined in the workbenchVolumeFactory
		if supervisorCm, err := configCopy.GenerateSupervisorConfigmap(ctx); err != nil {
			l.Error(err, "Error generating supervisor configmap contents")
			return ctrl.Result{}, err
		} else {
			targetSupervisorCm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            w.SupervisorConfigmapName(),
					Namespace:       req.Namespace,
					Labels:          w.KubernetesLabels(),
					OwnerReferences: w.OwnerReferencesForChildren(),
				},
				Data: supervisorCm,
			}

			existingSupervisorCm := &corev1.ConfigMap{}

			if err := internal.BasicCreateOrUpdate(ctx, r, l, supervisorKey, existingSupervisorCm, targetSupervisorCm); err != nil {
				l.Error(err, "Error creating or updating supervisor configmap")
				return ctrl.Result{}, err
			}
		}
	} else {
		// make sure that the supervisor configmap does not exist...
		existingSupervisorCm := &corev1.ConfigMap{}
		if err := internal.BasicDelete(ctx, r, l, supervisorKey, existingSupervisorCm); err != nil {
			if kerrors.IsNotFound(err) {
				// all good! it already does not exist
			} else {
				// unknown error... do not exit... just try again later? (i.e. no big deal if we leave this around for a bit)
				l.Error(err, "Error cleaning up supervisor configmap. Will try again next time")
			}
		}
	}

	// CONFIGMAP
	// - must come after SUPERVISOR CONFIGMAP because options may change when nonRoot is set...

	cmSha := ""
	if cm, err := configCopy.GenerateConfigmap(); err != nil {
		l.Error(err, "Error generating configmap contents")
		return ctrl.Result{}, err
	} else {
		targetConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            w.ComponentName(),
				Namespace:       req.Namespace,
				Labels:          w.KubernetesLabels(),
				OwnerReferences: w.OwnerReferencesForChildren(),
			},
			Data: cm,
		}

		if tmpCmSha, err := product.ComputeSha256(cm); err != nil {
			l.Error(err, "Error computing sha256 for configmap")
			return ctrl.Result{}, err
		} else {
			cmSha = tmpCmSha
		}

		existingConfigmap := &corev1.ConfigMap{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingConfigmap, targetConfigmap); err != nil {
			l.Error(err, "Error creating or updating configmap")
			return ctrl.Result{}, err
		}
	}

	// SESSION CONFIGMAP

	sessionCmSha := ""
	if sessionCm, err := configCopy.GenerateSessionConfigmap(); err != nil {
		l.Error(err, "Error generating session configmap contents")
		return ctrl.Result{}, err
	} else {
		targetSessionConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            w.SessionConfigMapName(),
				Namespace:       req.Namespace,
				Labels:          w.KubernetesLabels(),
				OwnerReferences: w.OwnerReferencesForChildren(),
			},
			Data: sessionCm,
		}

		if tmpSessionCmSha, err := product.ComputeSha256(sessionCm); err != nil {
			l.Error(err, "Error computing sha256 for session configmap")
			return ctrl.Result{}, err
		} else {
			sessionCmSha = tmpSessionCmSha
		}

		sessionKey := client.ObjectKey{
			Namespace: req.Namespace,
			Name:      w.SessionConfigMapName(),
		}
		existingSessionConfigmap := &corev1.ConfigMap{}
		if err := internal.BasicCreateOrUpdate(ctx, r, l, sessionKey, existingSessionConfigmap, targetSessionConfigmap); err != nil {
			l.Error(err, "Error creating or updating session configmap")
			return ctrl.Result{}, err
		}
	}

	// SESSION SERVICE ACCOUNT
	saAnnotations := internal.AddIamAnnotation(fmt.Sprintf("%s-ses", w.ShortName()), req.Namespace, w.SiteName(), map[string]string{}, w)
	saName := w.SessionServiceAccountName()
	saKey := client.ObjectKey{Name: saName, Namespace: req.Namespace}
	targetServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            saName,
			Namespace:       req.Namespace,
			Labels:          w.KubernetesLabels(),
			Annotations:     saAnnotations,
			OwnerReferences: w.OwnerReferencesForChildren(),
		},
	}

	existingServiceAccount := &corev1.ServiceAccount{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, saKey, existingServiceAccount, targetServiceAccount); err != nil {
		return ctrl.Result{}, err
	}

	// SECRET CONFIG
	secretSha := ""
	if secretData, err := secretConfigCopy.GenerateSecretData(); err != nil {
		l.Error(err, "Error generating secret config data")
		return ctrl.Result{}, err
	} else {
		secretName := fmt.Sprintf("%s-config", w.ComponentName())
		secretKey := client.ObjectKey{Name: secretName, Namespace: req.Namespace}
		targetSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secretName,
				Namespace:       req.Namespace,
				Labels:          w.KubernetesLabels(),
				OwnerReferences: w.OwnerReferencesForChildren(),
			},
			Immutable:  nil,
			StringData: secretData,
			Type:       "Opaque",
		}

		if tmpSecretSha, err := product.ComputeSha256(secretData); err != nil {
			l.Error(err, "Error computing sha256 for secret config")
			return ctrl.Result{}, err
		} else {
			secretSha = tmpSecretSha
		}

		existingSecret := &corev1.Secret{}
		if err := internal.BasicCreateOrUpdate(ctx, r, l, secretKey, existingSecret, targetSecret); err != nil {
			l.Error(err, "Error creating or updating secret config")
			return ctrl.Result{}, err
		}
	}

	// TEMPLATES

	templateSha := ""
	if w.Spec.OffHostExecution {
		templateKey := client.ObjectKey{
			Name:      w.TemplateConfigMapName(),
			Namespace: req.Namespace,
		}

		targetTemplatesConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            w.TemplateConfigMapName(),
				Namespace:       req.Namespace,
				Labels:          w.KubernetesLabels(),
				OwnerReferences: w.OwnerReferencesForChildren(),
			},
			Data: map[string]string{
				"job.tpl":                            templates.DumpJobTpl(),
				"service.tpl":                        templates.DumpServiceTpl(),
				"rstudio-library-templates-data.tpl": w.SessionConfigTemplateData(l, configCopy),
			},
		}

		if tmpTemplateSha, err := product.ComputeSha256(targetTemplatesConfigmap.Data); err != nil {
			l.Error(err, "Error computing sha256 for template configmap")
			return ctrl.Result{}, err
		} else {
			templateSha = tmpTemplateSha
		}

		existingTemplatesConfigmap := &corev1.ConfigMap{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, templateKey, existingTemplatesConfigmap, targetTemplatesConfigmap); err != nil {
			return ctrl.Result{}, err
		}
	}

	// SERVICE ACCOUNT & RBAC ...

	// TODO: there may be a time that you want to just generate a service account, even if Workbench is on-host execution...
	if w.Spec.OffHostExecution {
		if err := internal.GenerateRbac(ctx, r, req, w); err != nil {
			l.Error(err, "Error generating service account and rbac")
			return ctrl.Result{}, err
		}
	}

	// VOLUME CLAIM

	// TODO: note that if you change Volume.Create to false, we will just ignore it forever...
	if w.Spec.Volume != nil && w.Spec.Volume.Create {
		if pvc, err := product.DefinePvc(w, req, w.ComponentName(), w.Spec.Volume, defaultWorkbenchVolumeSize); err != nil {
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

	// SHARED STORAGE VOLUME CLAIM (for load balancing)
	// Create a shared storage PVC when load balancing is enabled
	if w.Spec.Config.RServer != nil && w.Spec.Config.RServer.LoadBalancingEnabled == 1 {
		sharedStoragePVCName := fmt.Sprintf("%s-shared-storage", w.ComponentName())
		// The site controller creates PVs with different storage class names:
		// - For NFS: {pv-name}-nfs
		// - For FSX: {pv-name} (no suffix)
		// Since NFS is more common and the PV is already created with -nfs suffix,
		// we'll use that format
		sharedStorageStorageClassName := fmt.Sprintf("%s-nfs", sharedStoragePVCName)
		sharedStorageVolumeSpec := &product.VolumeSpec{
			Create: true,
			Size:   "10Gi",
			AccessModes: []string{
				"ReadWriteMany", // RWX mode for shared storage
			},
			StorageClassName: sharedStorageStorageClassName,
			VolumeName:       sharedStoragePVCName, // This binds to the PV with the same name
		}

		if pvc, err := product.DefinePvc(w, req, sharedStoragePVCName, sharedStorageVolumeSpec, resource.MustParse("10Gi")); err != nil {
			l.Error(err, "error defining shared storage PVC", "pvc", sharedStoragePVCName)
			return ctrl.Result{}, err
		} else {
			localKey := client.ObjectKey{Name: pvc.Name, Namespace: req.Namespace}
			if err := internal.PvcCreateOrUpdate(ctx, r, l, localKey, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
				return ctrl.Result{}, err
			} else {
				l.Info("successfully created or updated shared storage PVC", "pvc", pvc.Name)
			}
		}
	}

	// create the volumes that will be referenced by the VolumeFactory later...
	for _, v := range w.Spec.AdditionalVolumes {
		if v.Create {
			// create the PVC
			if pvc, err := product.DefinePvc(w, req, v.PvcName, &v, resource.MustParse("10Gi")); err != nil {
				l.Error(err, "error defining PVC", "pvc", pvc.Name)
				return ctrl.Result{}, err
			} else {
				localKey := client.ObjectKey{Name: pvc.Name, Namespace: req.Namespace}
				if err := internal.PvcCreateOrUpdate(ctx, r, l, localKey, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
					return ctrl.Result{}, err
				} else {
					l.Info("successfully created or updated PVC", "pvc", pvc.Name)
				}
			}
		}
	}

	// DEPLOYMENT

	var pullSecrets []corev1.LocalObjectReference
	for _, s := range w.Spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: s})
	}

	maybeServiceAccountName := ""
	if w.Spec.OffHostExecution {
		maybeServiceAccountName = w.ComponentName()
	}

	workbenchVolumeFactory := w.CreateVolumeFactory(configCopy)
	workbenchSecretVolumeFactory := w.CreateSecretVolumeFactory()

	var chronicleSeededEnv []corev1.EnvVar
	if w.Spec.ChronicleSidecarProductApiKeyEnabled {
		chronicleSeededEnv = []corev1.EnvVar{
			{Name: "CHRONICLE_WORKBENCH_APIKEY", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-secret", w.ComponentName())},
					Key:                  "dev-chronicle-api-key",
					Optional:             ptr.To(true),
				},
			}},
		}
	}

	chronicleFactory := product.CreateChronicleWorkbenchVolumeFactory(w, chronicleSeededEnv)

	targetDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            w.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          w.KubernetesLabels(),
			OwnerReferences: w.OwnerReferencesForChildren(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(product.PassDefaultReplicas(w.Spec.Replicas, 1))),
			Selector: &metav1.LabelSelector{
				MatchLabels: w.SelectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: w.KubernetesLabels(),
					Annotations: map[string]string{
						// TODO: this is a hack to get config changes to trigger a new deployment (for now)
						//   In the future, we could use our own mechanism and decide whether to restart or SIGHUP the service...
						workbenchConfigShaKey:   cmSha,
						workbenchSessionShaKey:  sessionCmSha,
						workbenchSecretShaKey:   secretSha,
						workbenchTemplateShaKey: templateSha,
					},
				},
				Spec: corev1.PodSpec{
					EnableServiceLinks:           ptr.To(false),
					NodeSelector:                 w.Spec.NodeSelector,
					ImagePullSecrets:             pullSecrets,
					ServiceAccountName:           maybeServiceAccountName,
					AutomountServiceAccountToken: ptr.To(true),
					Containers: product.ConcatLists(
						[]corev1.Container{
							{
								Name:            "workbench",
								Image:           w.Spec.Image,
								ImagePullPolicy: w.Spec.ImagePullPolicy,
								Env: product.ConcatLists(
									workbenchVolumeFactory.EnvVars(),
									workbenchSecretVolumeFactory.EnvVars(),
									chronicleFactory.EnvVars(),
									product.StringMapToEnvVars(w.Spec.AddEnv),
									[]corev1.EnvVar{
										{
											Name:  "LAUNCHER_INSTANCE_ID",
											Value: w.ComponentName(),
										},
									},
								),
								Command: []string{"supervisord"},
								Args:    []string{},
								Ports: []corev1.ContainerPort{
									internal.DefaultPortWorkbenchHTTP.ContainerPort("http"),
									internal.DefaultPortWorkbenchMetrics.ContainerPort("metrics"),
								},
								SecurityContext: &corev1.SecurityContext{
									//RunAsUser:                ptr.To(int64(0)),
									RunAsNonRoot:             ptr.To(false),
									AllowPrivilegeEscalation: ptr.To(true),
									Capabilities:             &corev1.Capabilities{
										// Drop: []corev1.Capability{"ALL"},
									},
									//SeccompProfile: &corev1.SeccompProfile{
									// Type: "RuntimeDefault",
									//},
								},
								VolumeMounts: product.ConcatLists(
									workbenchVolumeFactory.VolumeMounts(),
									workbenchSecretVolumeFactory.VolumeMounts(),
									chronicleFactory.VolumeMounts(),
								),
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										// TODO: resources for Workbench
										//"cpu":               resource.Quantity{Format: "2000m"},
										//"memory":            resource.Quantity{Format: "3Gi"},
										//"ephemeral-storage": resource.Quantity{Format: "100Mi"},
									},
									Limits: corev1.ResourceList{
										//"cpu":               resource.Quantity{Format: "6000m"},
										//"memory":            resource.Quantity{Format: "8Gi"},
										//"ephemeral-storage": resource.Quantity{Format: "200Mi"},
									},
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/health-check",
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
						chronicleFactory.Sidecars(),
					),
					Affinity: &corev1.Affinity{
						PodAntiAffinity: positcov1beta1.ComponentSpecPodAntiAffinity(w, req.Namespace),
					},
					SecurityContext: &corev1.PodSecurityContext{
						//FSGroup: ptr.To(int64(999)),
					},
					Tolerations: w.Spec.Tolerations,
					Volumes: product.ConcatLists(
						workbenchVolumeFactory.Volumes(),
						workbenchSecretVolumeFactory.Volumes(),
						chronicleFactory.Volumes(),
					),
				},
			},
		},
	}

	if w.Spec.Sleep {
		targetDeployment.Spec.Template.Spec.Containers[0].Command = []string{"sleep"}
		targetDeployment.Spec.Template.Spec.Containers[0].Args = []string{"infinity"}
	}

	existingDeployment := &appsv1.Deployment{}

	// TODO: deployment will _definitely_ need custom CreateOrUpdate work at some point
	//   i.e. to handle version upgrades, etc. We could add an Updater() callback, or a
	//   CustomComparator... or just decide to inline the logic
	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingDeployment, targetDeployment); err != nil {
		return ctrl.Result{}, err
	}

	// SERVICE

	targetService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            w.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          w.KubernetesLabels(),
			OwnerReferences: w.OwnerReferencesForChildren(),
			Annotations:     internal.TraefikStickyServiceAnnotations(w),
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
			},
			Selector:                 w.KubernetesLabels(),
			Type:                     "ClusterIP",
			PublishNotReadyAddresses: false,
		},
	}

	existingService := &corev1.Service{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingService, targetService); err != nil {
		return ctrl.Result{}, err
	}

	// TRAEFIK MIDDLEWARES

	if err := r.deployTraefikMiddlewares(ctx, req, w); err != nil {
		l.Error(err, "Error deploying traefik middlewares")
		return ctrl.Result{}, err
	}

	// INGRESS
	annotations := map[string]string{}
	traefikMiddlewares := internal.BuildTraefikMiddlewareAnnotation(
		req.Namespace,
		r.ForwardMiddleware(w),
		r.CspMiddleware(w),
		r.HeadersMiddleware(w),
	)
	annotations[internal.TraefikMiddlewaresKey] = traefikMiddlewares

	// add spec annotations and append traefik middlewares
	for k, v := range w.Spec.IngressAnnotations {
		if k == internal.TraefikMiddlewaresKey {
			traefikMiddlewares = fmt.Sprintf("%s,%s", v, traefikMiddlewares)
			v = traefikMiddlewares
		}
		annotations[k] = v
	}

	targetIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            w.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          w.KubernetesLabels(),
			OwnerReferences: w.OwnerReferencesForChildren(),
			Annotations:     annotations,
		},
		Spec: networkingv1.IngressSpec{
			// IngressClass set below
			// TODO: TLS configuration, perhaps
			TLS: nil,
			Rules: []networkingv1.IngressRule{
				{
					Host: w.Spec.Url,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: w.ComponentName(),
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
	if w.Spec.IngressClass != "" {
		targetIngress.Spec.IngressClassName = &w.Spec.IngressClass
	}

	existingIngress := &networkingv1.Ingress{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingIngress, targetIngress); err != nil {
		return ctrl.Result{}, err
	}

	// POD DISRUPTION BUDGET
	if err := CreateOrUpdateDisruptionBudget(
		ctx, req, r, w, ptr.To(product.DetermineMinAvailableReplicas(w.Spec.Replicas)), nil,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WorkbenchReconciler) CleanupWorkbench(ctx context.Context, req ctrl.Request, w *positcov1beta1.Workbench) (ctrl.Result, error) {
	if err := r.cleanupDeployedService(ctx, req, w); err != nil {
		return ctrl.Result{}, err
	}
	if err := db.CleanupDatabasePasswordSecret(ctx, r, req, w.ComponentName()); err != nil {
		return ctrl.Result{}, err
	}
	if err := db.CleanupDatabase(ctx, r, req, w.ComponentName()); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *WorkbenchReconciler) cleanupDeployedService(ctx context.Context, req ctrl.Request, w *positcov1beta1.Workbench) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-service",
		"product", "connect",
	)

	l.Info("starting")

	return nil
}
