package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/keycloak/v2alpha1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/rstudio/goex/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
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

func TestSiteReconciler_ReadinessProbePath(t *testing.T) {
	t.Run("not_set_omits_override", func(t *testing.T) {
		siteName := "probe-default"
		siteNamespace := "posit-team"
		site := defaultSite(siteName)

		cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
		assert.Nil(t, err)

		testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
		assert.Nil(t, testWorkbench.Spec.ReadinessProbePath)
	})

	t.Run("override_propagates", func(t *testing.T) {
		siteName := "probe-override"
		siteNamespace := "posit-team"
		site := defaultSite(siteName)
		path := "/custom-probe"
		site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
			ReadinessProbePath: &path,
		}

		cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
		assert.Nil(t, err)

		testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
		require.NotNil(t, testWorkbench.Spec.ReadinessProbePath)
		assert.Equal(t, "/custom-probe", *testWorkbench.Spec.ReadinessProbePath)
	})
}

func TestSiteReconciler_LauncherSessionsProxyTimeout(t *testing.T) {
	t.Run("default_is_two", func(t *testing.T) {
		siteName := "proxy-timeout-default"
		siteNamespace := "posit-team"
		site := defaultSite(siteName)

		cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
		assert.Nil(t, err)

		testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
		require.NotNil(t, testWorkbench.Spec.Config.RServer)
		assert.Equal(t, 2, testWorkbench.Spec.Config.RServer.LauncherSessionsProxyTimeoutSeconds)
	})

	t.Run("override_propagates", func(t *testing.T) {
		siteName := "proxy-timeout-override"
		siteNamespace := "posit-team"
		site := defaultSite(siteName)
		site.Spec.Workbench.ExperimentalFeatures = &v1beta1.InternalWorkbenchExperimentalFeatures{
			LauncherSessionsProxyTimeoutSeconds: ptr.To(45),
		}

		cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
		assert.Nil(t, err)

		testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
		require.NotNil(t, testWorkbench.Spec.Config.RServer)
		assert.Equal(t, 45, testWorkbench.Spec.Config.RServer.LauncherSessionsProxyTimeoutSeconds)
	})
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

// TestSiteReconciler_EnvVars verifies that the new envVars field, including a
// valueFrom.secretKeyRef entry, propagates from the Site spec to each product CR
// spec for Workbench, Connect, and Package Manager.
func TestSiteReconciler_EnvVars(t *testing.T) {
	siteName := "env-vars"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	envVars := func(prefix string) []corev1.EnvVar {
		return []corev1.EnvVar{
			{
				Name:  prefix + "_PLAIN",
				Value: "plain-value",
			},
			{
				Name: prefix + "_FROM_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
						Key:                  "api-key",
					},
				},
			},
		}
	}

	site.Spec.Workbench.EnvVars = envVars("WB")
	site.Spec.Connect.EnvVars = envVars("CONNECT")
	site.Spec.PackageManager.EnvVars = envVars("PM")

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	assertEnvVars := func(t *testing.T, prefix string, got []corev1.EnvVar) {
		t.Helper()
		assert.Equal(t, envVars(prefix), got)
	}

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assertEnvVars(t, "WB", testWorkbench.Spec.EnvVars)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assertEnvVars(t, "CONNECT", testConnect.Spec.EnvVars)

	testPackageManager := getPackageManager(t, cli, siteNamespace, siteName)
	assertEnvVars(t, "PM", testPackageManager.Spec.EnvVars)
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

func getPodDisruptionBudget(t *testing.T, cli client.Client, namespace, name string) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, pdb, &client.GetOptions{})
	assert.Nil(t, err)
	return pdb
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

