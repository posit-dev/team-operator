package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/keycloak/v2alpha1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func loadSchemes(scheme *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1beta1.AddToScheme(scheme))

	// IMPORTANT: register schemes for other CRDs we need to create
	// secret store
	utilruntime.Must(secretsstorev1.AddToScheme(scheme))
	// traefik
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// keycloak
	utilruntime.Must(v2alpha1.AddToScheme(scheme))
}

// runFakeSiteReconciler uses a FakeClient to run the SiteReconciler in a "fake" capacity. (i.e. no actual server API)
func runFakeSiteReconciler(t *testing.T, namespace, name string, site *v1beta1.Site) (client.WithWatch, ctrl.Result, error) {
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}

	res, err := rec.reconcileResources(context.TODO(), req, site)

	return cli, res, err
}

func TestSiteReconcileWithoutError(t *testing.T) {
	r := require.New(t)
	localTestEnv := localtest.LocalTestEnv{}
	cli, cliScheme, log, err := localTestEnv.Start(loadSchemes)

	t.Cleanup(func() {
		r.NoError(localTestEnv.Stop())
	})

	r.NoError(err)

	rec := SiteReconciler{
		Client: cli,
		Scheme: cliScheme,
		Log:    log,
	}

	site := defaultSite("no-error-site")
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "posit-team",
			Name:      "test-name",
		},
	}
	res, err := rec.reconcileResources(context.TODO(), req, site)
	if err != nil {
		t.Logf("Result: %v", res)
	}
	r.NoError(err)
}

func TestSiteDatabaseUrl(t *testing.T) {
	_, _, err := runFakeSiteReconciler(t, "posit-team", "no-database", &v1beta1.Site{})
	// should have an error and no resources
	assert.ErrorContains(t, err, "database connection")
	//assert.Len(t, client.Resources, 0)
}

func defaultSite(name string) *v1beta1.Site {
	return &v1beta1.Site{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Site",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "posit-team",
			UID:       "example-uid",
		},
		Spec: v1beta1.SiteSpec{
			WorkloadSecret: v1beta1.SecretConfig{
				VaultName: "workload-vault",
				Type:      product.SiteSecretTest,
			},
			MainDatabaseCredentialSecret: v1beta1.SecretConfig{
				VaultName: "test-vault",
				Type:      product.SiteSecretTest,
			},
			Flightdeck: v1beta1.InternalFlightdeckSpec{
				Image: "test-image",
			},
			DropDatabaseOnTeardown: false,
			Debug:                  false,
			LogFormat:              "",
			NetworkTrust:           v1beta1.NetworkTrustSameSite,
		},
	}
}

func TestSiteReconciler_DefaultSessionServiceAccount(t *testing.T) {
	siteName := "session-sa"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.Equal(t, "session-sa-workbench-session", testWorkbench.Spec.SessionConfig.Pod.ServiceAccountName)

	testConnect := getConnect(t, cli, siteNamespace, siteName)

	assert.Equal(t, "session-sa-connect-session", testConnect.Spec.SessionConfig.Pod.ServiceAccountName)
}

func TestSiteReconciler_CustomSessionServiceAccount(t *testing.T) {
	siteName := "session-sa"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionServiceAccountName: "test-sa",
	}
	site.Spec.Connect.ExperimentalFeatures = &v1beta1.InternalConnectExperimentalFeatures{
		SessionServiceAccountName: "test-sa",
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.Equal(t, "test-sa", testWorkbench.Spec.SessionConfig.Pod.ServiceAccountName)

	testConnect := getConnect(t, cli, siteNamespace, siteName)

	assert.Equal(t, "test-sa", testConnect.Spec.SessionConfig.Pod.ServiceAccountName)
}

