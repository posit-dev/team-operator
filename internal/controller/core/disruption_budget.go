package core

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdateDisruptionBudget(ctx context.Context, req ctrl.Request, c client.Client, scheme *runtime.Scheme, p product.NamerAndOwnerProvider, owner client.Object, minAvailable, maxUnavailable *int) error {
	l := logr.FromContextOrDiscard(ctx)
	pdbSpec := product.DefineDisruptionBudget(p, req, minAvailable, maxUnavailable)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pdbSpec.Name,
			Namespace: product.PositTeamNamespace,
		},
	}

	// Create or update the PodDisruptionBudget
	_, err := internal.CreateOrUpdateResource(ctx, c, scheme, l, pdb, owner, func() error {
		pdb.Labels = pdbSpec.Labels
		pdb.Spec = pdbSpec.Spec
		return nil
	})
	return err
}