func TestSiteAuditedJobsConfiguration(t *testing.T) {
	siteName := "audited-jobs-config-site"
	siteNamespace := "posit-team"

	// Helper function to create int pointers
	intPtr := func(i int) *int {
		return &i
	}

	site := defaultSite(siteName)
	site.Spec.Workbench.AuditedJobs = &v1beta1.AuditedJobsConfig{
		Enabled:            intPtr(1),
		StoragePath:        "/mnt/shared-storage/audited-jobs",
		PrivateKeyPath:     "/etc/rstudio/audited-jobs-private-key.pem",
		PublicKeyPaths:     "/etc/rstudio/audited-jobs-public-key.pem",
		LogLimit:           intPtr(5000),
		DeletionExpiry:     intPtr(60),
		VanillaRequired:    intPtr(1),
		DetailsEnvironment: intPtr(1),
		DetailsUserDefined: intPtr(0),
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Verify that the Audited Jobs configuration was applied to the RServer config
	assert.NotNil(t, testWorkbench.Spec.Config.RServer)
	assert.Equal(t, intPtr(1), testWorkbench.Spec.Config.RServer.AuditedJobs)
	assert.Equal(t, "/mnt/shared-storage/audited-jobs", testWorkbench.Spec.Config.RServer.AuditedJobsStoragePath)
	assert.Equal(t, "/etc/rstudio/audited-jobs-private-key.pem", testWorkbench.Spec.Config.RServer.AuditedJobsPrivateKeyPath)
	assert.Equal(t, "/etc/rstudio/audited-jobs-public-key.pem", testWorkbench.Spec.Config.RServer.AuditedJobsPublicKeyPaths)
	assert.Equal(t, intPtr(5000), testWorkbench.Spec.Config.RServer.AuditedJobsLogLimit)
	assert.Equal(t, intPtr(60), testWorkbench.Spec.Config.RServer.AuditedJobsDeletionExpiry)
	assert.Equal(t, intPtr(1), testWorkbench.Spec.Config.RServer.AuditedJobsVanillaRequired)
	assert.Equal(t, intPtr(1), testWorkbench.Spec.Config.RServer.AuditedJobsDetailsEnvironment)
	assert.Equal(t, intPtr(0), testWorkbench.Spec.Config.RServer.AuditedJobsDetailsUserDefined)
}

func TestSiteAuditedJobsPartialConfiguration(t *testing.T) {
	siteName := "audited-jobs-partial-site"
	siteNamespace := "posit-team"

	intPtr := func(i int) *int {
		return &i
	}

	site := defaultSite(siteName)
	site.Spec.Workbench.AuditedJobs = &v1beta1.AuditedJobsConfig{
		Enabled:     intPtr(1),
		StoragePath: "/mnt/shared-storage/audited-jobs",
	}

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.NotNil(t, testWorkbench.Spec.Config.RServer)
	// Set fields should be propagated
	assert.Equal(t, intPtr(1), testWorkbench.Spec.Config.RServer.AuditedJobs)
	assert.Equal(t, "/mnt/shared-storage/audited-jobs", testWorkbench.Spec.Config.RServer.AuditedJobsStoragePath)
	// Unset string fields remain empty string
	assert.Equal(t, "", testWorkbench.Spec.Config.RServer.AuditedJobsPrivateKeyPath)
	assert.Equal(t, "", testWorkbench.Spec.Config.RServer.AuditedJobsPublicKeyPaths)
	// Unset *int fields should be nil
	assert.Nil(t, testWorkbench.Spec.Config.RServer.AuditedJobsLogLimit)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.AuditedJobsDeletionExpiry)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.AuditedJobsVanillaRequired)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.AuditedJobsDetailsEnvironment)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.AuditedJobsDetailsUserDefined)
}

func TestSitePositronVersionAutoDerivesExe(t *testing.T) {
	// When PositronSettings.Version is set and Exe is unset, the controller
	// should auto-derive the exe path and populate the three
	// launcher-positron-init-container-* keys on rserver.conf.
	siteName := "positron-version-derive"
	siteNamespace := "posit-team"

	site := defaultSite(siteName)
	site.Spec.Workbench.PositronSettings.Version = "2026.04.0-269"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	// Exe is auto-derived from the version.
	assert.NotNil(t, testWorkbench.Spec.Config.WorkbenchSessionIniConfig.Positron)
	assert.Equal(t,
		"/usr/lib/rstudio-server/bin/positron-server/2026.04.0-269/bin/positron-server",
		testWorkbench.Spec.Config.WorkbenchSessionIniConfig.Positron.Exe,
	)

	// rserver.conf positron init-container keys are populated.
	assert.NotNil(t, testWorkbench.Spec.Config.RServer)
	if assert.NotNil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerEnabled) {
		assert.Equal(t, 1, *testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerEnabled)
	}
	if assert.NotNil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageName) {
		assert.Equal(t, "posit/workbench-positron-init", *testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageName)
	}
	if assert.NotNil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageTag) {
		assert.Equal(t, "2026.04.0-269", *testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageTag)
	}
}

func TestSitePositronVersionUserExeOverrideWins(t *testing.T) {
	// When PositronSettings.Version AND PositronSettings.Exe are both set,
	// the user-supplied Exe must win — derivePositronExe should not clobber
	// it. The three rserver.conf keys are still populated based on Version.
	siteName := "positron-version-override"
	siteNamespace := "posit-team"

	site := defaultSite(siteName)
	site.Spec.Workbench.PositronSettings.Version = "2026.04.0-269"
	site.Spec.Workbench.PositronSettings.Exe = "/opt/custom/positron-server"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.NotNil(t, testWorkbench.Spec.Config.WorkbenchSessionIniConfig.Positron)
	assert.Equal(t,
		"/opt/custom/positron-server",
		testWorkbench.Spec.Config.WorkbenchSessionIniConfig.Positron.Exe,
	)

	// Init-container keys still populated based on Version.
	assert.NotNil(t, testWorkbench.Spec.Config.RServer)
	if assert.NotNil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerEnabled) {
		assert.Equal(t, 1, *testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerEnabled)
	}
	if assert.NotNil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageName) {
		assert.Equal(t, "posit/workbench-positron-init", *testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageName)
	}
	if assert.NotNil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageTag) {
		assert.Equal(t, "2026.04.0-269", *testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageTag)
	}
}

