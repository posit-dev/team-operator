package internal_test

import (
	"context"
	"testing"

	corev1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakectrl "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newRbacTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, corev1beta1.AddToScheme(s))
	return s
}

func newTestConnect(name, namespace string) *corev1beta1.Connect {
	return &corev1beta1.Connect{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Connect",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID("test-connect-uid"),
		},
	}
}

func newRbacTestClient(t *testing.T, s *runtime.Scheme) client.Client {
	t.Helper()
	return fakectrl.NewClientBuilder().WithScheme(s).Build()
}

func newTestRequest(namespace, name string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// TestGenerateRbac_DefaultName verifies the SA is created with ComponentName when no custom name is set.
func TestGenerateRbac_DefaultName(t *testing.T) {
	ctx := context.Background()
	s := newRbacTestScheme(t)
	c := newRbacTestClient(t, s)

	connect := newTestConnect("mysite", "posit-team")
	req := newTestRequest("posit-team", "mysite")

	err := internal.GenerateRbac(ctx, c, s, req, connect)
	require.NoError(t, err)

	sa := &v1.ServiceAccount{}
	err = c.Get(ctx, client.ObjectKey{Name: connect.ComponentName(), Namespace: "posit-team"}, sa)
	require.NoError(t, err)
	require.Equal(t, connect.ComponentName(), sa.Name)
}

// TestGenerateRbac_CustomServiceAccountName verifies the SA and RoleBinding subject use the custom name.
func TestGenerateRbac_CustomServiceAccountName(t *testing.T) {
	ctx := context.Background()
	s := newRbacTestScheme(t)
	c := newRbacTestClient(t, s)

	connect := newTestConnect("mysite", "posit-team")
	connect.Spec.ServiceAccountName = "my-custom-sa"
	req := newTestRequest("posit-team", "mysite")

	err := internal.GenerateRbac(ctx, c, s, req, connect)
	require.NoError(t, err)

	// SA should be created with custom name
	sa := &v1.ServiceAccount{}
	err = c.Get(ctx, client.ObjectKey{Name: "my-custom-sa", Namespace: "posit-team"}, sa)
	require.NoError(t, err)
	require.Equal(t, "my-custom-sa", sa.Name)

	// RoleBinding subject must reference the custom SA name, not ComponentName
	rb := &rbacv1.RoleBinding{}
	err = c.Get(ctx, client.ObjectKey{Name: connect.ComponentName(), Namespace: "posit-team"}, rb)
	require.NoError(t, err)
	require.Len(t, rb.Subjects, 1)
	require.Equal(t, "my-custom-sa", rb.Subjects[0].Name)
}

// TestGenerateRbac_CustomAnnotationsSkipsIRSA verifies custom annotations are used and IRSA is skipped.
func TestGenerateRbac_CustomAnnotationsSkipsIRSA(t *testing.T) {
	ctx := context.Background()
	s := newRbacTestScheme(t)
	c := newRbacTestClient(t, s)

	connect := newTestConnect("mysite", "posit-team")
	// Set AWS fields that would normally trigger IRSA
	connect.Spec.AwsAccountId = "123456789012"
	connect.Spec.ClusterDate = "20240101"
	connect.Spec.WorkloadCompoundName = "compound-name"
	// Provide explicit custom annotations — IRSA fallback must be skipped
	connect.Spec.ServiceAccountAnnotations = map[string]string{
		"custom-key": "custom-value",
	}
	req := newTestRequest("posit-team", "mysite")

	err := internal.GenerateRbac(ctx, c, s, req, connect)
	require.NoError(t, err)

	sa := &v1.ServiceAccount{}
	err = c.Get(ctx, client.ObjectKey{Name: connect.ComponentName(), Namespace: "posit-team"}, sa)
	require.NoError(t, err)
	require.Equal(t, "custom-value", sa.Annotations["custom-key"])
	require.NotContains(t, sa.Annotations, "eks.amazonaws.com/role-arn")
}

// TestGenerateRbac_NilAnnotationsUsesIRSAFallback verifies IRSA annotation is added when
// ServiceAccountAnnotations is nil and AWS credentials are present.
func TestGenerateRbac_NilAnnotationsUsesIRSAFallback(t *testing.T) {
	ctx := context.Background()
	s := newRbacTestScheme(t)
	c := newRbacTestClient(t, s)

	connect := newTestConnect("mysite", "posit-team")
	connect.Spec.AwsAccountId = "123456789012"
	connect.Spec.ClusterDate = "20240101"
	connect.Spec.WorkloadCompoundName = "compound-name"
	// ServiceAccountAnnotations is nil by default — IRSA fallback must fire
	req := newTestRequest("posit-team", "mysite")

	err := internal.GenerateRbac(ctx, c, s, req, connect)
	require.NoError(t, err)

	sa := &v1.ServiceAccount{}
	err = c.Get(ctx, client.ObjectKey{Name: connect.ComponentName(), Namespace: "posit-team"}, sa)
	require.NoError(t, err)
	arn, ok := sa.Annotations["eks.amazonaws.com/role-arn"]
	require.True(t, ok, "expected IRSA annotation to be present")
	require.Contains(t, arn, "123456789012")
}
