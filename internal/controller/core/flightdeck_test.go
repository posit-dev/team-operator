package core

import (
	"context"
	"testing"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func defaultFlightdeck(name, namespace string) *v1beta1.Flightdeck {
	return &v1beta1.Flightdeck{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Flightdeck",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "flightdeck-test-uid",
		},
		Spec: v1beta1.FlightdeckSpec{
			SiteName:        "test-site",
			Image:           "ghcr.io/rstudio/flightdeck:test",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Port:            8080,
			Replicas:        1,
			FeatureEnabler: v1beta1.FeatureEnablerConfig{
				ShowConfig:  true,
				ShowAcademy: false,
			},
			Domain:             "test.posit.team",
			IngressClass:       "nginx",
			IngressAnnotations: map[string]string{"test": "annotation"},
			ImagePullSecrets:   []string{"test-secret"},
		},
	}
}

func runFakeFlightdeckReconciler(t *testing.T, namespace, name string, fd *v1beta1.Flightdeck) (client.WithWatch, ctrl.Result, error) {
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)

	// Create the Flightdeck resource first
	err := cli.Create(context.TODO(), fd)
	require.NoError(t, err)

	rec := FlightdeckReconciler{
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

	res, err := rec.Reconcile(context.TODO(), req)

	return cli, res, err
}

func TestFlightdeckReconciler_CreatesAllResources(t *testing.T) {
	fdName := "test-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	componentName := fd.ComponentName()

	// Verify ServiceAccount is created
	sa := &corev1.ServiceAccount{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: componentName, Namespace: fdNamespace}, sa)
	assert.NoError(t, err)
	assert.Equal(t, componentName, sa.Name)
	assert.Len(t, sa.ImagePullSecrets, 1)
	assert.Equal(t, "test-secret", sa.ImagePullSecrets[0].Name)

	// Verify Role is created
	role := &rbacv1.Role{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: componentName + "-role", Namespace: fdNamespace}, role)
	assert.NoError(t, err)
	assert.Len(t, role.Rules, 1)
	assert.Contains(t, role.Rules[0].Resources, "sites")

	// Verify RoleBinding is created
	rb := &rbacv1.RoleBinding{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: componentName + "-rolebinding", Namespace: fdNamespace}, rb)
	assert.NoError(t, err)
	assert.Equal(t, componentName, rb.Subjects[0].Name)

	// Verify Deployment is created
	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: componentName, Namespace: fdNamespace}, dep)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), *dep.Spec.Replicas)
	assert.Equal(t, "ghcr.io/rstudio/flightdeck:test", dep.Spec.Template.Spec.Containers[0].Image)

	// Verify Service is created
	svc := &corev1.Service{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: componentName, Namespace: fdNamespace}, svc)
	assert.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Equal(t, int32(80), svc.Spec.Ports[0].Port)

	// Verify Ingress is created
	ing := &networkingv1.Ingress{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: componentName, Namespace: fdNamespace}, ing)
	assert.NoError(t, err)
	assert.Equal(t, "test.posit.team", ing.Spec.Rules[0].Host)
	assert.Equal(t, "/", ing.Spec.Rules[0].HTTP.Paths[0].Path)
	assert.Equal(t, "nginx", *ing.Spec.IngressClassName)
}

func TestFlightdeckReconciler_DeploymentHasCorrectEnvVars(t *testing.T) {
	fdName := "env-vars-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)
	fd.Spec.FeatureEnabler.ShowConfig = true
	fd.Spec.FeatureEnabler.ShowAcademy = true

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	envVars := dep.Spec.Template.Spec.Containers[0].Env
	envMap := make(map[string]string)
	for _, env := range envVars {
		envMap[env.Name] = env.Value
	}

	assert.Equal(t, "test-site", envMap["SITE_NAME"])
	assert.Equal(t, "true", envMap["SHOW_CONFIG"])
	assert.Equal(t, "true", envMap["SHOW_ACADEMY"])
}

