package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	localtest "github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func initConnectReconciler(t *testing.T, ctx context.Context, namespace, name string) (context.Context, *ConnectReconciler, ctrl.Request, client.Client) {
	localEnv := localtest.LocalTestEnv{}
	cli, cliScheme, log, err := localEnv.Start(loadSchemes)
	require.NoError(t, err)
	r := &ConnectReconciler{
		Client: cli,
		Scheme: cliScheme,
		Log:    log,
	}

	ctx2 := logr.NewContext(ctx, log)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}

	return ctx2, r, req, cli
}

func defineDefaultConnect(t *testing.T, ns, name string) *positcov1beta1.Connect {
	err := product.GlobalTestSecretProvider.SetSecret("dev-db-password", "dev-password")
	require.NoError(t, err)

	return &positcov1beta1.Connect{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Connect",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			UID:       "config-example-uid",
			Labels: map[string]string{
				positcov1beta1.ManagedByLabelKey: positcov1beta1.ManagedByLabelValue,
			},
		},
		Spec: positcov1beta1.ConnectSpec{
			Secret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			WorkloadSecret: positcov1beta1.SecretConfig{
				VaultName: "workload-vault",
				Type:      product.SiteSecretTest,
			},
			MainDatabaseCredentialSecret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			DatabaseConfig: positcov1beta1.PostgresDatabaseConfig{
				Host:           "localhost",
				DropOnTeardown: true,
				SslMode:        "",
			},
			Image: "some-image:latest",
		},
	}
}
func TestConnectReconciler_SAML(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-saml"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:            positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl: "https://idp.example.com/saml/metadata",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check SAML configuration in rserver.conf
	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	assert.Contains(t, config, "[Authentication]\nProvider = saml", "SAML auth should be enabled")
	assert.Contains(t, config, "[SAML]\nIdPMetaDataURL = https://idp.example.com/saml/metadata\nIdPAttributeProfile = default\n", "SAML section should be configured")
}

func TestConnectReconciler_SAML_WithIdPAttributeProfile(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-saml-profile"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:                    positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl:         "https://idp.example.com/saml/metadata",
		SamlIdPAttributeProfile: "custom-profile",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check SAML configuration in rserver.conf
	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	assert.Contains(t, config, "[Authentication]\nProvider = saml", "SAML auth should be enabled")
	assert.Contains(t, config, "[SAML]\nIdPMetaDataURL = https://idp.example.com/saml/metadata\nIdPAttributeProfile = custom-profile\n", "SAML section should have custom profile")
}

func TestConnectReconciler_SAML_WithIndividualAttributes(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-saml-attrs"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:                   positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl:        "https://idp.example.com/saml/metadata",
		SamlUsernameAttribute:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn",
		SamlFirstNameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
		SamlLastNameAttribute:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
		SamlEmailAttribute:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check SAML configuration in rserver.conf
	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	assert.Contains(t, config, "[Authentication]\nProvider = saml", "SAML auth should be enabled")
	assert.Contains(t, config, "IdPMetaDataURL = https://idp.example.com/saml/metadata", "SAML metadata URL should be configured")
	assert.Contains(t, config, "UsernameAttribute = http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn", "SAML username attribute should be configured")
	assert.Contains(t, config, "FirstNameAttribute = http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname", "SAML first name attribute should be configured")
	assert.Contains(t, config, "LastNameAttribute = http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname", "SAML last name attribute should be configured")
	assert.Contains(t, config, "EmailAttribute = http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", "SAML email attribute should be configured")
	assert.NotContains(t, config, "IdPAttributeProfile", "IdPAttributeProfile should not be present when individual attributes are set")
}

func TestConnectReconciler_SAML_PartialIndividualAttributes(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-saml-partial"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:                  positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl:       "https://idp.example.com/saml/metadata",
		SamlUsernameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn",
		SamlEmailAttribute:    "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		// Only setting username and email, not first/last name
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check SAML configuration in rserver.conf
	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	assert.Contains(t, config, "UsernameAttribute = http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn", "SAML username attribute should be configured")
	assert.Contains(t, config, "EmailAttribute = http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", "SAML email attribute should be configured")
	assert.NotContains(t, config, "FirstNameAttribute", "FirstNameAttribute should not be present when not set")
	assert.NotContains(t, config, "LastNameAttribute", "LastNameAttribute should not be present when not set")
	assert.NotContains(t, config, "IdPAttributeProfile", "IdPAttributeProfile should not be present when individual attributes are set")
}

func TestConnectReconciler_SAML_ValidationError_MutualExclusivity(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-saml-invalid"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:                    positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl:         "https://idp.example.com/saml/metadata",
		SamlIdPAttributeProfile: "custom-profile",
		SamlUsernameAttribute:   "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn", // This should cause an error
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	_, err = r.ReconcileConnect(ctx, req, c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SAML IdPAttributeProfile cannot be specified together with individual SAML attribute mappings")
}

func TestConnectReconciler_DefaultDatabaseSchemas(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-default-schemas"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check database configuration in config
	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")

	// Verify the Postgres URLs contain the default schema names
	assert.Contains(t, config, "URL = postgres://connect_default_schemas_connect@localhost/connect_default_schemas_connect?options=-csearch_path=connect", "Default connect schema should be used")
	assert.Contains(t, config, "InstrumentationURL = postgres://connect_default_schemas_connect@localhost/connect_default_schemas_connect?options=-csearch_path=instrumentation", "Default instrumentation schema should be used")
}

func TestConnectReconciler_CustomDatabaseSchemas(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-custom-schemas"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	// Set custom schema names
	c.Spec.DatabaseConfig.Schema = "custom_schema"
	c.Spec.DatabaseConfig.InstrumentationSchema = "custom_metrics"

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check database configuration in config
	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")

	// Verify the Postgres URLs contain the custom schema names
	assert.Contains(t, config, "URL = postgres://connect_custom_schemas_connect@localhost/connect_custom_schemas_connect?options=-csearch_path=custom_schema", "Custom schema should be used")
	assert.Contains(t, config, "InstrumentationURL = postgres://connect_custom_schemas_connect@localhost/connect_custom_schemas_connect?options=-csearch_path=custom_metrics", "Custom instrumentation schema should be used")
}

func TestConnectReconciler_OIDC_DisableGroupsClaim(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-oidc-no-groups"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:               positcov1beta1.AuthTypeOidc,
		ClientId:           "test-client",
		Issuer:             "https://idp.example.com",
		Groups:             true, // Enable groups auto-provision
		DisableGroupsClaim: true, // But explicitly disable the groups claim
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	t.Logf("Generated config:\n%s", config)

	// Verify OAuth2 config includes GroupsAutoProvision but has empty GroupsClaim
	assert.Contains(t, config, "[OAuth2]", "OAuth2 section should exist")
	assert.Contains(t, config, "GroupsAutoProvision = true", "GroupsAutoProvision should be enabled")
	assert.Contains(t, config, "GroupsClaim = ", "GroupsClaim should be explicitly set to empty")
	// Ensure it's not set to a non-empty value
	assert.NotContains(t, config, "GroupsClaim = groups", "GroupsClaim should not have the default 'groups' value")
}
