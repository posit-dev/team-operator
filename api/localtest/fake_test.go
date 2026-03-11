package localtest_test

import (
	"context"
	"testing"

	v1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/localtest"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func loadFakeSchemes(scheme *runtime.Scheme) {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
}

// TestStatusUpdateOnlyMutatesStatus verifies that Status().Update() persists the
// status subresource independently from the main object body. Without
// WithStatusSubresource registration in the fake client, status updates silently
// mutate the whole object (including spec), producing test false-positives.
func TestStatusUpdateOnlyMutatesStatus(t *testing.T) {
	r := require.New(t)
	ctx := context.TODO()

	fte := &localtest.FakeTestEnv{}
	cli, _, _ := fte.Start(loadFakeSchemes)

	site := &v1beta1.Site{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-site",
			Namespace: "default",
		},
		Spec: v1beta1.SiteSpec{
			Domain: "original.example.com",
		},
	}
	r.NoError(cli.Create(ctx, site))

	// Mutate both spec and status on the in-memory object, then call
	// Status().Update(). Only the status change should be persisted.
	site.Spec.Domain = "should-not-persist.example.com"
	site.Status.ConnectReady = true
	r.NoError(cli.Status().Update(ctx, site))

	fetched := &v1beta1.Site{}
	r.NoError(cli.Get(ctx, client.ObjectKeyFromObject(site), fetched))

	// Status update must be persisted.
	r.True(fetched.Status.ConnectReady, "status.connectReady should be true after Status().Update()")

	// Spec must not be affected by the status update.
	r.Equal("original.example.com", fetched.Spec.Domain,
		"spec.domain must not be modified by Status().Update()")
}