func TestSiteReconciler_SessionEnvVars(t *testing.T) {
	siteName := "session-env-vars"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionEnvVars: []corev1.EnvVar{
			{
				Name:  "TEST_ENV_VAR",
				Value: "test-value",
			},
		},
		VsCodeExtensionsDir: "/some/dir",
	}
	site.Spec.Connect.ExperimentalFeatures = &v1beta1.InternalConnectExperimentalFeatures{
		SessionEnvVars: []corev1.EnvVar{
			{
				Name:  "CONNECT_ENV",
				Value: "some-value",
			},
		},
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.Len(t, testWorkbench.Spec.SessionConfig.Pod.Env, 1)
	assert.Equal(t, "TEST_ENV_VAR", testWorkbench.Spec.SessionConfig.Pod.Env[0].Name)
	assert.Equal(t, "test-value", testWorkbench.Spec.SessionConfig.Pod.Env[0].Value)
	assert.Contains(t, testWorkbench.Spec.Config.VsCode.Args, "--extensions-dir=/some/dir")
	assert.Contains(t, testWorkbench.Spec.Config.VsCode.Args, "--host=0.0.0.0")

	testConnect := getConnect(t, cli, siteNamespace, siteName)

	assert.Len(t, testWorkbench.Spec.SessionConfig.Pod.Env, 1)
	assert.Equal(t, "CONNECT_ENV", testConnect.Spec.SessionConfig.Pod.Env[0].Name)
	assert.Equal(t, "some-value", testConnect.Spec.SessionConfig.Pod.Env[0].Value)
}

func TestSiteLoggingAndDebug(t *testing.T) {
	siteName := "logging-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Debug = true
	site.Spec.LogFormat = product.LogFormatJson

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)

	assert.True(t, testConnect.Spec.Debug)
	assert.Equal(t, v1beta1.ConnectServiceLogFormatJson, string(testConnect.Spec.Config.Logging.ServiceLogFormat))

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.Equal(t, v1beta1.WorkbenchLogFormatJson, string(testWorkbench.Spec.Config.Logging.All.LogMessageFormat))
}

func TestSiteReplicas(t *testing.T) {
	siteName := "replicas-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.Replicas = 3
	site.Spec.Connect.Replicas = 2
	site.Spec.Flightdeck.Replicas = 1

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, 2, testConnect.Spec.Replicas)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, 3, testWorkbench.Spec.Replicas)

	testPackageManager := getPackageManager(t, cli, siteNamespace, siteName)
	assert.Equal(t, 1, testPackageManager.Spec.Replicas)

	testFlightdeck := getFlightdeck(t, cli, siteNamespace, siteName)
	assert.Equal(t, 1, testFlightdeck.Spec.Replicas)
}

func TestSiteVolumeSource(t *testing.T) {
	siteName := "volume-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.VolumeSource = v1beta1.VolumeSource{
		Type:    v1beta1.VolumeSourceTypeFsxZfs,
		DnsName: "some-dns.name",
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// test that volumes are created
	testConnect := getConnect(t, cli, siteNamespace, siteName)

	assert.NotNil(t, testConnect.Spec.Volume)
	assert.True(t, testConnect.Spec.Volume.Create)
	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.True(t, testWorkbench.Spec.Volume.Create)
	assert.NotNil(t, testWorkbench.Spec.Volume)

	// NFS
	site.Spec.VolumeSource = v1beta1.VolumeSource{
		Type:    v1beta1.VolumeSourceTypeNfs,
		DnsName: "some-nfs.name",
	}
	cli, _, err = runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)
	// test that volumes are created
	testConnect = getConnect(t, cli, siteNamespace, siteName)

	assert.NotNil(t, testConnect.Spec.Volume)
	assert.True(t, testConnect.Spec.Volume.Create)
	testWorkbench = getWorkbench(t, cli, siteNamespace, siteName)

	assert.True(t, testWorkbench.Spec.Volume.Create)
	assert.NotNil(t, testWorkbench.Spec.Volume)
}

func getConnect(t *testing.T, cli client.Client, siteNamespace, siteName string) *v1beta1.Connect {
	connect := &v1beta1.Connect{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect, &client.GetOptions{})
	assert.Nil(t, err)

	connect.APIVersion = "core.posit.team/v1beta1"
	connect.Kind = "Connect"
	return connect
}

func getWorkbench(t *testing.T, cli client.Client, siteNamespace, siteName string) *v1beta1.Workbench {
	workbench := &v1beta1.Workbench{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench, &client.GetOptions{})
	assert.Nil(t, err)

	workbench.APIVersion = "core.posit.team/v1beta1"
	workbench.Kind = "Workbench"
	return workbench
}

func getPackageManager(t *testing.T, cli client.Client, siteNamespace, siteName string) *v1beta1.PackageManager {
	pm := &v1beta1.PackageManager{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm, &client.GetOptions{})
	assert.Nil(t, err)
	return pm
}

