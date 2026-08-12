package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	localtest "github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/rstudio/goex/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
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

func TestConnectReconciler_OIDC_EnableRegisterOnFirstLogin(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-oidc-enable-reg"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.RegisterOnFirstLogin = ptr.To(true)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:     positcov1beta1.AuthTypeOidc,
		ClientId: "test-client",
		Issuer:   "https://idp.example.com",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	t.Logf("Generated config:\n%s", config)

	assert.Contains(t, config, "[OAuth2]", "OAuth2 section should exist")
	assert.Contains(t, config, "RegisterOnFirstLogin = true", "RegisterOnFirstLogin should be explicitly enabled")
}

func TestConnectReconciler_OIDC_DefaultRegisterOnFirstLogin(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-oidc-default-reg"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:     positcov1beta1.AuthTypeOidc,
		ClientId: "test-client",
		Issuer:   "https://idp.example.com",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")
	t.Logf("Generated config:\n%s", config)

	assert.Contains(t, config, "[OAuth2]", "OAuth2 section should exist")
	assert.NotContains(t, config, "RegisterOnFirstLogin", "RegisterOnFirstLogin should not be written to config when not set, allowing Connect to use its default")
}

func TestConnectReconciler_RegisterOnFirstLogin_IgnoredWithNoAuth(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-reg-no-auth"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.RegisterOnFirstLogin = ptr.To(true)
	// Auth.Type left empty (no auth configured)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	c = getConnect(t, cli, ns, name)

	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")

	assert.NotContains(t, config, "[OAuth2]", "OAuth2 section should not exist when auth type is not oidc")
	assert.NotContains(t, config, "RegisterOnFirstLogin", "RegisterOnFirstLogin should not appear in config when auth type is not oidc")
}

func TestConnectReconciler_RegisterOnFirstLogin_IgnoredWithSAML(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-reg-saml"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.RegisterOnFirstLogin = ptr.To(true)
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

	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	config, exists := configmap.Data["rstudio-connect.gcfg"]
	require.True(t, exists, "rstudio-connect.gcfg should exist in the ConfigMap")

	assert.NotContains(t, config, "[OAuth2]", "OAuth2 section should not exist when auth type is saml")
	assert.NotContains(t, config, "RegisterOnFirstLogin", "RegisterOnFirstLogin should not appear in config when auth type is saml")
	assert.Contains(t, config, "[SAML]", "SAML section should still be configured")
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

// TestConnectReconciler_EnvVars verifies that the envVars field, including a
// valueFrom.secretKeyRef entry, flows through to the rendered Connect
// container's Env.
func TestConnectReconciler_EnvVars(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-env-vars"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	plainEnv := corev1.EnvVar{
		Name:  "CONNECT_ENVVARS_TEST",
		Value: "plain-value",
	}
	secretEnv := corev1.EnvVar{
		Name: "CONNECT_ENVVARS_TEST_FROM_SECRET",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
				Key:                  "api-key",
			},
		},
	}

	c := defineDefaultConnect(t, ns, name)
	c.Spec.EnvVars = []corev1.EnvVar{plainEnv, secretEnv}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, c.ComponentName())
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Contains(t, container.Env, plainEnv, "plain envVar should be rendered into the container Env")
	assert.Contains(t, container.Env, secretEnv, "secretKeyRef envVar should be rendered into the container Env")
}

// resourceNames returns the keys of a ResourceList, for asserting that a
// resource map contains exactly the expected entries.
func resourceNames(rl corev1.ResourceList) []corev1.ResourceName {
	names := make([]corev1.ResourceName, 0, len(rl))
	for name := range rl {
		names = append(names, name)
	}
	return names
}

// assertQuantityEqual compares a resource.Quantity by value (Cmp) rather than
// by its string form, since the API round-trip canonicalizes quantity strings
// (e.g. "2000m" -> "2"), which would break assert.Equal on the raw struct if a
// non-canonical override literal were used.
func assertQuantityEqual(t *testing.T, expected string, actual resource.Quantity) {
	w := resource.MustParse(expected)
	assert.Zerof(t, w.Cmp(actual), "expected %s, got %s", expected, actual.String())
}