func TestFlightdeckReconciler_DeploymentHasResourceLimits(t *testing.T) {
	fdName := "resources-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	resources := dep.Spec.Template.Spec.Containers[0].Resources

	// Verify resource requests
	cpuRequest := resources.Requests[corev1.ResourceCPU]
	memRequest := resources.Requests[corev1.ResourceMemory]
	assert.Equal(t, resource.MustParse("50m"), cpuRequest)
	assert.Equal(t, resource.MustParse("64Mi"), memRequest)

	// Verify resource limits
	cpuLimit := resources.Limits[corev1.ResourceCPU]
	memLimit := resources.Limits[corev1.ResourceMemory]
	assert.Equal(t, resource.MustParse("200m"), cpuLimit)
	assert.Equal(t, resource.MustParse("256Mi"), memLimit)
}

func TestFlightdeckReconciler_DeploymentHasProbes(t *testing.T) {
	fdName := "probes-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	container := dep.Spec.Template.Spec.Containers[0]

	// Verify LivenessProbe exists
	assert.NotNil(t, container.LivenessProbe)
	assert.Equal(t, "/", container.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(10), container.LivenessProbe.InitialDelaySeconds)

	// Verify ReadinessProbe exists
	assert.NotNil(t, container.ReadinessProbe)
	assert.Equal(t, "/", container.ReadinessProbe.HTTPGet.Path)
	assert.Equal(t, int32(3), container.ReadinessProbe.InitialDelaySeconds)
}

func TestFlightdeckReconciler_DeploymentHasSecurityContext(t *testing.T) {
	fdName := "security-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	secCtx := dep.Spec.Template.Spec.Containers[0].SecurityContext

	assert.NotNil(t, secCtx)
	assert.Equal(t, int64(999), *secCtx.RunAsUser)
	assert.True(t, *secCtx.RunAsNonRoot)
	assert.False(t, *secCtx.AllowPrivilegeEscalation)
	assert.Contains(t, secCtx.Capabilities.Drop, corev1.Capability("ALL"))
}

func TestFlightdeckReconciler_CustomReplicas(t *testing.T) {
	fdName := "replicas-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)
	fd.Spec.Replicas = 3

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	assert.Equal(t, int32(3), *dep.Spec.Replicas)
}

func TestFlightdeckReconciler_CustomPort(t *testing.T) {
	fdName := "port-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)
	fd.Spec.Port = 9090

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	assert.Equal(t, int32(9090), dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
}

func TestFlightdeckReconciler_IngressAnnotations(t *testing.T) {
	fdName := "annotations-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)
	fd.Spec.IngressAnnotations = map[string]string{
		"nginx.ingress.kubernetes.io/ssl-redirect": "true",
		"cert-manager.io/cluster-issuer":           "letsencrypt",
	}

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	ing := &networkingv1.Ingress{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, ing)
	assert.NoError(t, err)

	assert.Equal(t, "true", ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"])
	assert.Equal(t, "letsencrypt", ing.Annotations["cert-manager.io/cluster-issuer"])
}

func TestFlightdeckReconciler_ServiceUsesCorrectSelector(t *testing.T) {
	fdName := "selector-flightdeck"
	fdNamespace := "posit-team"
	fd := defaultFlightdeck(fdName, fdNamespace)

	cli, _, err := runFakeFlightdeckReconciler(t, fdNamespace, fdName, fd)
	assert.NoError(t, err)

	svc := &corev1.Service{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, svc)
	assert.NoError(t, err)

	dep := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: fd.ComponentName(), Namespace: fdNamespace}, dep)
	assert.NoError(t, err)

	// Service selector should match deployment selector
	assert.Equal(t, dep.Spec.Selector.MatchLabels, svc.Spec.Selector)
}

