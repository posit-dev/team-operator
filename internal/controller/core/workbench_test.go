package core

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	localtest "github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/db"
	"github.com/posit-dev/team-operator/internal/observability"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
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
	t.Cleanup(func() { _ = localEnv.Stop() })
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

	// Wire up an in-memory meter so we can assert metric recording.
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer mp.Shutdown(context.Background())
	r.Meter = mp.Meter("test")

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

	// Assert that status transition metric was recorded.
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &rm))
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == observability.MetricStatusTransitionTotal {
				found = true
			}
		}
	}
	assert.True(t, found, "expected status transition to be recorded")
}

func TestWorkbenchReadinessProbePath(t *testing.T) {
	cases := []struct {
		name     string
		override string
		wantPath string
	}{
		{name: "default", override: "", wantPath: defaultWorkbenchReadinessProbePath},
		{name: "override", override: "/custom-health", wantPath: "/custom-health"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ns := "posit-team"
			wbName := "workbench-probe-" + tc.name

			ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, wbName)

			wb := defineDefaultWorkbench(t, ns, wbName)
			if tc.override != "" {
				override := tc.override
				wb.Spec.ReadinessProbePath = &override
			}

			err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
			require.NoError(t, err)

			wb = getWorkbench(t, cli, ns, wbName)
			res, err := r.ReconcileWorkbench(ctx, req, wb)
			require.NoError(t, err)
			require.True(t, res.IsZero())

			deployment := getDeployment(t, cli, ns, wbName+"-workbench")
			mainContainer := deployment.Spec.Template.Spec.Containers[0]
			require.NotNil(t, mainContainer.ReadinessProbe, "readiness probe must be set")
			require.NotNil(t, mainContainer.ReadinessProbe.HTTPGet, "readiness probe must use HTTPGet")
			assert.Equal(t, tc.wantPath, mainContainer.ReadinessProbe.HTTPGet.Path)
		})
	}
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

func TestWorkbenchLoadBalancingInitContainer(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-lb"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	// Enable load balancing
	wb.Spec.Config.RServer = &positcov1beta1.WorkbenchRServerConfig{
		LoadBalancingEnabled: 1,
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// Get the deployment and verify init container exists
	deployment := getDeployment(t, cli, ns, name+"-workbench")

	// Verify init container is present
	require.Len(t, deployment.Spec.Template.Spec.InitContainers, 1, "Should have one init container when load balancing is enabled")
	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "load-balancer-init", initContainer.Name)
	assert.Equal(t, "busybox:1.36", initContainer.Image)
	assert.Contains(t, initContainer.Args[0], "www-host-name=$(hostname -i)")
	assert.Contains(t, initContainer.Args[0], "delete-node-on-exit=1")

	// Verify volume mount on init container
	require.Len(t, initContainer.VolumeMounts, 1, "Init container should have volume mount")
	assert.Equal(t, "load-balancer-config", initContainer.VolumeMounts[0].Name)
	assert.Equal(t, "/mnt/load-balancer", initContainer.VolumeMounts[0].MountPath)

	// Verify the main container has the volume mount
	mainContainer := deployment.Spec.Template.Spec.Containers[0]
	var foundLbMount bool
	for _, vm := range mainContainer.VolumeMounts {
		if vm.Name == "load-balancer-config" {
			foundLbMount = true
			assert.Equal(t, "/mnt/load-balancer", vm.MountPath)
			break
		}
	}
	assert.True(t, foundLbMount, "Main container should have load-balancer-config volume mount")

	// Verify the emptyDir volume is defined
	var foundLbVolume bool
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == "load-balancer-config" {
			foundLbVolume = true
			assert.NotNil(t, v.EmptyDir, "load-balancer-config should be an emptyDir volume")
			break
		}
	}
	assert.True(t, foundLbVolume, "Deployment should have load-balancer-config volume")
}

func TestWorkbenchLoadBalancingDisabled(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-no-lb"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	// Do NOT enable load balancing - leave RServer config nil

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// Get the deployment and verify NO init container exists
	deployment := getDeployment(t, cli, ns, name+"-workbench")

	// Verify no init containers when load balancing is disabled
	assert.Empty(t, deployment.Spec.Template.Spec.InitContainers, "Should have no init containers when load balancing is disabled")

	// Verify no load-balancer-config volume exists
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		assert.NotEqual(t, "load-balancer-config", v.Name, "Should not have load-balancer-config volume when load balancing is disabled")
	}
}

