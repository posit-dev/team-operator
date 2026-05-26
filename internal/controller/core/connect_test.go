package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	localtest "github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/observability"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/rstudio/goex/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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
	t.Cleanup(func() { _ = localEnv.Stop() })
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

	// Wire up an in-memory meter so we can assert metric recording.
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, mp.Shutdown(context.Background())) })
	r.Instruments = observability.NewInstruments(mp.Meter("test"))

	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:            positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl: "https://idp.example.com/saml/metadata",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c)
	require.NoError(t, err)

	c = getConnect(t, cli, ns, name)

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	// Assert that the status transition metric was emitted with the expected
	// label contract. A regression that swapped from/to phases, omitted the
	// namespace label, or recorded the wrong controller would change this map.
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	var dp metricdata.DataPoint[int64]
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != observability.MetricStatusTransitionTotal {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok, "expected Sum[int64] data type")
			require.Len(t, sum.DataPoints, 1, "expected one transition per reconcile")
			dp = sum.DataPoints[0]
			found = true
		}
	}
	require.True(t, found, "expected status transition metric to be emitted")
	attrs := make(map[string]string, dp.Attributes.Len())
	for _, kv := range dp.Attributes.ToSlice() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	assert.Equal(t, map[string]string{
		observability.LabelController: "connect",
		observability.LabelNamespace:  ns,
		observability.LabelFromPhase:  observability.PhaseUnknown,
		observability.LabelToPhase:    observability.PhaseReady,
	}, attrs)
	assert.Equal(t, int64(1), dp.Value, "expected exactly one transition recorded")
}

// TestConnectReconciler_ErrorRecordsTransition exercises the error emission
// site in Reconcile (not ReconcileConnect), so a regression that drops the
// error metric while keeping the success metric — or vice versa — is caught.
func TestConnectReconciler_ErrorRecordsTransition(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "connect-err"

	ctx, r, req, _ := initConnectReconciler(t, ctx, ns, name)

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, mp.Shutdown(context.Background())) })
	r.Instruments = observability.NewInstruments(mp.Meter("test"))

	// Force ReconcileConnect to error early via the SAML mutual-exclusivity check.
	c := defineDefaultConnect(t, ns, name)
	c.Spec.Auth = positcov1beta1.AuthSpec{
		Type:                    positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl:         "https://idp.example.com/saml/metadata",
		SamlIdPAttributeProfile: "custom-profile",
		SamlUsernameAttribute:   "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/upn",
	}

	require.NoError(t, internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Connect{}, c))

	_, err := r.Reconcile(ctx, req)
	require.Error(t, err)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	var dp metricdata.DataPoint[int64]
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != observability.MetricStatusTransitionTotal {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.True(t, ok, "expected Sum[int64] data type")
			require.Len(t, sum.DataPoints, 1, "expected one transition per reconcile")
			dp = sum.DataPoints[0]
			found = true
		}
	}
	require.True(t, found, "expected status transition metric to be emitted on error")
	attrs := make(map[string]string, dp.Attributes.Len())
	for _, kv := range dp.Attributes.ToSlice() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	assert.Equal(t, observability.PhaseError, attrs[observability.LabelToPhase], "to_phase should be error")
	assert.Equal(t, "connect", attrs[observability.LabelController], "controller should be connect")
	assert.Equal(t, ns, attrs[observability.LabelNamespace], "namespace label should match")
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	_, err = r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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

	res, err := r.ReconcileConnect(ctx, req, c, observability.PhaseUnknown)
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