func getFlightdeck(t *testing.T, cli client.Client, siteNamespace, siteName string) *v1beta1.Flightdeck {
	fd := &v1beta1.Flightdeck{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd, &client.GetOptions{})
	assert.Nil(t, err)
	return fd
}

func getDeployment(t *testing.T, cli client.Client, siteNamespace, deploymentName string) *appsv1.Deployment {
	dep := &appsv1.Deployment{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: deploymentName, Namespace: siteNamespace}, dep, &client.GetOptions{})
	assert.Nil(t, err)
	return dep
}

func getMiddleware(t *testing.T, cli client.Client, siteNamespace, siteName string) *v1alpha1.Middleware {
	middleware := &v1alpha1.Middleware{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, middleware, &client.GetOptions{})
	assert.Nil(t, err)

	return middleware
}

func TestSiteReconcileWithTolerations(t *testing.T) {
	siteName := "tolerations-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	// SessionTolerations apply to workbench session pods
	site.Spec.Workbench.SessionTolerations = []corev1.Toleration{
		{
			Key:      "workbench-session",
			Operator: corev1.TolerationOpExists,
		},
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify session pods receive the SessionTolerations
	workbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.NotNil(t, workbench.Spec.SessionConfig)
	assert.NotNil(t, workbench.Spec.SessionConfig.Pod)
	assert.Len(t, workbench.Spec.SessionConfig.Pod.Tolerations, 1)
	assert.Equal(t, "workbench-session", workbench.Spec.SessionConfig.Pod.Tolerations[0].Key)
}

func TestSiteReconcileWithSharedDirectory(t *testing.T) {
	siteName := "shared-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.SharedDirectory = "my-shared-dir"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// the "site" itself is not created by the operator...
	testConnect := getConnect(t, cli, siteNamespace, siteName)

	assert.Equal(t, site.Name, testConnect.Name)
	assert.Len(t, testConnect.Spec.AdditionalVolumes, 1)
	assert.Equal(t, "shared-site-shared", testConnect.Spec.AdditionalVolumes[0].PvcName)
	assert.Equal(t, fmt.Sprintf("/mnt/%s", site.Spec.SharedDirectory), testConnect.Spec.AdditionalVolumes[0].MountPath)

	// test Workbench too
	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.Equal(t, site.Name, testWorkbench.Name)
	assert.Len(t, testWorkbench.Spec.AdditionalVolumes, 1)
	assert.Equal(t, "shared-site-shared", testWorkbench.Spec.AdditionalVolumes[0].PvcName)
	assert.Equal(t, fmt.Sprintf("/mnt/%s", site.Spec.SharedDirectory), testWorkbench.Spec.AdditionalVolumes[0].MountPath)

	// test PVC exists
	pvc := &corev1.PersistentVolumeClaim{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: "shared-site-shared", Namespace: siteNamespace}, pvc, &client.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, "shared-site-shared", pvc.Name)
	assert.Equal(t, siteNamespace, pvc.Namespace)
	assert.Equal(t, corev1.PersistentVolumeAccessMode("ReadWriteMany"), pvc.Spec.AccessModes[0])
	assert.Equal(t, resource.MustParse("10Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
}

func TestSiteJupyterConfiguration(t *testing.T) {
	siteName := "jupyter-config-site"
	siteNamespace := "posit-team"

	site := defaultSite(siteName)
	site.Spec.Workbench.JupyterConfig = &v1beta1.WorkbenchJupyterConfig{
		NotebooksEnabled:             0,
		LabsEnabled:                  1,
		JupyterExe:                   "/opt/jupyter/bin/jupyter",
		LabVersion:                   "4.0.0",
		NotebookVersion:              "7.0.0",
		SessionCullMinutes:           60,
		SessionShutdownMinutes:       15,
		DefaultSessionCluster:        "custom-cluster",
		DefaultSessionContainerImage: "custom/jupyter:latest",
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that the Jupyter configuration was applied
	assert.NotNil(t, testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter)
	assert.Equal(t, 0, testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.NotebooksEnabled)
	assert.Equal(t, 1, testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.LabsEnabled)
	assert.Equal(t, "/opt/jupyter/bin/jupyter", testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.JupyterExe)
	assert.Equal(t, "4.0.0", testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.LabVersion)
	assert.Equal(t, "7.0.0", testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.NotebookVersion)
	assert.Equal(t, 60, testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.SessionCullMinutes)
	assert.Equal(t, 15, testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.SessionShutdownMinutes)
	assert.Equal(t, "custom-cluster", testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.DefaultSessionCluster)
	assert.Equal(t, "custom/jupyter:latest", testWorkbench.Spec.Config.WorkbenchIniConfig.Jupyter.DefaultSessionContainerImage)
}

func TestSiteConnectDatabaseSchemas(t *testing.T) {
	siteName := "connect-schemas-site"
	siteNamespace := "posit-team"

	// Test with custom schemas
	site := defaultSite(siteName)
	// Initialize Connect and DatabaseSettings
	site.Spec.Connect = v1beta1.InternalConnectSpec{
		DatabaseSettings: &v1beta1.DatabaseSettings{
			Schema:                "custom_connect_schema",
			InstrumentationSchema: "custom_metrics_schema",
		},
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)

	// Verify that the custom schema values were passed to the Connect controller
	assert.Equal(t, "custom_connect_schema", testConnect.Spec.DatabaseConfig.Schema)
	assert.Equal(t, "custom_metrics_schema", testConnect.Spec.DatabaseConfig.InstrumentationSchema)

	// Test with empty string schemas
	siteNameEmpty := "connect-empty-schemas-site"
	siteEmpty := defaultSite(siteNameEmpty)
	// Explicitly set empty schemas to ensure they are passed as empty
	siteEmpty.Spec.Connect = v1beta1.InternalConnectSpec{
		DatabaseSettings: &v1beta1.DatabaseSettings{
			Schema:                "",
			InstrumentationSchema: "",
		},
	}

	cliEmpty, _, err := runFakeSiteReconciler(t, siteNamespace, siteNameEmpty, siteEmpty)
	assert.Nil(t, err)

	testConnectEmpty := getConnect(t, cliEmpty, siteNamespace, siteNameEmpty)

	// Verify that empty schema values are correctly passed through
	// (The Connect controller will apply the default values of "connect" and "instrumentation" later)
	assert.Empty(t, testConnectEmpty.Spec.DatabaseConfig.Schema)
	assert.Empty(t, testConnectEmpty.Spec.DatabaseConfig.InstrumentationSchema)

	// Test with completely omitted schema fields (default behavior)
	siteNameOmitted := "connect-omitted-schemas-site"
	siteOmitted := defaultSite(siteNameOmitted)
	// Initialize Connect without DatabaseSettings to simulate omitted fields
	siteOmitted.Spec.Connect = v1beta1.InternalConnectSpec{}

	cliOmitted, _, err := runFakeSiteReconciler(t, siteNamespace, siteNameOmitted, siteOmitted)
	assert.Nil(t, err)

	testConnectOmitted := getConnect(t, cliOmitted, siteNamespace, siteNameOmitted)

	// When schema fields are omitted in the Site CR, empty strings are passed to the Connect CR
	// The Connect controller will then apply its defaults ("connect" and "instrumentation")
	assert.Empty(t, testConnectOmitted.Spec.DatabaseConfig.Schema)
	assert.Empty(t, testConnectOmitted.Spec.DatabaseConfig.InstrumentationSchema)
}

func TestSiteReconcileWithSmtp(t *testing.T) {
	siteName := "smtp-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Connect.ExperimentalFeatures = &v1beta1.InternalConnectExperimentalFeatures{
		MailSender: "my-email@address.com",
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, site.Spec.Connect.ExperimentalFeatures.MailSender, testConnect.Spec.Config.Server.SenderEmail)
	assert.Equal(t, "SMTP", testConnect.Spec.Config.Server.EmailProvider)
}

func TestSiteReconcileWithSA(t *testing.T) {
	r := require.New(t)
	localTestEnv := localtest.LocalTestEnv{}
	cli, cliScheme, log, err := localTestEnv.Start(loadSchemes)

	r.NoError(err)

	t.Cleanup(func() {
		r.NoError(localTestEnv.Stop())
	})

	site := defaultSite("test-site")
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionServiceAccountName: "test-sa",
	}
	site.Spec.Workbench.NodeSelector = map[string]string{"team": "posit"}

	rec := SiteReconciler{
		Client: cli,
		Scheme: cliScheme,
		Log:    log,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "posit-team",
			Name:      "test-sa",
		},
	}
	res, err := rec.reconcileResources(context.TODO(), req, site)
	if err != nil {
		t.Logf("Result: %v", res)
	}
	r.NoError(err)

	// turn on privileged
	site.Spec.Workbench.ExperimentalFeatures.PrivilegedSessions = true
	res, err = rec.reconcileResources(context.TODO(), req, site)
	if err != nil {
		t.Logf("Result: %v", res)
	}
	r.NoError(err)

	// check that workbench looks how we expect
	tmpWorkbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), req.NamespacedName, tmpWorkbench, &client.GetOptions{})
	r.NoError(err)
	r.NotNil(tmpWorkbench)
	r.True(*tmpWorkbench.Spec.SessionConfig.Pod.ContainerSecurityContext.Privileged)
	r.Equal(tmpWorkbench.Spec.SessionConfig.Pod.NodeSelector["team"], "posit")

	// turn off service-account
	site.Spec.Workbench.ExperimentalFeatures.SessionServiceAccountName = ""
	res, err = rec.reconcileResources(context.TODO(), req, site)
	if err != nil {
		t.Logf("Result: %v", res)
	}
	r.NoError(err)
}