func TestWorkbenchPodDisruptionBudgets(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-pdb"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// Verify session PDB is created
	sessionPdb := getPodDisruptionBudget(t, cli, ns, name+"-workbench-sessions")
	require.NotNil(t, sessionPdb, "Session PDB should be created")
	assert.Equal(t, name+"-workbench-sessions", sessionPdb.Name)

	// Verify session PDB has correct selector to target session pods
	require.NotNil(t, sessionPdb.Spec.Selector, "Session PDB should have a selector")
	assert.Equal(t, wb.ComponentName(), sessionPdb.Spec.Selector.MatchLabels["launcher-instance-id"],
		"Session PDB should select pods with launcher-instance-id label matching workbench component name")

	// Verify session PDB has maxUnavailable=0 to prevent any evictions
	require.NotNil(t, sessionPdb.Spec.MaxUnavailable, "Session PDB should have maxUnavailable set")
	assert.Equal(t, int32(0), sessionPdb.Spec.MaxUnavailable.IntVal,
		"Session PDB should have maxUnavailable=0 to prevent session evictions")
}

// TestWorkbenchReconciler_Suspended verifies that when Workbench has Suspended=true,
// ReconcileWorkbench does not create serving resources (Deployment, Service, Ingress).
func TestWorkbenchReconciler_Suspended(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-suspended"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	suspended := true
	wb.Spec.Suspended = &suspended

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// No serving resources should be created when suspended
	dep := &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, dep)
	assert.Error(t, err, "Deployment should not exist when Workbench is suspended")

	svc := &corev1.Service{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, svc)
	assert.Error(t, err, "Service should not exist when Workbench is suspended")

	ing := &networkingv1.Ingress{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, ing)
	assert.Error(t, err, "Ingress should not exist when Workbench is suspended")

	// Status should reflect the suspended state
	updated := &positcov1beta1.Workbench{}
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

// TestWorkbenchReconciler_SuspendRemovesDeployment verifies that when Workbench transitions
// to Suspended=true, the Deployment is removed while data resources are preserved.
func TestWorkbenchReconciler_SuspendRemovesDeployment(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-suspend-removes"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	// Pass 1: normal reconcile — Deployment should be created
	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	dep := &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, dep)
	require.NoError(t, err, "Deployment should exist after normal reconcile")

	// Pre-create DB password secret to verify it is preserved during suspension
	pwSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      db.PasswordSecretName(wb.ComponentName()),
			Namespace: ns,
		},
	}
	err = cli.Create(ctx, pwSecret)
	require.NoError(t, err)

	// Pass 2: suspend — Deployment should be removed
	wb = getWorkbench(t, cli, ns, name)
	suspended := true
	wb.Spec.Suspended = &suspended
	err = cli.Update(ctx, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)
	res, err = r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	dep = &appsv1.Deployment{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, dep)
	assert.Error(t, err, "Deployment should be removed when Workbench is suspended")

	svc := &corev1.Service{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, svc)
	assert.Error(t, err, "Service should be removed when Workbench is suspended")

	ing := &networkingv1.Ingress{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName(), Namespace: ns}, ing)
	assert.Error(t, err, "Ingress should be removed when Workbench is suspended")

	// Data resources must be preserved during suspension
	loginCm := &corev1.ConfigMap{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.LoginConfigmapName(), Namespace: ns}, loginCm)
	assert.NoError(t, err, "Login ConfigMap should be preserved when Workbench is suspended")

	// DB password secret must also be preserved during suspension
	err = cli.Get(ctx, client.ObjectKey{Name: db.PasswordSecretName(wb.ComponentName()), Namespace: ns}, pwSecret)
	assert.NoError(t, err, "DB password secret should be preserved when Workbench is suspended")
}

// TestWorkbenchReconciler_CleanupDeletesDatabasePasswordSecret verifies that CleanupWorkbench
// deletes the DB password secret.
func TestWorkbenchReconciler_CleanupDeletesDatabasePasswordSecret(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-cleanup-db-secret"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	// Pre-create the DB password secret
	secretName := db.PasswordSecretName(wb.ComponentName())
	pwSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
		},
	}
	err = cli.Create(ctx, pwSecret)
	require.NoError(t, err)

	// Verify it exists before cleanup
	existing := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: secretName, Namespace: ns}, existing)
	require.NoError(t, err, "DB password secret should exist before cleanup")

	// Run CleanupWorkbench
	_, err = r.CleanupWorkbench(ctx, req, wb)
	require.NoError(t, err)

	// Assert the secret is gone
	deleted := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: secretName, Namespace: ns}, deleted)
	assert.True(t, apierrors.IsNotFound(err), "DB password secret should be deleted after CleanupWorkbench")
}

