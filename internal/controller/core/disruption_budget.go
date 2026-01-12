package core

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	policyv1 "k8s.io/api/policy/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdateDisruptionBudget(ctx context.Context, req ctrl.Request, r product.SomeReconciler, p product.NamerAndOwnerProvider, minAvailable, maxUnavailable *int) error {
	pdb := product.DefineDisruptionBudget(p, req, minAvailable, maxUnavailable)

	key := client.ObjectKey{
		Name:      pdb.Name,
		Namespace: product.PositTeamNamespace,
	}

	// Create or update the PodDisruptionBudget
	return internal.BasicCreateOrUpdate(ctx, r, logr.FromContextOrDiscard(ctx), key, &policyv1.PodDisruptionBudget{}, pdb)
}
