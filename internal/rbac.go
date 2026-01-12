package internal

import (
	"context"
	"fmt"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/rstudio/goex/ptr"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GenerateRbac generates the SA, Role, and Rolebinding necessary for Launcher to function
func GenerateRbac(ctx context.Context, r product.SomeReconciler, req controllerruntime.Request, p product.ProductAndOwnerProvider) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "generate-rbac",
	)
	key := client.ObjectKey{
		Name:      p.ComponentName(),
		Namespace: req.Namespace,
	}
	annotations := AddIamAnnotation("", req.Namespace, "", map[string]string{}, p)
	targetServiceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            p.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          p.KubernetesLabels(),
			Annotations:     annotations,
			OwnerReferences: p.OwnerReferencesForChildren(),
		},
		// TODO: we should specify secrets here for "minimal access"
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: ptr.To(true),
	}

	existingServiceAccount := &v1.ServiceAccount{}

	if err := BasicCreateOrUpdate(ctx, r, l, key, existingServiceAccount, targetServiceAccount); err != nil {
		return err
	}

	// ROLE

	targetRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            p.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          p.KubernetesLabels(),
			Annotations:     nil,
			OwnerReferences: p.OwnerReferencesForChildren(),
		},
		Rules: []rbacv1.PolicyRule{
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
		},
	}

	existingRole := &rbacv1.Role{}

	if err := BasicCreateOrUpdate(ctx, r, l, key, existingRole, targetRole); err != nil {
		return err
	}

	// ROLE BINDING
	targetRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            p.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          p.KubernetesLabels(),
			Annotations:     nil,
			OwnerReferences: p.OwnerReferencesForChildren(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      targetServiceAccount.Name,
				Namespace: targetServiceAccount.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     targetRole.Name,
		},
	}

	existingRoleBinding := &rbacv1.RoleBinding{}

	if err := BasicCreateOrUpdate(ctx, r, l, key, existingRoleBinding, targetRoleBinding); err != nil {
		return err
	}

	return nil
}