// TestWorkbenchSCIM_Disabled verifies that when SCIM is not configured, no SCIM secret is
// created and no scim-token volume mount is added to the deployment.
func TestWorkbenchSCIM_Disabled(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-scim-disabled"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	// SCIM is nil by default — do not set it.

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// No managed SCIM secret should be created.
	scimSecret := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: wb.ComponentName() + "-scim-token", Namespace: ns}, scimSecret)
	assert.True(t, apierrors.IsNotFound(err), "managed SCIM secret should not exist when SCIM is disabled")

	// No scim-token volume mount, volume, or env var on the main container.
	deployment := getDeployment(t, cli, ns, wb.ComponentName())
	mainContainer := deployment.Spec.Template.Spec.Containers[0]
	for _, vm := range mainContainer.VolumeMounts {
		assert.NotEqual(t, "scim-token", vm.Name, "scim-token volume mount should not exist when SCIM is disabled")
	}
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		assert.NotEqual(t, "scim-token", v.Name, "scim-token volume should not exist when SCIM is disabled")
	}
	for _, e := range mainContainer.Env {
		assert.NotEqual(t, "WORKBENCH_USER_SERVICE_AUTH_TOKEN_PATH", e.Name, "WORKBENCH_USER_SERVICE_AUTH_TOKEN_PATH should not be set when SCIM is disabled")
	}
}

// TestWorkbenchSCIM_EnabledManagedToken verifies that when SCIM is enabled with no tokenSecretName,
// the operator creates a managed secret with a random token and wires a volume mount into the deployment.
func TestWorkbenchSCIM_EnabledManagedToken(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-scim-managed"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.SCIM = &positcov1beta1.WorkbenchSCIMConfig{
		Enabled: true,
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// Managed SCIM secret should be created with a non-empty token.
	managedSecretName := wb.ComponentName() + "-scim-token"
	scimSecret := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: managedSecretName, Namespace: ns}, scimSecret)
	require.NoError(t, err, "managed SCIM secret should be created")
	assert.NotEmpty(t, scimSecret.Data["token"], "managed SCIM secret should have a non-empty token")

	// Deployment should have scim-token volume and volume mount.
	deployment := getDeployment(t, cli, ns, wb.ComponentName())
	mainContainer := deployment.Spec.Template.Spec.Containers[0]

	var foundMount bool
	for _, vm := range mainContainer.VolumeMounts {
		if vm.Name == "scim-token" {
			foundMount = true
			assert.Equal(t, "/etc/rstudio/scim-token", vm.MountPath)
			assert.Equal(t, "scim-token", vm.SubPath)
			assert.True(t, vm.ReadOnly)
			break
		}
	}
	assert.True(t, foundMount, "scim-token volume mount should be present when SCIM is enabled")

	var foundVolume bool
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == "scim-token" {
			foundVolume = true
			require.NotNil(t, v.Secret, "scim-token volume should be backed by a Secret")
			assert.Equal(t, managedSecretName, v.Secret.SecretName)
			break
		}
	}
	assert.True(t, foundVolume, "scim-token volume should be present when SCIM is enabled")

	var foundEnvVar bool
	for _, e := range mainContainer.Env {
		if e.Name == "WORKBENCH_USER_SERVICE_AUTH_TOKEN_PATH" {
			foundEnvVar = true
			assert.Equal(t, "/etc/rstudio/scim-token", e.Value)
			break
		}
	}
	assert.True(t, foundEnvVar, "WORKBENCH_USER_SERVICE_AUTH_TOKEN_PATH should be set when SCIM is enabled")
}

// TestWorkbenchSCIM_BYOToken verifies that when tokenSecretName is specified, the operator
// uses that secret directly and does not create a managed secret.
func TestWorkbenchSCIM_BYOToken(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-scim-byo"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	// Pre-create the BYO secret.
	byoSecretName := "my-custom-scim-token"
	byoSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: byoSecretName, Namespace: ns},
		StringData: map[string]string{"token": "my-byo-token"},
	}
	err := cli.Create(ctx, byoSecret)
	require.NoError(t, err)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.SCIM = &positcov1beta1.WorkbenchSCIMConfig{
		Enabled:         true,
		TokenSecretName: byoSecretName,
	}

	err = internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// No managed secret should be created.
	managedSecretName := wb.ComponentName() + "-scim-token"
	managedSecret := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: managedSecretName, Namespace: ns}, managedSecret)
	assert.True(t, apierrors.IsNotFound(err), "managed SCIM secret should not be created in BYO mode")

	// The volume should reference the BYO secret.
	deployment := getDeployment(t, cli, ns, wb.ComponentName())
	var foundVolume bool
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == "scim-token" {
			foundVolume = true
			require.NotNil(t, v.Secret)
			assert.Equal(t, byoSecretName, v.Secret.SecretName, "volume should reference the BYO secret")
			break
		}
	}
	assert.True(t, foundVolume, "scim-token volume should be present in BYO mode")
}

