package internal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/rstudio/goex/ptr"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddIamAnnotation(name, namespace, site string, annotations map[string]string, p product.WorkloadAccountProvider) map[string]string {
	// default to using p.ShortName() if name is not provided
	if name == "" {
		name = p.ShortName()
	}

	if p.GetAwsAccountId() != "" && p.GetClusterDate() != "" {

		roleName := ""
		if site != "" {
			roleName = fmt.Sprintf("%s.%s.%s.%s.posit.team", name, p.GetClusterDate(), site, p.WorkloadCompoundName())
		} else {
			roleName = fmt.Sprintf("%s.%s.%s.posit.team", name, p.GetClusterDate(), p.WorkloadCompoundName())
		}
		annotations["eks.amazonaws.com/role-arn"] = fmt.Sprintf("arn:aws:iam::%s:role/%s", p.GetAwsAccountId(), roleName)
	}
	return annotations
}

// ProductOwner combines ProductAndOwnerProvider with client.Object for type safety
type ProductOwner interface {
	product.ProductAndOwnerProvider
	client.Object
}

// GenerateRbac generates the SA, Role, and Rolebinding necessary for Launcher to function
func GenerateRbac(ctx context.Context, c client.Client, scheme *runtime.Scheme, req controllerruntime.Request, p ProductOwner) error {
	l := logr.FromContextOrDiscard(ctx).WithValues(
		"event", "generate-rbac",
	)
	annotations := AddIamAnnotation("", req.Namespace, "", map[string]string{}, p)

	// SERVICE ACCOUNT
	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	if _, err := CreateOrUpdateResource(ctx, c, scheme, l, serviceAccount, p, func() error {
		serviceAccount.Labels = p.KubernetesLabels()
		serviceAccount.Annotations = annotations
		// TODO: we should specify secrets here for "minimal access"
		serviceAccount.Secrets = nil
		serviceAccount.ImagePullSecrets = nil
		serviceAccount.AutomountServiceAccountToken = ptr.To(true)
		return nil
	}); err != nil {
		return err
	}

	// ROLE
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	if _, err := CreateOrUpdateResource(ctx, c, scheme, l, role, p, func() error {
		role.Labels = p.KubernetesLabels()
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"serviceaccounts"},
				Verbs:     []string{"list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods/log"},
				Verbs:     []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/attach", "pods/exec"},
				Verbs:     []string{"get", "create", "update", "patch", "watch", "list", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "create", "watch", "list", "delete"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"get", "create", "update", "patch", "watch", "list", "delete"},
			},
			{
				APIGroups: []string{"metrics.k8s.io"},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	// ROLE BINDING
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.ComponentName(),
			Namespace: req.Namespace,
		},
	}
	if _, err := CreateOrUpdateResource(ctx, c, scheme, l, roleBinding, p, func() error {
		roleBinding.Labels = p.KubernetesLabels()
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		}
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
