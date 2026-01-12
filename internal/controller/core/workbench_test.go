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