// TestWorkbenchSCIM_NoTokenRotation verifies that a second reconcile does not rotate the token
// when the managed secret already exists.
func TestWorkbenchSCIM_NoTokenRotation(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-scim-no-rotate"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.SCIM = &positcov1beta1.WorkbenchSCIMConfig{
		Enabled: true,
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	// First reconcile — creates the managed secret.
	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	managedSecretName := wb.ComponentName() + "-scim-token"
	firstSecret := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: managedSecretName, Namespace: ns}, firstSecret)
	require.NoError(t, err)
	firstToken := string(firstSecret.Data["token"])
	require.NotEmpty(t, firstToken)

	// Second reconcile — token must not change.
	wb = getWorkbench(t, cli, ns, name)
	res, err = r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	secondSecret := &corev1.Secret{}
	err = cli.Get(ctx, client.ObjectKey{Name: managedSecretName, Namespace: ns}, secondSecret)
	require.NoError(t, err)
	secondToken := string(secondSecret.Data["token"])

	assert.Equal(t, firstToken, secondToken, "SCIM token must not be rotated on re-reconcile")
}

// TestWorkbenchSCIM_DisableAfterEnable verifies that disabling SCIM after it was enabled removes
// the scim-token volume mount, volume, and env var from the deployment.
func TestWorkbenchSCIM_DisableAfterEnable(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-scim-disable-after-enable"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.SCIM = &positcov1beta1.WorkbenchSCIMConfig{
		Enabled: true,
	}

	err := internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	// First reconcile — SCIM enabled.
	res, err := r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// Verify SCIM volume is present after the first reconcile.
	deployment := getDeployment(t, cli, ns, wb.ComponentName())
	var foundVolume bool
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == "scim-token" {
			foundVolume = true
			break
		}
	}
	require.True(t, foundVolume, "scim-token volume should be present when SCIM is enabled")

	// Disable SCIM and reconcile again.
	wb = getWorkbench(t, cli, ns, name)
	wb.Spec.SCIM = &positcov1beta1.WorkbenchSCIMConfig{
		Enabled: false,
	}
	err = cli.Update(ctx, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)
	res, err = r.ReconcileWorkbench(ctx, req, wb)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	// All SCIM-related resources should be gone from the deployment.
	deployment = getDeployment(t, cli, ns, wb.ComponentName())
	mainContainer := deployment.Spec.Template.Spec.Containers[0]

	for _, vm := range mainContainer.VolumeMounts {
		assert.NotEqual(t, "scim-token", vm.Name, "scim-token volume mount should be removed when SCIM is disabled")
	}
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		assert.NotEqual(t, "scim-token", v.Name, "scim-token volume should be removed when SCIM is disabled")
	}
	for _, e := range mainContainer.Env {
		assert.NotEqual(t, "WORKBENCH_USER_SERVICE_AUTH_TOKEN_PATH", e.Name, "WORKBENCH_USER_SERVICE_AUTH_TOKEN_PATH should be removed when SCIM is disabled")
	}
}

// TestWorkbenchSCIM_BYOTokenMissingKey verifies that when a BYO secret exists but lacks the
// "token" key, reconciliation fails with a descriptive error.
func TestWorkbenchSCIM_BYOTokenMissingKey(t *testing.T) {
	ctx := context.Background()
	ns := "posit-team"
	name := "workbench-scim-byo-missing-key"

	ctx, r, req, cli := initWorkbenchReconciler(t, ctx, ns, name)

	// Pre-create a BYO secret that is missing the "token" key.
	byoSecretName := "my-incomplete-scim-secret"
	byoSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: byoSecretName, Namespace: ns},
		StringData: map[string]string{"wrong-key": "some-value"},
	}
	err := cli.Create(ctx, byoSecret)
	require.NoError(t, err)

	wb := defineDefaultWorkbench(t, ns, name)
	wb.Spec.SCIM = &positcov1beta1.WorkbenchSCIMConfig{
		Enabled:         true,
		TokenSecretName: byoSecretName,
	}

	err = internal.BasicCreateOrUpdate(ctx, r, r.GetLogger(ctx), req.NamespacedName, &positcov1beta1.Workbench{}, wb)
	require.NoError(t, err)

	wb = getWorkbench(t, cli, ns, name)

	// Reconciliation should fail — missing "token" key is a blocking error.
	_, err = r.ReconcileWorkbench(ctx, req, wb)
	require.ErrorContains(t, err, `BYO SCIM token secret "my-incomplete-scim-secret" is missing required key "token"`)
}
