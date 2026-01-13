package core

import (
	"context"
	"net/url"
	"strconv"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/keycloak/v2alpha1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/db"
	"github.com/rstudio/goex/ptr"
	v14 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v13 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func (r *SiteReconciler) reconcileKeycloak(ctx context.Context, req controllerruntime.Request, site *v1beta1.Site, dbUrl *url.URL, sslMode string) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-keycloak",
	)

	existingKeycloak := &v2alpha1.Keycloak{}
	localKeycloak := &v1beta1.Keycloak{
		Site: site,
	}
	keycloakKey := client.ObjectKey{Name: localKeycloak.ComponentName(), Namespace: req.Namespace}
	if site.Spec.Keycloak.Enabled {
		keycloakDomain := prefixDomain("key", site.Spec.Domain, v1beta1.SiteSubDomain)

		// ensure database...
		secretKey := "keycloak-db-password"
		if err := db.EnsureDatabaseExists(ctx, r, req, site,
			v1beta1.PostgresDatabaseConfig{
				Host:           dbUrl.Hostname(),
				DropOnTeardown: site.Spec.DropDatabaseOnTeardown,
				SslMode:        sslMode,
			}, localKeycloak.ComponentName(), "",
			[]string{"keycloak"}, site.Spec.Secret, site.Spec.WorkloadSecret, site.Spec.MainDatabaseCredentialSecret,
			secretKey); err != nil {
			l.Error(err, "error creating database", "database", localKeycloak.DatabaseName())
			return err
		}

		// ensure service account...
		annotations := internal.AddIamAnnotation("keycloak", req.Namespace, "", map[string]string{}, localKeycloak)

		targetServiceAccount := &v1.ServiceAccount{
			ObjectMeta: v12.ObjectMeta{
				Name:            localKeycloak.ComponentName(),
				Namespace:       req.Namespace,
				Labels:          site.KubernetesLabels(),
				Annotations:     annotations,
				OwnerReferences: site.OwnerReferencesForChildren(),
			},
			// TODO: we should specify secrets here for "minimal access"
			Secrets:                      nil,
			ImagePullSecrets:             nil,
			AutomountServiceAccountToken: ptr.To(true),
		}

		existingServiceAccount := &v1.ServiceAccount{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, keycloakKey, existingServiceAccount, targetServiceAccount); err != nil {
			return err
		}

		// potentially create secret provider class
		if site.GetSecretType() == product.SiteSecretAws {

			if targetKeycloakSpc, err := localKeycloak.SecretProviderClass(req); err != nil {
				l.Error(err, "Error preparing keycloak secret provider class")
				return err
			} else {
				existingKeycloakSpc := &v13.SecretProviderClass{}

				if err := internal.BasicCreateOrUpdate(ctx, r, l, client.ObjectKey{Name: targetKeycloakSpc.Name, Namespace: req.Namespace}, existingKeycloakSpc, targetKeycloakSpc); err != nil {
					l.Error(err, "error creating or updating keycloak secret provider class")
					return err
				}
			}

			// create a dummy secret provider class consumer to provision the secret
			// (because Keycloak uses a CRD, we do not get arbitrary volume support)
			if targetKeycloakSpcConsumer, err := localKeycloak.SpcConsumerDeployment(req); err != nil {
				l.Error(err, "Error preparing keycloak secret provider class consumer deployment")
				return err
			} else {
				existingKeycloakSpcConsumer := &v14.Deployment{}

				if err := internal.BasicCreateOrUpdate(ctx, r, l, client.ObjectKey{Name: targetKeycloakSpcConsumer.Name, Namespace: req.Namespace}, existingKeycloakSpcConsumer, targetKeycloakSpcConsumer); err != nil {
					l.Error(err, "error creating or updating keycloak secret provider class consumer deployment")
					return err
				}
			}
		}

		// deploy keycloak middleware
		if err := internal.DeployTraefikForwardMiddlewareWithHost(
			ctx, req, r,
			localKeycloak.MiddlewareForwardName(),
			site,
			keycloakDomain,
		); err != nil {
			l.Error(err, "error deploying keycloak middlewares")
			return err
		}

		// deploy keycloak instance by using operator
		dbPort, err := strconv.Atoi(dbUrl.Port())
		if err != nil {
			l.Error(err, "error parsing port from dbUrl. Using default of 5432")
			dbPort = 5432
		}
		keycloakSpec := v2alpha1.KeycloakSpec{
			Db: &v2alpha1.KeycloakDbSpec{
				Vendor: "postgres",
				UsernameSecret: &v2alpha1.KeycloakSecretSpec{
					Name: localKeycloak.SecretName(),
					Key:  "keycloak-db-user",
				},
				PasswordSecret: &v2alpha1.KeycloakSecretSpec{
					Name: localKeycloak.SecretName(),
					Key:  "keycloak-db-password",
				},
				Host:            dbUrl.Hostname(),
				Database:        localKeycloak.DatabaseName(),
				Port:            dbPort,
				Schema:          "keycloak",
				PoolInitialSize: 1,
				PoolMinSize:     2,
				PoolMaxSize:     5,
			},
			Http: &v2alpha1.KeycloakHttpSpec{
				HttpEnabled: true,
				HttpPort:    8080,
				HttpsPort:   8443,
			},
			Hostname: &v2alpha1.KeycloakHostnameSpec{
				Hostname: keycloakDomain,
				//Admin:             prefixDomain("key", site.Spec.Domain, v1beta1.SiteSubDomain),
				Strict:            true,
				StrictBackchannel: true,
			},
			Instances: 1,
			Ingress: &v2alpha1.KeycloakIngressSpec{
				Enabled: true,
				Annotations: map[string]string{
					internal.TraefikMiddlewaresKey: internal.BuildTraefikMiddlewareAnnotation(
						req.Namespace,
						localKeycloak.MiddlewareForwardName(),
					),
				},
			},
			Features: &v2alpha1.KeycloakFeaturesSpec{
				Enabled: []string{
					// some of these are commented out... deprecated or experimental... (based on log entries)
					"account-api",
					"account",
					"admin-api",
					"admin-fine-grained-authz",
					"admin",
					"authorization",
					"ciba",
					"client-policies",
					"client-secret-rotation",
					"client-types",
					"declarative-ui",
					"device-flow",
					"docker",
					"dpop",
					"dynamic-scopes",
					//"fips",
					//"impersonation",
					//"js-adapter",
					//"kerberos",
					//"linkedin-oauth",
					"login",
					"oid4vc-vci",
					"par",
					"passkeys",
					"persistent-user-sessions",
					"preview",
					"recovery-codes",
					"scripts",
					"step-up-authentication",
					"token-exchange",
					//"transient-users",
					"update-email",
					"web-authn",
				},
				Disabled: []string{},
			},
			Transaction: &v2alpha1.KeycloakTransactionSpec{
				XaEnabled: true,
			},
			Unsupported: &v2alpha1.KeycloakUnsupportedSpec{
				PodTemplate: &v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						ServiceAccountName: localKeycloak.ComponentName(),
						Containers: []v1.Container{
							{
								Env: []v1.EnvVar{
									{Name: "KC_PROXY", Value: "edge"},
									{Name: "KC_PROXY_HEADERS", Value: "xforwarded"},
								},
							},
						},
					},
				},
			},
		}

		// Set custom image if specified
		if site.Spec.Keycloak.Image != "" {
			keycloakSpec.Image = site.Spec.Keycloak.Image
		}

		targetKeycloak := &v2alpha1.Keycloak{
			ObjectMeta: v12.ObjectMeta{
				Name:            localKeycloak.ComponentName(),
				Namespace:       req.Namespace,
				OwnerReferences: site.OwnerReferencesForChildren(),
				Labels:          site.KubernetesLabels(),
			},
			Spec: keycloakSpec,
		}
		// then deploy it...

		if err := internal.BasicCreateOrUpdate(ctx, r, l, keycloakKey, existingKeycloak, targetKeycloak); err != nil {
			l.Error(err, "error converging keycloak")
			return err
		}
	} else {

		// delete the object if it exists
		if err := internal.BasicDelete(ctx, r, l, keycloakKey, existingKeycloak); err != nil && errors.IsNotFound(err) {
			l.Info("Keycloak instance not found and not requested")
		} else if err != nil {
			l.Error(err, "error deleting keycloak instance")
			// do not exit... because we will try again later...
		}
	}
	return nil
}
