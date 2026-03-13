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

// TestConnectReconciler_PPMAuth verifies that when AuthenticatedRepos is enabled,
// the Connect Deployment includes PPM auth init container, sidecar, volumes, volume mounts, and env vars.
func TestConnectReconciler_PPMAuth(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-ppm-auth"

	ctx, r, req, cli := initConnectReconciler(t, ctx, ns, name)

	c := defineDefaultConnect(t, ns, name)
	c.Spec.AuthenticatedRepos = true
	c.Spec.PPMUrl = "https://packagemanager.example.com"
	c.Spec.PPMAuthAudience = "sts.amazonaws.com"

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	// Create the PPM auth script ConfigMap that the volume references
	scriptCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PPMAuthConfigMapName(name),
			Namespace: ns,
		},
		Data: map[string]string{"token-exchange.sh": "#!/bin/sh\necho test"},
	}
	require.NoError(t, cli.Create(ctx, scriptCM))

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	deployment := getDeployment(t, cli, ns, name+"-connect")

	// Verify PPM auth init container is present
	var foundPPMInit bool
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		if ic.Name == "ppm-auth-init" {
			foundPPMInit = true
			break
		}
	}
	assert.True(t, foundPPMInit, "Should have PPM auth init container")

	// Verify sidecar container is present (main container + sidecar)
	var foundSidecar bool
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "ppm-auth-sidecar" {
			foundSidecar = true
			break
		}
	}
	assert.True(t, foundSidecar, "Should have PPM auth sidecar container")

	// Verify PPM auth volumes
	var foundTokenVol, foundNetrcVol, foundScriptVol bool
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		switch v.Name {
		case "ppm-sa-token":
			foundTokenVol = true
			require.NotNil(t, v.Projected)
		case "ppm-auth":
			foundNetrcVol = true
			require.NotNil(t, v.EmptyDir)
		case "ppm-auth-script":
			foundScriptVol = true
			require.NotNil(t, v.ConfigMap)
		}
	}
	assert.True(t, foundTokenVol, "Should have projected SA token volume")
	assert.True(t, foundNetrcVol, "Should have netrc emptyDir volume")
	assert.True(t, foundScriptVol, "Should have script ConfigMap volume")

	// Verify main container has PPM auth volume mount and env vars
	mainContainer := deployment.Spec.Template.Spec.Containers[0]
	var foundNetrcMount bool
	for _, vm := range mainContainer.VolumeMounts {
		if vm.Name == "ppm-auth" {
			foundNetrcMount = true
			break
		}
	}
	assert.True(t, foundNetrcMount, "Main container should have ppm-auth volume mount")

	var foundNetrcEnv, foundCurlHomeEnv bool
	for _, env := range mainContainer.Env {
		switch env.Name {
		case "NETRC":
			foundNetrcEnv = true
		case "CURL_HOME":
			foundCurlHomeEnv = true
		}
	}
	assert.True(t, foundNetrcEnv, "Main container should have NETRC env var")
	assert.True(t, foundCurlHomeEnv, "Main container should have CURL_HOME env var")
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
