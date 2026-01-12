package product

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func DefineDisruptionBudget(p NamerAndOwnerProvider, req ctrl.Request, minAvailable *int, maxUnavailable *int) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            p.ComponentName(),
			Namespace:       req.Namespace,
			OwnerReferences: p.OwnerReferencesForChildren(),
			Labels:          p.KubernetesLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: p.SelectorLabels(),
			},
		},
	}

	// set minAvailable first, then maxUnavailable, then default maxUnavail = 1
	if minAvailable != nil {
		pdb.Spec.MinAvailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: int32(*minAvailable),
		}
	} else if maxUnavailable != nil {
		pdb.Spec.MaxUnavailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: int32(*maxUnavailable),
		}
	} else {
		pdb.Spec.MaxUnavailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: 1,
		}
	}

	return pdb
}