func TestFlightdeckReconciler_NotFoundReturnsNoError(t *testing.T) {
	fakeClient := localtest.FakeTestEnv{}
	cli, scheme, log := fakeClient.Start(loadSchemes)

	rec := FlightdeckReconciler{
		Client: cli,
		Scheme: scheme,
		Log:    log,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "posit-team",
			Name:      "nonexistent-flightdeck",
		},
	}

	_, err := rec.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
}

func TestSiteFlightdeckCreatedWithDefaultImage(t *testing.T) {
	siteName := "default-flightdeck-image"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Flightdeck.Image = "" // No image configured - should use default

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Flightdeck CRD should be created with default image
	fd := &v1beta1.Flightdeck{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.NoError(t, err)
	assert.Equal(t, "docker.io/posit/ptd-flightdeck:latest", fd.Spec.Image)
}

func TestSiteFlightdeckSkippedWhenDisabled(t *testing.T) {
	siteName := "disabled-flightdeck"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	enabled := false
	site.Spec.Flightdeck.Enabled = &enabled

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Flightdeck CRD should NOT be created when explicitly disabled
	fd := &v1beta1.Flightdeck{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.Error(t, err) // Should error because it doesn't exist
}

func TestSiteFlightdeckCreatedWithImage(t *testing.T) {
	siteName := "with-flightdeck-image"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Flightdeck.Image = "ghcr.io/rstudio/flightdeck:v1.0.0"
	site.Spec.Flightdeck.Replicas = 2
	site.Spec.Flightdeck.FeatureEnabler.ShowConfig = true
	site.Spec.Flightdeck.FeatureEnabler.ShowAcademy = true

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Flightdeck CRD should be created
	fd := &v1beta1.Flightdeck{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.NoError(t, err)

	assert.Equal(t, "ghcr.io/rstudio/flightdeck:v1.0.0", fd.Spec.Image)
	assert.Equal(t, 2, fd.Spec.Replicas)
	assert.True(t, fd.Spec.FeatureEnabler.ShowConfig)
	assert.True(t, fd.Spec.FeatureEnabler.ShowAcademy)
	assert.Equal(t, site.Spec.Domain, fd.Spec.Domain)
	assert.Equal(t, site.Spec.IngressClass, fd.Spec.IngressClass)
}

func TestSiteFlightdeckCreatedWithTagOnly(t *testing.T) {
	siteName := "tag-only-flightdeck"
	siteNamespace := "posit-team"
	site := defaultSite(siteName)
	site.Spec.Flightdeck.Image = "v1.2.3" // Just a tag

	cli, _, err := runFakeSiteReconciler(t, siteNamespace, siteName, site)
	assert.NoError(t, err)

	// Flightdeck CRD should be created with resolved image
	fd := &v1beta1.Flightdeck{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: siteName, Namespace: siteNamespace}, fd)
	assert.NoError(t, err)
	assert.Equal(t, "docker.io/posit/ptd-flightdeck:v1.2.3", fd.Spec.Image)
}

func TestResolveFlightdeckImage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns default",
			input:    "",
			expected: "docker.io/posit/ptd-flightdeck:latest",
		},
		{
			name:     "tag only is combined with default registry",
			input:    "v1.2.3",
			expected: "docker.io/posit/ptd-flightdeck:v1.2.3",
		},
		{
			name:     "latest tag",
			input:    "latest",
			expected: "docker.io/posit/ptd-flightdeck:latest",
		},
		{
			name:     "full image path is returned as-is",
			input:    "my-registry.io/custom-flightdeck:v1.0.0",
			expected: "my-registry.io/custom-flightdeck:v1.0.0",
		},
		{
			name:     "docker.io path is returned as-is",
			input:    "docker.io/other/image:tag",
			expected: "docker.io/other/image:tag",
		},
		{
			name:     "ghcr.io path is returned as-is",
			input:    "ghcr.io/rstudio/flightdeck:test",
			expected: "ghcr.io/rstudio/flightdeck:test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveFlightdeckImage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