func TestSiteReconcileWithExperimental(t *testing.T) {
	localTestEnv := localtest.LocalTestEnv{}
	cli, cliScheme, log, err := localTestEnv.Start(loadSchemes)

	assert.Nil(t, err)

	site := defaultSite("experimental-site")
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionSaveActionDefault: v1beta1.SessionSaveActionYes,
	}

	rec := SiteReconciler{
		Client: cli,
		Scheme: cliScheme,
		Log:    log,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "posit-team",
			Name:      "test-sa",
		},
	}
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.Nil(t, err)

	// check that workbench looks how we expect
	tmpWorkbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), req.NamespacedName, tmpWorkbench, &client.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, tmpWorkbench)
	assert.NotNil(t, tmpWorkbench.Spec.Config.RSession)
	assert.Equal(t, v1beta1.SessionSaveActionYes, string(tmpWorkbench.Spec.Config.RSession.SessionSaveActionDefault))

	site.Spec.Workbench.ExperimentalFeatures.SessionSaveActionDefault = ""
	site.Spec.Workbench.ExperimentalFeatures.VsCodePath = "/some/path"

	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.Nil(t, err)

	// check that things look right
	err = cli.Get(context.TODO(), req.NamespacedName, tmpWorkbench, &client.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, tmpWorkbench)
	assert.NotNil(t, tmpWorkbench.Spec.Config.VsCode)
	assert.Equal(t, "/some/path", tmpWorkbench.Spec.Config.WorkbenchIniConfig.VsCode.Exe)

	site.Spec.Workbench.ExperimentalFeatures.VsCodePath = ""
	site.Spec.Workbench.ExperimentalFeatures.DatabricksForceEnabled = true

	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.Nil(t, err)

	// check that things look right
	err = cli.Get(context.TODO(), req.NamespacedName, tmpWorkbench, &client.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, tmpWorkbench)
	assert.NotNil(t, tmpWorkbench.Spec.Config.RServer)
	assert.Equal(t, 1, tmpWorkbench.Spec.Config.RServer.DatabricksEnabled)

	// stop testEnv
	err = localTestEnv.Stop()
	assert.Nil(t, err)
}