func TestSitePositronVersionUnsetLeavesExeEmpty(t *testing.T) {
	// When PositronSettings.Version is unset and Exe is unset, Exe stays
	// empty and the three positron init-container pointer fields are nil
	// so omitempty drops them from rserver.conf entirely. Workbench's
	// strict program-options parser rejects unknown keys, so we must not
	// emit them when no Positron Pro version is pinned.
	siteName := "positron-version-unset"
	siteNamespace := "posit-team"

	site := defaultSite(siteName)
	// PositronSettings left untouched — Version="" and Exe="".

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)

	assert.NotNil(t, testWorkbench.Spec.Config.WorkbenchSessionIniConfig.Positron)
	assert.Equal(t, "", testWorkbench.Spec.Config.WorkbenchSessionIniConfig.Positron.Exe)

	assert.NotNil(t, testWorkbench.Spec.Config.RServer)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerEnabled)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageName)
	assert.Nil(t, testWorkbench.Spec.Config.RServer.LauncherPositronInitContainerImageTag)
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

func TestSiteReconciler_RegisterOnFirstLoginPropagation(t *testing.T) {
	siteName := "register-on-first-login"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Connect.RegisterOnFirstLogin = ptr.To(true)

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	require.NotNil(t, testConnect.Spec.RegisterOnFirstLogin)
	assert.True(t, *testConnect.Spec.RegisterOnFirstLogin)
}

func TestSiteReconciler_RegisterOnFirstLoginDefaultNil(t *testing.T) {
	siteName := "register-on-first-login-default"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Nil(t, testConnect.Spec.RegisterOnFirstLogin)
}

func TestSiteReconciler_BaseDomainNotSet(t *testing.T) {
	siteName := "base-domain-not-set"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Domain = "example.com"
	site.Spec.Connect.DomainPrefix = "connect"
	site.Spec.Workbench.DomainPrefix = "workbench"
	site.Spec.PackageManager.DomainPrefix = "packagemanager"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify default behavior is preserved when BaseDomain is not set
	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, "connect.example.com", testConnect.Spec.Url)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, "workbench.example.com", testWorkbench.Spec.Url)
	// Verify DefaultRSConnectServer uses site domain when BaseDomain not set
	assert.Equal(t, "https://connect.example.com", testWorkbench.Spec.Config.RSession.DefaultRSConnectServer)

	testPackageManager := getPackageManager(t, cli, siteNamespace, siteName)
	assert.Equal(t, "packagemanager.example.com", testPackageManager.Spec.Url)
}

func TestSiteReconciler_BaseDomainConnectOnly(t *testing.T) {
	siteName := "base-domain-connect-only"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Domain = "example.com"
	site.Spec.Connect.DomainPrefix = "connect"
	site.Spec.Connect.BaseDomain = "connect-custom.com"
	site.Spec.Workbench.DomainPrefix = "workbench"
	site.Spec.PackageManager.DomainPrefix = "packagemanager"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify Connect uses custom BaseDomain
	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, "connect.connect-custom.com", testConnect.Spec.Url)

	// Verify Workbench uses site domain (not custom)
	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, "workbench.example.com", testWorkbench.Spec.Url)
	// Verify DefaultRSConnectServer uses Connect's BaseDomain
	assert.Equal(t, "https://connect.connect-custom.com", testWorkbench.Spec.Config.RSession.DefaultRSConnectServer)

	// Verify PackageManager uses site domain (not custom)
	testPackageManager := getPackageManager(t, cli, siteNamespace, siteName)
	assert.Equal(t, "packagemanager.example.com", testPackageManager.Spec.Url)
}

func TestSiteReconciler_BaseDomainAllProducts(t *testing.T) {
	siteName := "base-domain-all-products"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Domain = "example.com"
	site.Spec.Connect.DomainPrefix = "connect"
	site.Spec.Connect.BaseDomain = "connect-domain.com"
	site.Spec.Workbench.DomainPrefix = "workbench"
	site.Spec.Workbench.BaseDomain = "workbench-domain.com"
	site.Spec.PackageManager.DomainPrefix = "packagemanager"
	site.Spec.PackageManager.BaseDomain = "pm-domain.com"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify all products use their custom BaseDomains
	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, "connect.connect-domain.com", testConnect.Spec.Url)

	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, "workbench.workbench-domain.com", testWorkbench.Spec.Url)
	// Verify DefaultRSConnectServer uses Connect's BaseDomain
	assert.Equal(t, "https://connect.connect-domain.com", testWorkbench.Spec.Config.RSession.DefaultRSConnectServer)

	testPackageManager := getPackageManager(t, cli, siteNamespace, siteName)
	assert.Equal(t, "packagemanager.pm-domain.com", testPackageManager.Spec.Url)
}

func TestSiteReconciler_BaseDomainWithCustomPrefix(t *testing.T) {
	siteName := "base-domain-custom-prefix"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Domain = "example.com"
	site.Spec.Connect.DomainPrefix = "rsc"
	site.Spec.Connect.BaseDomain = "custom-domain.com"
	site.Spec.Workbench.DomainPrefix = "workbench"

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.Nil(t, err)

	// Verify custom prefix is preserved with BaseDomain
	testConnect := getConnect(t, cli, siteNamespace, siteName)
	assert.Equal(t, "rsc.custom-domain.com", testConnect.Spec.Url)

	// Verify Workbench's DefaultRSConnectServer uses custom prefix and BaseDomain
	testWorkbench := getWorkbench(t, cli, siteNamespace, siteName)
	assert.Equal(t, "https://rsc.custom-domain.com", testWorkbench.Spec.Config.RSession.DefaultRSConnectServer)
}