// TestConnectReconciler_DefaultResources verifies the server container gets the
// default resource requests (and no limits) when Spec.Resources is unset.
func TestConnectReconciler_DefaultResources(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-default-resources"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, c.ComponentName())
	resources := deployment.Spec.Template.Spec.Containers[0].Resources

	assert.Equal(t, resource.MustParse("100m"), resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("1Gi"), resources.Requests[corev1.ResourceMemory])
	assert.Empty(t, resources.Limits, "default Connect resources should not set limits")
}

// TestConnectReconciler_ResourcesOverride verifies an explicit Spec.Resources
// override replaces the defaults entirely.
func TestConnectReconciler_ResourcesOverride(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-resources-override"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	override := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
	}

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Resources = &override

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, c.ComponentName())
	resources := deployment.Spec.Template.Spec.Containers[0].Resources

	// The override fully replaces the defaults: requests match the override and
	// limits are set (Connect's default has no limits).
	assert.ElementsMatch(t, []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory}, resourceNames(resources.Requests))
	assertQuantityEqual(t, "500m", resources.Requests[corev1.ResourceCPU])
	assertQuantityEqual(t, "4Gi", resources.Requests[corev1.ResourceMemory])

	assert.ElementsMatch(t, []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory}, resourceNames(resources.Limits))
	assertQuantityEqual(t, "2", resources.Limits[corev1.ResourceCPU])
	assertQuantityEqual(t, "8Gi", resources.Limits[corev1.ResourceMemory])
}

// TestConnectReconciler_DefaultCommand verifies that when Spec.Command/Spec.Args
// are unset, the rendered connect container has no Command/Args override, so the
// image's own ENTRYPOINT/CMD is used.
func TestConnectReconciler_DefaultCommand(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-default-command"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, c.ComponentName())
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Empty(t, container.Command, "Command should not be set by default")
	assert.Empty(t, container.Args, "Args should not be set by default")
}

// TestConnectReconciler_CommandOverride verifies that explicit Spec.Command/Spec.Args
// flow through to the rendered connect container exactly.
func TestConnectReconciler_CommandOverride(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-command-override"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Command = []string{"tini", "--"}
	c.Spec.Args = []string{"/usr/local/bin/startup.sh"}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, c.ComponentName())
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, []string{"tini", "--"}, container.Command)
	assert.Equal(t, []string{"/usr/local/bin/startup.sh"}, container.Args)
}

// TestConnectReconciler_SleepOverridesCommand verifies that Spec.Sleep takes
// precedence over an explicit Spec.Command/Spec.Args, putting the container to
// sleep instead of running the user-specified command.
func TestConnectReconciler_SleepOverridesCommand(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-sleep-overrides-command"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Command = []string{"tini", "--"}
	c.Spec.Args = []string{"/usr/local/bin/startup.sh"}
	c.Spec.Sleep = true

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, c.ComponentName())
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, []string{"sleep"}, container.Command)
	assert.Equal(t, []string{"infinity"}, container.Args)
}

// TestConnectReconciler_Suspended verifies that when Connect has Suspended=true,
// ReconcileConnect does not create serving resources (Deployment, Service, Ingress).
func TestConnectReconciler_Suspended(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-suspended"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	suspended := true
	c.Spec.Suspended = &suspended

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// No Deployment should be created when suspended
	dep := &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: c.ComponentName(), Namespace: ns}, dep)
	assert.Error(t, err, "Deployment should not exist when Connect is suspended")

	// Status should reflect the suspended state
	updated := &positcov1beta1.Connect{}
	require.NoError(t, cli.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, updated))
	assert.False(t, updated.Status.Ready, "Ready bool should be false when suspended")
	readyCond := apimeta.FindStatusCondition(updated.Status.Conditions, status.TypeReady)
	require.NotNil(t, readyCond, "Ready condition should be set when suspended")
	assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
	assert.Equal(t, status.ReasonSuspended, readyCond.Reason)
	progressCond := apimeta.FindStatusCondition(updated.Status.Conditions, status.TypeProgressing)
	require.NotNil(t, progressCond, "Progressing condition should be set when suspended")
	assert.Equal(t, metav1.ConditionFalse, progressCond.Status)
	assert.Equal(t, status.ReasonSuspended, progressCond.Reason)
}
