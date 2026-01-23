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

func TestAzureDatabricks(t *testing.T) {
	r := &WorkbenchReconciler{}
	ctx := context.TODO()
	req := ctrl.Request{}
	w := &positcov1beta1.Workbench{
		Spec: positcov1beta1.WorkbenchSpec{
			Secret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			SecretConfig: positcov1beta1.WorkbenchSecretConfig{
				WorkbenchSecretIniConfig: positcov1beta1.WorkbenchSecretIniConfig{
					Databricks: map[string]*positcov1beta1.WorkbenchDatabricksConfig{
						// this one checks that azure works
						"posit-azure": {
							Name:     "Azure Databricks",
							Url:      "https://someprefix.azuredatabricks.net",
							ClientId: "some-client-id",
						},
						// this checks that other targets do not get interfered with
						"posit-aws": {
							Name:     "AWS Databricks",
							Url:      "https://some-other-url.com",
							ClientId: "aws-client-id",
						},
						// this one checks that a suffix does not interfere with the match
						"another-azure": {
							Name:     "Azure Databricks 2",
							Url:      "https://someprefix.azuredatabricks.net/some-suffix/another-suffix",
							ClientId: "another-client-id",
						},
					},
				},
			},
		},
	}

	var err error
	// azure
	require.Equal(t, w.Spec.SecretConfig.Databricks["posit-azure"].ClientSecret, "")
	require.Equal(t, w.Spec.SecretConfig.Databricks["posit-aws"].ClientSecret, "")
	require.Equal(t, w.Spec.SecretConfig.Databricks["another-azure"].ClientSecret, "")
	err = r.FetchAndSetClientSecretForAzureDatabricks(ctx, req, w)
	require.NoError(t, err)
	require.Equal(t, w.Spec.SecretConfig.Databricks["posit-azure"].ClientSecret, "dev-client-secret-some-client-id")
	require.Equal(t, w.Spec.SecretConfig.Databricks["posit-aws"].ClientSecret, "")
	require.Equal(t, w.Spec.SecretConfig.Databricks["another-azure"].ClientSecret, "dev-client-secret-another-client-id")
}