// TestSiteConnectDisableNeverEnabled verifies that setting enabled=false when Connect was
// never enabled is a no-op: no Connect CR is created.
func TestSiteConnectDisableNeverEnabled(t *testing.T) {
	siteName := "never-enabled-connect"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	enabled := false
	site.Spec.Connect.Enabled = &enabled

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Connect CR should NOT exist — disable with no prior enablement is a no-op
	connect := &v1beta1.Connect{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.Error(t, err, "expected Connect CR to not exist when disabled without ever being enabled")
}

// TestSiteConnectSuspendAfterEnable verifies that setting enabled=false after Connect was running
// suspends the Connect CR (Suspended=true) rather than deleting it, preserving data.
// It also verifies that re-enabling clears Suspended and restores full reconciliation,
// confirming that reconcileConnect's full spec replace via controllerutil.CreateOrUpdate
// correctly overwrites Suspended=true back to nil.
func TestSiteConnectSuspendAfterEnable(t *testing.T) {
	siteName := "suspend-connect"
	siteNamespace := "posit-team"

	// Share a single fake environment across all reconcile passes.
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: Connect enabled (default)
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	connect := &v1beta1.Connect{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.NoError(t, err, "Connect CR should exist after first reconcile")
	assert.Nil(t, connect.Spec.Suspended)

	// Pass 2: disable Connect without teardown
	enabled := false
	site.Spec.Connect.Enabled = &enabled
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.NoError(t, err, "Connect CR should still exist when disabled without teardown")
	assert.NotNil(t, connect.Spec.Suspended)
	assert.True(t, *connect.Spec.Suspended)

	// Pass 3: re-enable Connect — reconcileConnect does a full spec replace via
	// controllerutil.CreateOrUpdate, so Suspended must be cleared back to nil.
	site.Spec.Connect.Enabled = nil
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.NoError(t, err, "Connect CR should still exist after re-enable")
	assert.Nil(t, connect.Spec.Suspended, "Suspended should be cleared after re-enable")
}

// TestSiteConnectTeardown verifies that setting enabled=false + teardown=true causes the
// Connect CR to be deleted (triggering the destructive finalizer path).
// The CR must be pre-created to confirm it is actually deleted (not just absent from the start).
func TestSiteConnectTeardown(t *testing.T) {
	siteName := "teardown-connect"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: establish a running Connect CR
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	connect := &v1beta1.Connect{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.NoError(t, err, "Connect CR should exist before teardown")

	// Pass 2: teardown
	enabled := false
	teardown := true
	site.Spec.Connect.Enabled = &enabled
	site.Spec.Connect.Teardown = &teardown
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	// Connect CR should NOT exist after teardown
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.Error(t, err, "Connect CR should not exist after teardown=true")
}

// TestSiteWorkbenchDisableNeverEnabled verifies that setting enabled=false when Workbench was
// never enabled is a no-op: no Workbench CR is created.
func TestSiteWorkbenchDisableNeverEnabled(t *testing.T) {
	siteName := "never-enabled-workbench"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	enabled := false
	site.Spec.Workbench.Enabled = &enabled

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Workbench CR should NOT exist — disable with no prior enablement is a no-op
	workbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.Error(t, err, "expected Workbench CR to not exist when disabled without ever being enabled")
}

// TestSiteWorkbenchSuspendAfterEnable verifies that setting enabled=false after Workbench was running
// suspends the Workbench CR (Suspended=true) rather than deleting it, preserving data.
// It also verifies that re-enabling clears Suspended and restores full reconciliation.
func TestSiteWorkbenchSuspendAfterEnable(t *testing.T) {
	siteName := "suspend-workbench"
	siteNamespace := "posit-team"

	// Share a single fake environment across all reconcile passes.
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: Workbench enabled (default)
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	workbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.NoError(t, err, "Workbench CR should exist after first reconcile")
	assert.Nil(t, workbench.Spec.Suspended)

	// Pass 2: disable Workbench without teardown — Suspended should be true
	enabled := false
	site.Spec.Workbench.Enabled = &enabled
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.NoError(t, err, "Workbench CR should still exist when disabled without teardown")
	assert.NotNil(t, workbench.Spec.Suspended)
	assert.True(t, *workbench.Spec.Suspended)

	// Pass 3: re-enable Workbench — Suspended should be cleared
	site.Spec.Workbench.Enabled = nil
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.NoError(t, err, "Workbench CR should still exist after re-enable")
	assert.Nil(t, workbench.Spec.Suspended, "Suspended should be cleared after re-enable")
}

// TestSiteWorkbenchTeardown verifies that setting enabled=false + teardown=true causes the
// Workbench CR to be deleted (triggering the destructive finalizer path).
func TestSiteWorkbenchTeardown(t *testing.T) {
	siteName := "teardown-workbench"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: establish a running Workbench CR
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	workbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.NoError(t, err, "Workbench CR should exist before teardown")

	// Pass 2: teardown
	enabled := false
	teardown := true
	site.Spec.Workbench.Enabled = &enabled
	site.Spec.Workbench.Teardown = &teardown
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	// Workbench CR should NOT exist after teardown
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.Error(t, err, "Workbench CR should not exist after teardown=true")
}

// TestSitePackageManagerDisableNeverEnabled verifies that setting enabled=false when Package Manager was
// never enabled is a no-op: no PackageManager CR is created.
func TestSitePackageManagerDisableNeverEnabled(t *testing.T) {
	siteName := "never-enabled-pm"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	enabled := false
	site.Spec.PackageManager.Enabled = &enabled

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// PackageManager CR should NOT exist — disable with no prior enablement is a no-op
	pm := &v1beta1.PackageManager{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.Error(t, err, "expected PackageManager CR to not exist when disabled without ever being enabled")
}

// TestSitePackageManagerSuspendAfterEnable verifies that setting enabled=false after Package Manager was running
// suspends the PackageManager CR (Suspended=true) rather than deleting it, preserving data.
// It also verifies that re-enabling clears Suspended and restores full reconciliation.
func TestSitePackageManagerSuspendAfterEnable(t *testing.T) {
	siteName := "suspend-pm"
	siteNamespace := "posit-team"

	// Share a single fake environment across all reconcile passes.
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: PackageManager enabled (default)
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	pm := &v1beta1.PackageManager{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.NoError(t, err, "PackageManager CR should exist after first reconcile")
	assert.Nil(t, pm.Spec.Suspended)

	// Pass 2: disable PackageManager without teardown — Suspended should be true
	enabled := false
	site.Spec.PackageManager.Enabled = &enabled
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.NoError(t, err, "PackageManager CR should still exist when disabled without teardown")
	assert.NotNil(t, pm.Spec.Suspended)
	assert.True(t, *pm.Spec.Suspended)

	// Pass 3: re-enable PackageManager — Suspended should be cleared
	site.Spec.PackageManager.Enabled = nil
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.NoError(t, err, "PackageManager CR should still exist after re-enable")
	assert.Nil(t, pm.Spec.Suspended, "Suspended should be cleared after re-enable")
}

// TestSitePackageManagerTeardown verifies that setting enabled=false + teardown=true causes the
// PackageManager CR to be deleted (triggering the destructive finalizer path).
func TestSitePackageManagerTeardown(t *testing.T) {
	siteName := "teardown-pm"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: establish a running PackageManager CR
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	pm := &v1beta1.PackageManager{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.NoError(t, err, "PackageManager CR should exist before teardown")

	// Pass 2: teardown
	enabled := false
	teardown := true
	site.Spec.PackageManager.Enabled = &enabled
	site.Spec.PackageManager.Teardown = &teardown
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	// PackageManager CR should NOT exist after teardown
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.Error(t, err, "PackageManager CR should not exist after teardown=true")
}

// TestSiteChronicleDisableNeverEnabled verifies that setting enabled=false when Chronicle was
// never enabled is a no-op: no Chronicle CR is created.
func TestSiteChronicleDisableNeverEnabled(t *testing.T) {
	siteName := "never-enabled-chronicle"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	enabled := false
	site.Spec.Chronicle.Enabled = &enabled

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Chronicle CR should NOT exist — disable with no prior enablement is a no-op
	chronicle := &v1beta1.Chronicle{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.Error(t, err, "expected Chronicle CR to not exist when disabled without ever being enabled")
}

// TestSiteChronicleSuspendAfterEnable verifies that setting enabled=false after Chronicle was running
// suspends the Chronicle CR (Suspended=true) rather than deleting it, preserving data.
// It also verifies that re-enabling clears Suspended and restores full reconciliation.
func TestSiteChronicleSuspendAfterEnable(t *testing.T) {
	siteName := "suspend-chronicle"
	siteNamespace := "posit-team"

	// Share a single fake environment across all reconcile passes.
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: Chronicle enabled (default)
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	chronicle := &v1beta1.Chronicle{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.NoError(t, err, "Chronicle CR should exist after first reconcile")
	assert.Nil(t, chronicle.Spec.Suspended)

	// Pass 2: disable Chronicle without teardown — Suspended should be true
	enabled := false
	site.Spec.Chronicle.Enabled = &enabled
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.NoError(t, err, "Chronicle CR should still exist when disabled without teardown")
	assert.NotNil(t, chronicle.Spec.Suspended)
	assert.True(t, *chronicle.Spec.Suspended)

	// Pass 3: re-enable Chronicle — Suspended should be cleared
	site.Spec.Chronicle.Enabled = nil
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.NoError(t, err, "Chronicle CR should still exist after re-enable")
	assert.Nil(t, chronicle.Spec.Suspended, "Suspended should be cleared after re-enable")
}

// TestSiteChronicleTeardown verifies that setting enabled=false + teardown=true causes the
// Chronicle CR to be deleted (triggering the destructive finalizer path).
func TestSiteChronicleTeardown(t *testing.T) {
	siteName := "teardown-chronicle"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: establish a running Chronicle CR
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	chronicle := &v1beta1.Chronicle{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.NoError(t, err, "Chronicle CR should exist before teardown")

	// Pass 2: teardown
	enabled := false
	teardown := true
	site.Spec.Chronicle.Enabled = &enabled
	site.Spec.Chronicle.Teardown = &teardown
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	// Chronicle CR should NOT exist after teardown
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.Error(t, err, "Chronicle CR should not exist after teardown=true")
}

// TestSiteTeardownIgnoredWhileEnabled verifies that setting teardown=true while a product is
// still enabled (or defaults to enabled) is a no-op: no CRs are deleted.
// This guards the warning-path guard in reconcileResources against accidental removal.
func TestSiteTeardownIgnoredWhileEnabled(t *testing.T) {
	siteName := "teardown-while-enabled"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pass 1: establish running CRs for all three products
	site := defaultSite(siteName)
	_, err := rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	workbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.NoError(t, err, "Workbench CR should exist after first reconcile")

	pm := &v1beta1.PackageManager{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.NoError(t, err, "PackageManager CR should exist after first reconcile")

	chronicle := &v1beta1.Chronicle{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.NoError(t, err, "Chronicle CR should exist after first reconcile")

	// Pass 2: set teardown=true but leave enabled=true (default) — should be a no-op
	teardown := true
	site.Spec.Workbench.Teardown = &teardown
	site.Spec.PackageManager.Teardown = &teardown
	site.Spec.Chronicle.Teardown = &teardown
	_, err = rec.reconcileResources(context.TODO(), req, site)
	assert.NoError(t, err)

	// All CRs should still exist — teardown while enabled is a no-op
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.NoError(t, err, "Workbench CR should still exist: teardown has no effect while enabled=true")

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.NoError(t, err, "PackageManager CR should still exist: teardown has no effect while enabled=true")

	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, chronicle)
	assert.NoError(t, err, "Chronicle CR should still exist: teardown has no effect while enabled=true")
}

// TestSiteReadyWithDisabledProducts verifies that a Site can be Ready when all
// products are explicitly disabled (enabled: false), since disabled products don't
// create CRs and therefore shouldn't block site readiness.
func TestSiteReadyWithDisabledProducts(t *testing.T) {
	siteName := "ready-with-disabled-products"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	// Disable all products so none create CRs that would block readiness
	connectEnabled := false
	workbenchEnabled := false
	pmEnabled := false
	chronicleEnabled := false
	flightdeckEnabled := false
	site.Spec.Connect.Enabled = &connectEnabled
	site.Spec.Workbench.Enabled = &workbenchEnabled
	site.Spec.PackageManager.Enabled = &pmEnabled
	site.Spec.Chronicle.Enabled = &chronicleEnabled
	site.Spec.Flightdeck.Enabled = &flightdeckEnabled

	// Use shared fake client to run multiple reconcile passes
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Create the Site
	err := cli.Create(context.TODO(), site)
	assert.NoError(t, err)

	// Run initial reconcile
	_, err = rec.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Fetch the Site to check its status
	fetchedSite := &v1beta1.Site{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fetchedSite)
	assert.NoError(t, err)

	// Verify per-product readiness for disabled products
	assert.True(t, fetchedSite.Status.ConnectReady, "ConnectReady should be true when Connect is disabled")
	assert.True(t, fetchedSite.Status.WorkbenchReady, "WorkbenchReady should be true when Workbench is disabled")
	assert.True(t, fetchedSite.Status.PackageManagerReady, "PackageManagerReady should be true when PackageManager is disabled")

	// Verify aggregate site readiness - the main goal of the fix
	assert.True(t, status.IsReady(fetchedSite.Status.Conditions), "site should be Ready when all required products are disabled")

	// Verify CRs do NOT exist for disabled products
	connect := &v1beta1.Connect{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, connect)
	assert.Error(t, err, "Connect CR should not exist when disabled")

	workbench := &v1beta1.Workbench{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, workbench)
	assert.Error(t, err, "Workbench CR should not exist when disabled")

	pm := &v1beta1.PackageManager{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, pm)
	assert.Error(t, err, "PackageManager CR should not exist when disabled")
}

// TestSiteNilEnabledMissingCR is a regression test verifying that when Enabled=nil (the default)
// and the product CR does not exist, the product is NOT treated as ready. This guards against
// future refactors that might accidentally collapse the nil and false cases.
func TestSiteNilEnabledMissingCR(t *testing.T) {
	siteName := "nil-enabled-missing-cr"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// site with Connect.Enabled = nil (default: not set)
	site := defaultSite(siteName)
	// Connect.Enabled is nil — product is expected but CR does not yet exist

	err := rec.aggregateChildStatus(context.TODO(), req, site)
	assert.NoError(t, err)

	assert.False(t, site.Status.ConnectReady, "ConnectReady should be false when Enabled=nil and Connect CR does not exist")
	assert.False(t, site.Status.WorkbenchReady, "WorkbenchReady should be false when Enabled=nil and Workbench CR does not exist")
	assert.False(t, site.Status.PackageManagerReady, "PackageManagerReady should be false when Enabled=nil and PackageManager CR does not exist")
}

// TestSiteReadyWithDisabledFlightdeck verifies that FlightdeckReady=true when Flightdeck is
// explicitly disabled (Enabled=false), and that the site is Ready when all products including
// Chronicle are also disabled.
func TestSiteReadyWithDisabledFlightdeck(t *testing.T) {
	siteName := "disabled-flightdeck"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	// Disable all products so none create CRs that would block readiness
	connectEnabled := false
	workbenchEnabled := false
	pmEnabled := false
	chronicleEnabled := false
	flightdeckEnabled := false
	site.Spec.Connect.Enabled = &connectEnabled
	site.Spec.Workbench.Enabled = &workbenchEnabled
	site.Spec.PackageManager.Enabled = &pmEnabled
	site.Spec.Chronicle.Enabled = &chronicleEnabled
	site.Spec.Flightdeck.Enabled = &flightdeckEnabled

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	err := cli.Create(context.TODO(), site)
	assert.NoError(t, err)

	_, err = rec.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	fetchedSite := &v1beta1.Site{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fetchedSite)
	assert.NoError(t, err)

	assert.True(t, fetchedSite.Status.FlightdeckReady, "FlightdeckReady should be true when Flightdeck is disabled")
	assert.True(t, status.IsReady(fetchedSite.Status.Conditions), "site should be Ready when all products are disabled")
}

// errorGetClient wraps a client.Client and injects a fixed error for Get calls on a specific type.
type errorGetClient struct {
	client.Client
	errForType func(obj client.Object) error
}

func (c *errorGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.errForType != nil {
		if err := c.errForType(obj); err != nil {
			return err
		}
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// TestAggregateChildStatusContinuesOnTransientError verifies that when one product returns a
// transient API error, aggregateChildStatus still evaluates all remaining products and returns
// the error at the end (rather than returning early with stale status for the other products).
func TestAggregateChildStatusContinuesOnTransientError(t *testing.T) {
	siteName := "transient-error-site"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)

	transientErr := fmt.Errorf("transient server error")

	fakeClient := localtest.FakeTestEnv{}
	baseCli, scheme, log := fakeClient.Start(loadSchemes)

	// Inject a transient error for Connect Get calls only
	errCli := &errorGetClient{
		Client: baseCli,
		errForType: func(obj client.Object) error {
			if _, ok := obj.(*v1beta1.Connect); ok {
				return transientErr
			}
			return nil
		},
	}

	rec := SiteReconciler{Client: errCli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	err := rec.aggregateChildStatus(context.TODO(), req, site)

	// Error should be propagated
	assert.Error(t, err, "transient API error should be returned")
	assert.ErrorContains(t, err, "fetching Connect for status aggregation")

	// All products should have been evaluated (not left stale): remaining products have no CRs
	// so they fall into the NotFound path and are set to false (Enabled=nil means expected but missing).
	assert.False(t, site.Status.ConnectReady, "ConnectReady should be false on transient error")
	assert.False(t, site.Status.WorkbenchReady, "WorkbenchReady should be false when CR missing")
	assert.False(t, site.Status.PackageManagerReady, "PackageManagerReady should be false when CR missing")
}

// TestSiteOptionalComponentsNilEnabledNoCR verifies that Chronicle and Flightdeck with Enabled=nil
// and no CR present are treated as not ready (Enabled=nil means enabled via checkBool,
// so the CR is expected but missing → not ready yet).
func TestSiteOptionalComponentsNilEnabledNoCR(t *testing.T) {
	siteName := "optional-nil-no-cr"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Enabled=nil (default) — no Chronicle or Flightdeck CRs pre-created
	site := defaultSite(siteName)
	// Chronicle.Enabled and Flightdeck.Enabled are nil by default

	err := rec.aggregateChildStatus(context.TODO(), req, site)
	assert.NoError(t, err)

	assert.False(t, site.Status.ChronicleReady, "ChronicleReady should be false when Enabled=nil and no CR exists (CR expected but missing)")
	assert.False(t, site.Status.FlightdeckReady, "FlightdeckReady should be false when Enabled=nil and no CR exists (CR expected but missing)")
}

// TestSiteOptionalComponentsNilEnabledWithCR verifies that when Enabled=nil but a CR already
// exists (e.g., mid-teardown after disabling), readiness is derived from the CR conditions rather
// than unconditionally set to true.
func TestSiteOptionalComponentsNilEnabledWithCR(t *testing.T) {
	siteName := "optional-nil-with-cr"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pre-create Chronicle CR (not ready — no Ready condition set)
	chronicle := &v1beta1.Chronicle{
		ObjectMeta: metav1.ObjectMeta{Namespace: siteNamespace, Name: siteName},
	}
	err := cli.Create(context.TODO(), chronicle)
	require.NoError(t, err)

	// Pre-create Flightdeck CR (not ready — no Ready condition set)
	flightdeck := &v1beta1.Flightdeck{
		ObjectMeta: metav1.ObjectMeta{Namespace: siteNamespace, Name: siteName},
	}
	err = cli.Create(context.TODO(), flightdeck)
	require.NoError(t, err)

	// Enabled=nil — CRs exist (simulating transition/teardown)
	site := defaultSite(siteName)

	err = rec.aggregateChildStatus(context.TODO(), req, site)
	assert.NoError(t, err)

	// CRs exist but have no Ready condition → IsReady returns false
	assert.False(t, site.Status.ChronicleReady, "ChronicleReady should reflect CR conditions, not be unconditionally true when CR exists")
	assert.False(t, site.Status.FlightdeckReady, "FlightdeckReady should reflect CR conditions, not be unconditionally true when CR exists")
}

// TestAggregateChildStatusDisabledWithExistingCR verifies that when a product is explicitly
// disabled (Enabled=false) but the CR still exists (e.g. suspended), aggregateChildStatus
// treats it as ready from the Site's perspective.
func TestAggregateChildStatusDisabledWithExistingCR(t *testing.T) {
	siteName := "disabled-with-cr"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Pre-create Connect CR with suspended status (simulates disabled state)
	connect := &v1beta1.Connect{
		ObjectMeta: metav1.ObjectMeta{Namespace: siteNamespace, Name: siteName},
	}
	require.NoError(t, cli.Create(context.TODO(), connect))
	status.SetReady(&connect.Status.Conditions, 0, metav1.ConditionFalse, status.ReasonSuspended, "Product is suspended")
	require.NoError(t, cli.Status().Update(context.TODO(), connect))

	// Pre-create Chronicle CR with suspended status
	chronicle := &v1beta1.Chronicle{
		ObjectMeta: metav1.ObjectMeta{Namespace: siteNamespace, Name: siteName},
	}
	require.NoError(t, cli.Create(context.TODO(), chronicle))
	status.SetReady(&chronicle.Status.Conditions, 0, metav1.ConditionFalse, status.ReasonSuspended, "Product is suspended")
	require.NoError(t, cli.Status().Update(context.TODO(), chronicle))

	site := defaultSite(siteName)
	// Explicitly disable Connect and Chronicle
	site.Spec.Connect.Enabled = ptr.To(false)
	site.Spec.Chronicle.Enabled = ptr.To(false)

	err := rec.aggregateChildStatus(context.TODO(), req, site)
	assert.NoError(t, err)

	// Disabled products with existing CRs should be treated as ready
	assert.True(t, site.Status.ConnectReady, "ConnectReady should be true when explicitly disabled, even if CR exists")
	assert.True(t, site.Status.ChronicleReady, "ChronicleReady should be true when explicitly disabled, even if CR exists")

	// Products with Enabled=nil (default, not disabled) and no CR should be not ready
	assert.False(t, site.Status.WorkbenchReady, "WorkbenchReady should be false when Enabled=nil and no CR")
	assert.False(t, site.Status.PackageManagerReady, "PackageManagerReady should be false when Enabled=nil and no CR")
}

// TestSiteFlightdeckDisableReenableCycle verifies that Flightdeck CR is deleted when disabled
// and recreated when re-enabled.
func TestSiteFlightdeckDisableReenableCycle(t *testing.T) {
	siteName := "flightdeck-cycle"
	siteNamespace := "posit-team"

	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)
	rec := SiteReconciler{Client: cli, Scheme: scheme, Log: log}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: siteNamespace, Name: siteName}}

	// Create site with Flightdeck enabled (default)
	site := defaultSite(siteName)
	require.NoError(t, cli.Create(context.TODO(), site))

	// First reconcile: Flightdeck CR should be created
	_, err := rec.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	fd := &v1beta1.Flightdeck{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.NoError(t, err, "Flightdeck CR should exist after initial reconcile")

	// Disable Flightdeck
	fetchedSite := &v1beta1.Site{}
	require.NoError(t, cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fetchedSite))
	fetchedSite.Spec.Flightdeck.Enabled = ptr.To(false)
	require.NoError(t, cli.Update(context.TODO(), fetchedSite))

	// Reconcile with Flightdeck disabled
	_, err = rec.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Flightdeck CR should be deleted
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.Error(t, err, "Flightdeck CR should not exist after disabling")

	// Verify FlightdeckReady is true for disabled product
	fetchedSite = &v1beta1.Site{}
	require.NoError(t, cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fetchedSite))
	assert.True(t, fetchedSite.Status.FlightdeckReady, "FlightdeckReady should be true when Flightdeck is disabled")

	// Re-enable Flightdeck
	fetchedSite.Spec.Flightdeck.Enabled = nil
	require.NoError(t, cli.Update(context.TODO(), fetchedSite))

	// Reconcile with Flightdeck re-enabled
	_, err = rec.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	// Flightdeck CR should be recreated
	fd = &v1beta1.Flightdeck{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.NoError(t, err, "Flightdeck CR should be recreated after re-enabling")
}