func TestSiteKeycloak(t *testing.T) {
	siteName := "keycloak"
	siteNamespace := "posit-team"

	err := product.GlobalTestSecretProvider.SetSecret("main-database-url", "postgres://my-url:5432/my-db")
	assert.Nil(t, err)
	siteKey := client.ObjectKey{Name: siteName, Namespace: siteNamespace}
	site := defaultSite("keycloak")
	site.Spec.Keycloak = v1beta1.InternalKeycloakSpec{
		Enabled:         true,
		Image:           "",
		ImagePullPolicy: "",
		NodeSelector:    nil,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// the "site" itself is not created by the operator...
	testConnect := &v1beta1.Connect{}
	err = cli.Get(context.TODO(), siteKey, testConnect, &client.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, site.Name, testConnect.Name)
	assert.Equal(t, site.Namespace, testConnect.Namespace)

	testKeycloakList := &v2alpha1.KeycloakList{}
	err = cli.List(context.TODO(), testKeycloakList, &client.ListOptions{})
	assert.Nil(t, err)
	assert.Len(t, testKeycloakList.Items, 1)
	fmt.Printf("Keycloak List: %+v\n", testKeycloakList)

	// should be able to find a keycloak
	testKeycloak := &v2alpha1.Keycloak{}
	keycloakName := fmt.Sprintf("%s-keycloak", site.Name)
	keycloakKey := client.ObjectKey{Name: keycloakName, Namespace: siteNamespace}
	err = cli.Get(context.TODO(), keycloakKey, testKeycloak, &client.GetOptions{})
	assert.Nil(t, err)

	// name should have a "-keycloak" suffix to ensure operator adds that distinction
	assert.Equal(t, keycloakName, testKeycloak.Name)
	assert.Equal(t, site.Namespace, testKeycloak.Namespace)

	testPostgres := &v1beta1.PostgresDatabase{}
	err = cli.Get(context.TODO(), keycloakKey, testPostgres, &client.GetOptions{})

	assert.Nil(t, err)
	assert.Equal(t, keycloakName, testPostgres.Name)
	assert.Equal(t, site.Namespace, testPostgres.Namespace)

	testTraefik := &v1alpha1.Middleware{}
	middlewareName := fmt.Sprintf("%s-keycloak-forward", site.Name)
	middlewareKey := client.ObjectKey{Name: middlewareName, Namespace: siteNamespace}
	err = cli.Get(context.TODO(), middlewareKey, testTraefik, &client.GetOptions{})

	assert.Nil(t, err)
	assert.Equal(t, middlewareName, testTraefik.Name)
	assert.Equal(t, site.Namespace, testTraefik.Namespace)
}

func TestSiteKeycloakCustomImage(t *testing.T) {
	siteName := "keycloak-custom-image"
	siteNamespace := "posit-team"
	customImage := "quay.io/my-company/my-keycloak:latest"

	err := product.GlobalTestSecretProvider.SetSecret("main-database-url", "postgres://my-url:5432/my-db")
	assert.Nil(t, err)

	site := defaultSite(siteName)
	site.Spec.Keycloak = v1beta1.InternalKeycloakSpec{
		Enabled:         true,
		Image:           customImage,
		ImagePullPolicy: "",
		NodeSelector:    nil,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify that the Keycloak CR was created with the custom image
	testKeycloak := &v2alpha1.Keycloak{}
	keycloakName := fmt.Sprintf("%s-keycloak", site.Name)
	keycloakKey := client.ObjectKey{Name: keycloakName, Namespace: siteNamespace}
	err = cli.Get(context.TODO(), keycloakKey, testKeycloak, &client.GetOptions{})
	assert.Nil(t, err)

	// Verify the custom image is set in the Keycloak spec
	assert.Equal(t, customImage, testKeycloak.Spec.Image, "Custom Keycloak image should be set in the Keycloak CR")
	assert.Equal(t, keycloakName, testKeycloak.Name)
	assert.Equal(t, site.Namespace, testKeycloak.Namespace)
}

func TestSiteKeycloakWithoutCustomImage(t *testing.T) {
	siteName := "keycloak-no-custom-image"
	siteNamespace := "posit-team"

	err := product.GlobalTestSecretProvider.SetSecret("main-database-url", "postgres://my-url:5432/my-db")
	assert.Nil(t, err)

	site := defaultSite(siteName)
	site.Spec.Keycloak = v1beta1.InternalKeycloakSpec{
		Enabled:         true,
		Image:           "", // No custom image specified
		ImagePullPolicy: "",
		NodeSelector:    nil,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify that the Keycloak CR was created without a custom image
	testKeycloak := &v2alpha1.Keycloak{}
	keycloakName := fmt.Sprintf("%s-keycloak", site.Name)
	keycloakKey := client.ObjectKey{Name: keycloakName, Namespace: siteNamespace}
	err = cli.Get(context.TODO(), keycloakKey, testKeycloak, &client.GetOptions{})
	assert.Nil(t, err)

	// Verify the image field is empty when no custom image is specified
	assert.Empty(t, testKeycloak.Spec.Image, "Keycloak image should be empty when no custom image is specified")
	assert.Equal(t, keycloakName, testKeycloak.Name)
	assert.Equal(t, site.Namespace, testKeycloak.Namespace)
}

func TestSiteWorkbenchAdminGroupsDefault(t *testing.T) {
	siteName := "admin-groups-default"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that the default admin group is used when AdminGroups is not specified
	assert.Equal(t, "workbench-admin", testWorkbench.Spec.Config.RServer.AdminGroup)
}

func TestSiteWorkbenchAdminGroupsSingle(t *testing.T) {
	siteName := "admin-groups-single"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.AdminGroups = []string{"super-admins"}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that the single admin group is set correctly
	assert.Equal(t, "super-admins", testWorkbench.Spec.Config.RServer.AdminGroup)
}

func TestSiteWorkbenchAdminGroupsMultiple(t *testing.T) {
	siteName := "admin-groups-multiple"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.AdminGroups = []string{"workbench-admin", "super-admins", "it-team"}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that multiple admin groups are joined with commas
	assert.Equal(t, "workbench-admin,super-admins,it-team", testWorkbench.Spec.Config.RServer.AdminGroup)
}

func TestSiteWorkbenchAdminSuperuserGroupsDefault(t *testing.T) {
	siteName := "admin-superuser-groups-default"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that when AdminSuperuserGroups is not specified, it defaults to empty
	assert.Equal(t, "", testWorkbench.Spec.Config.RServer.AdminSuperuserGroup)
}

func TestSiteWorkbenchAdminSuperuserGroupsSingle(t *testing.T) {
	siteName := "admin-superuser-groups-single"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.AdminSuperuserGroups = []string{"super-admins"}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that the single admin superuser group is set correctly
	assert.Equal(t, "super-admins", testWorkbench.Spec.Config.RServer.AdminSuperuserGroup)
}

func TestSiteWorkbenchAdminSuperuserGroupsMultiple(t *testing.T) {
	siteName := "admin-superuser-groups-multiple"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.AdminSuperuserGroups = []string{"super-admins", "root-admins"}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that multiple admin superuser groups are joined with commas
	assert.Equal(t, "super-admins,root-admins", testWorkbench.Spec.Config.RServer.AdminSuperuserGroup)
}

func TestSiteReconciler_ConnectSessionImagePullPolicy(t *testing.T) {
	siteName := "connect-session-image-pull-policy"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Connect.ExperimentalFeatures = &v1beta1.InternalConnectExperimentalFeatures{
		SessionImagePullPolicy: corev1.PullAlways,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, corev1.PullAlways, testConnect.Spec.SessionConfig.Pod.ImagePullPolicy)
}

func TestSiteReconciler_ConnectSessionImagePullPolicyIfNotPresent(t *testing.T) {
	siteName := "connect-session-image-pull-policy-ifnotpresent"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Connect.ExperimentalFeatures = &v1beta1.InternalConnectExperimentalFeatures{
		SessionImagePullPolicy: corev1.PullIfNotPresent,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, corev1.PullIfNotPresent, testConnect.Spec.SessionConfig.Pod.ImagePullPolicy)
}

func TestSiteReconciler_ConnectSessionImagePullPolicyNever(t *testing.T) {
	siteName := "connect-session-image-pull-policy-never"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Connect.ExperimentalFeatures = &v1beta1.InternalConnectExperimentalFeatures{
		SessionImagePullPolicy: corev1.PullNever,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, corev1.PullNever, testConnect.Spec.SessionConfig.Pod.ImagePullPolicy)
}

func TestSiteReconciler_WorkbenchSessionImagePullPolicy(t *testing.T) {
	siteName := "workbench-session-image-pull-policy"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionImagePullPolicy: corev1.PullAlways,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, corev1.PullAlways, testWorkbench.Spec.SessionConfig.Pod.ImagePullPolicy)
}

func TestSiteReconciler_WorkbenchSessionImagePullPolicyIfNotPresent(t *testing.T) {
	siteName := "workbench-session-image-pull-policy-ifnotpresent"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionImagePullPolicy: corev1.PullIfNotPresent,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, corev1.PullIfNotPresent, testWorkbench.Spec.SessionConfig.Pod.ImagePullPolicy)
}

func TestSiteReconciler_WorkbenchSessionImagePullPolicyNever(t *testing.T) {
	siteName := "workbench-session-image-pull-policy-never"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
		SessionImagePullPolicy: corev1.PullNever,
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, corev1.PullNever, testWorkbench.Spec.SessionConfig.Pod.ImagePullPolicy)
}