func initWorkbenchReconciler(t *testing.T, ctx context.Context, namespace, name string) (context.Context, *WorkbenchReconciler, ctrl.Request, client.Client) {
	localEnv := localtest.LocalTestEnv{}
	cli, cliScheme, log, err := localEnv.Start(loadSchemes)
	require.NoError(t, err)
	r := &WorkbenchReconciler{
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

func defineDefaultWorkbench(t *testing.T, ns, name string) *positcov1beta1.Workbench {
	err := product.GlobalTestSecretProvider.SetSecret("dev-db-password", "dev-password")
	require.NoError(t, err)

	return &positcov1beta1.Workbench{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Workbench",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			UID:       "config-example-uid",
			Labels: map[string]string{
				positcov1beta1.ManagedByLabelKey: positcov1beta1.ManagedByLabelValue,
			},
		},
		Spec: positcov1beta1.WorkbenchSpec{
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

func TestWorkbenchReconciler_Basic(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-basic"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)

	// have to make sure the CRD _actually exists_
	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// check the middlewares
	cspMiddleware := getMiddleware(t, cli, ns, r.CspMiddleware(wb))
	require.Equal(t, cspMiddleware.Name, r.CspMiddleware(wb))

	forwardMiddleware := getMiddleware(t, cli, ns, r.ForwardMiddleware(wb))
	require.Equal(t, forwardMiddleware.Name, r.ForwardMiddleware(wb))

	headersMiddleware := getMiddleware(t, cli, ns, r.HeadersMiddleware(wb))
	require.Equal(t, headersMiddleware.Name, r.HeadersMiddleware(wb))
}

func TestWorkbenchConfigReload(t *testing.T) {
	ctx := context.Background()
	var err error
	ns := "posit-team"
	name := "workbench-config-reload"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)

	// have to make sure the CRD _actually exists_
	err = internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// get config SHA...
	preWb := getWorkbench(t, cli, ns, name)
	preWbDeployment := getDeployment(t, cli, ns, name+"-workbench")
	preSha := preWbDeployment.Spec.Template.ObjectMeta.Annotations[workbenchSessionShaKey]
	require.NotEqual(t, "", preSha)

	// update config...
	preWb.Spec.Config.WorkbenchSessionIniConfig.RSession = &positcov1beta1.WorkbenchRSessionConfig{
		DefaultRSConnectServer:          "https://new-rsconnect-server.com",
		SessionFirstProjectTemplatePath: "/some/path",
	}

	// reconcile again... (have to create/update too...?)
	err = internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, preWb)
	require.NoError(t, err)
	res, err = r.ReconcileWorkbench(ctx, req, preWb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// require that the config SHA has changed...
	postWbDeployment := getDeployment(t, cli, ns, name+"-workbench")
	postSha := postWbDeployment.Spec.Template.ObjectMeta.Annotations[workbenchSessionShaKey]
	require.NotEqual(t, "", postSha)

	require.NotEqual(t, preSha, postSha)
}

func TestWorkbenchAuthSaml(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-saml-auth"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.Auth = positcov1beta1.AuthSpec{
		Type:            positcov1beta1.AuthTypeSaml,
		SamlMetadataUrl: "https://saml-provider.example.com/metadata",
		UsernameClaim:   "email",
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// Verify the configmap
	configmap := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, configmap, &client.GetOptions{})
	require.NoError(t, err)

	// Check SAML configuration in rserver.conf
	rserverConfig, exists := configmap.Data["rserver.conf"]
	require.True(t, exists, "rserver.conf should exist in the ConfigMap")
	assert.Contains(t, rserverConfig, "auth-saml=1", "SAML auth should be enabled")
	assert.Contains(t, rserverConfig, "auth-saml-metadata-url=https://saml-provider.example.com/metadata", "SAML metadata URL should be set")
	assert.Contains(t, rserverConfig, "auth-saml-sp-attribute-username=email", "SAML username claim should be set")
}

func TestWorkbenchAuthSamlMissingMetadata(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-saml-no-metadata"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.Auth = positcov1beta1.AuthSpec{
		Type:          positcov1beta1.AuthTypeSaml,
		UsernameClaim: "email",
		// Intentionally not setting SamlMetadataUrl
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	// Should return an error when SamlMetadataUrl is not provided
	_, err = r.ReconcileWorkbench(ctx, req, wb)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SAML authentication requires a metadata URL")
}

// OAuth Client Tests

func TestOAuthClientSecrets(t *testing.T) {
	r := &WorkbenchReconciler{}
	ctx := context.TODO()
	req := ctrl.Request{}
	w := &positcov1beta1.Workbench{
		Spec: positcov1beta1.WorkbenchSpec{
			Secret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			SecretConfig: positcov1beta1.WorkbenchSecretConfig{
				WorkbenchSecretIniConfig: positcov1beta1.WorkbenchSecretIniConfig{
					OAuthClients: map[string]*positcov1beta1.WorkbenchOAuthClientConfig{
						"box-integration": {
							Name:             "Box",
							ClientId:         "box-client-id",
							AuthorizationUrl: "https://account.box.com/api/oauth2/authorize",
							TokenUrl:         "https://api.box.com/oauth2/token",
						},
						"google-drive": {
							Name:             "Google Drive",
							ClientId:         "google-client-id",
							AuthorizationUrl: "https://accounts.google.com/o/oauth2/v2/auth",
							TokenUrl:         "https://oauth2.googleapis.com/token",
							Scopes:           []string{"drive.readonly"},
						},
					},
				},
			},
		},
	}

	// Verify secrets are empty before fetching
	require.Equal(t, "", w.Spec.SecretConfig.OAuthClients["box-integration"].ClientSecret)
	require.Equal(t, "", w.Spec.SecretConfig.OAuthClients["google-drive"].ClientSecret)

	// Fetch secrets
	err := r.FetchAndSetOAuthClientSecrets(ctx, req, w)
	require.NoError(t, err)

	// Verify secrets were populated (test provider returns the secret name as the value)
	require.Equal(t, "dev-oauth-client-secret-box-integration", w.Spec.SecretConfig.OAuthClients["box-integration"].ClientSecret)
	require.Equal(t, "dev-oauth-client-secret-google-drive", w.Spec.SecretConfig.OAuthClients["google-drive"].ClientSecret)
}

func TestOAuthClientSecrets_NilOAuthClients(t *testing.T) {
	r := &WorkbenchReconciler{}
	ctx := context.TODO()
	req := ctrl.Request{}
	w := &positcov1beta1.Workbench{
		Spec: positcov1beta1.WorkbenchSpec{
			Secret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			SecretConfig: positcov1beta1.WorkbenchSecretConfig{
				WorkbenchSecretIniConfig: positcov1beta1.WorkbenchSecretIniConfig{
					// OAuthClients is nil - should be a no-op
				},
			},
		},
	}

	// Should return nil error for nil OAuthClients (no-op)
	err := r.FetchAndSetOAuthClientSecrets(ctx, req, w)
	require.NoError(t, err)
}

func TestOAuthClientSecrets_MissingRequiredFields(t *testing.T) {
	r := &WorkbenchReconciler{}
	ctx := context.TODO()
	req := ctrl.Request{}
	w := &positcov1beta1.Workbench{
		Spec: positcov1beta1.WorkbenchSpec{
			Secret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			SecretConfig: positcov1beta1.WorkbenchSecretConfig{
				WorkbenchSecretIniConfig: positcov1beta1.WorkbenchSecretIniConfig{
					OAuthClients: map[string]*positcov1beta1.WorkbenchOAuthClientConfig{
						// Missing client-id
						"missing-client-id": {
							Name:             "Missing Client ID",
							AuthorizationUrl: "https://example.com/auth",
							TokenUrl:         "https://example.com/token",
						},
						// Missing authorization-url
						"missing-auth-url": {
							Name:     "Missing Auth URL",
							ClientId: "some-client-id",
							TokenUrl: "https://example.com/token",
						},
						// Missing token-url
						"missing-token-url": {
							Name:             "Missing Token URL",
							ClientId:         "some-client-id",
							AuthorizationUrl: "https://example.com/auth",
						},
						// Valid integration - should still be processed
						"valid-integration": {
							Name:             "Valid",
							ClientId:         "valid-client-id",
							AuthorizationUrl: "https://example.com/auth",
							TokenUrl:         "https://example.com/token",
						},
					},
				},
			},
		},
	}

	// Should not return error - invalid integrations are skipped, valid ones processed
	err := r.FetchAndSetOAuthClientSecrets(ctx, req, w)
	require.NoError(t, err)

	// Invalid integrations should have empty secrets (they were skipped)
	require.Equal(t, "", w.Spec.SecretConfig.OAuthClients["missing-client-id"].ClientSecret)
	require.Equal(t, "", w.Spec.SecretConfig.OAuthClients["missing-auth-url"].ClientSecret)
	require.Equal(t, "", w.Spec.SecretConfig.OAuthClients["missing-token-url"].ClientSecret)

	// Valid integration should have its secret populated
	require.Equal(t, "dev-oauth-client-secret-valid-integration", w.Spec.SecretConfig.OAuthClients["valid-integration"].ClientSecret)
}

func TestOAuthClientSecrets_BackwardsCompatibility(t *testing.T) {
	// This test verifies that existing Workbench configurations without OAuth
	// continue to work correctly (backwards compatibility)
	r := &WorkbenchReconciler{}
	ctx := context.TODO()
	req := ctrl.Request{}

	// Simulate an existing Workbench with Databricks but no OAuth
	w := &positcov1beta1.Workbench{
		Spec: positcov1beta1.WorkbenchSpec{
			Secret: positcov1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			SecretConfig: positcov1beta1.WorkbenchSecretConfig{
				WorkbenchSecretIniConfig: positcov1beta1.WorkbenchSecretIniConfig{
					// Existing Databricks config
					Databricks: map[string]*positcov1beta1.WorkbenchDatabricksConfig{
						"existing-workspace": {
							Name:     "Existing Workspace",
							Url:      "https://existing.cloud.databricks.com",
							ClientId: "existing-client-id",
						},
					},
					// No OAuthClients configured (nil)
				},
			},
		},
	}

	// OAuth fetching should be a no-op and not affect Databricks
	err := r.FetchAndSetOAuthClientSecrets(ctx, req, w)
	require.NoError(t, err)

	// Databricks config should be unchanged
	require.NotNil(t, w.Spec.SecretConfig.Databricks)
	require.Equal(t, "Existing Workspace", w.Spec.SecretConfig.Databricks["existing-workspace"].Name)

	// OAuthClients should still be nil
	require.Nil(t, w.Spec.SecretConfig.OAuthClients)
}
