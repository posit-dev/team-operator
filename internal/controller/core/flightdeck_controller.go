// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/rstudio/goex/ptr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FlightdeckReconciler reconciles a Flightdeck object
type FlightdeckReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=flightdecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=flightdecks/status,verbs=get;update;patch
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=flightdecks/finalizers,verbs=update
//+kubebuilder:rbac:namespace=posit-team,groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=sites,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FlightdeckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"controller", "flightdeck",
		"namespace", req.Namespace,
		"name", req.Name,
	)

	l.V(1).Info("starting reconciliation")

	fd := &positcov1beta1.Flightdeck{}

	// Fetch the Flightdeck instance
	if err := r.Get(ctx, req.NamespacedName, fd); err != nil && errors.IsNotFound(err) {
		l.Info("flightdeck resource not found, skipping reconciliation")
		return ctrl.Result{}, nil
	} else if err != nil {
		l.Error(err, "failed to fetch flightdeck resource")
		return ctrl.Result{}, err
	}

	l.V(1).Info("flightdeck resource found",
		"image", fd.Spec.Image,
		"replicas", fd.Spec.Replicas,
		"domain", fd.Spec.Domain,
	)

	if res, err := r.reconcileFlightdeckResources(ctx, req, fd, l); err != nil {
		l.Error(err, "failed to reconcile flightdeck resources")
		return res, err
	}

	l.Info("reconciliation completed successfully",
		"component", fd.ComponentName(),
		"domain", fd.Spec.Domain,
	)

	return ctrl.Result{}, nil
}

func (r *FlightdeckReconciler) reconcileFlightdeckResources(
	ctx context.Context,
	req ctrl.Request,
	fd *positcov1beta1.Flightdeck,
	l logr.Logger,
) (ctrl.Result, error) {
	componentName := fd.ComponentName()
	componentKey := client.ObjectKey{Namespace: req.Namespace, Name: componentName}

	l.V(1).Info("reconciling flightdeck resources", "component", componentName)

	// Prepare image pull secrets
	var pullSecrets []corev1.LocalObjectReference
	for _, secretName := range fd.Spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{
			Name: secretName,
		})
	}

	l.V(2).Info("prepared image pull secrets", "count", len(pullSecrets))

	// SERVICE ACCOUNT
	targetServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fd.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          fd.KubernetesLabels(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fd, fd.GroupVersionKind())},
		},
		ImagePullSecrets: pullSecrets,
	}

	existingServiceAccount := &corev1.ServiceAccount{}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, componentKey, existingServiceAccount, targetServiceAccount); err != nil {
		l.Error(err, "failed to reconcile service account", "serviceAccount", componentName)
		return ctrl.Result{}, err
	}
	l.V(1).Info("reconciled service account", "serviceAccount", componentName)

	// ROLE
	targetRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fd.ComponentName() + "-role",
			Namespace:       req.Namespace,
			Labels:          fd.KubernetesLabels(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fd, fd.GroupVersionKind())},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.posit.team"},
				Resources: []string{"sites"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	roleName := componentName + "-role"
	roleKey := client.ObjectKey{Namespace: req.Namespace, Name: roleName}
	existingRole := &rbacv1.Role{}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, roleKey, existingRole, targetRole); err != nil {
		l.Error(err, "failed to reconcile role", "role", roleName)
		return ctrl.Result{}, err
	}
	l.V(1).Info("reconciled role", "role", roleName)

	// ROLE BINDING
	targetRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fd.ComponentName() + "-rolebinding",
			Namespace:       req.Namespace,
			Labels:          fd.KubernetesLabels(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fd, fd.GroupVersionKind())},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      fd.ComponentName(),
				Namespace: req.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     targetRole.Name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	roleBindingName := componentName + "-rolebinding"
	roleBindingKey := client.ObjectKey{Namespace: req.Namespace, Name: roleBindingName}
	existingRoleBinding := &rbacv1.RoleBinding{}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, roleBindingKey, existingRoleBinding, targetRoleBinding); err != nil {
		l.Error(err, "failed to reconcile role binding", "roleBinding", roleBindingName)
		return ctrl.Result{}, err
	}
	l.V(1).Info("reconciled role binding", "roleBinding", roleBindingName)

	// Prepare environment variables
	showConfigValue := "false"
	if fd.Spec.FeatureEnabler.ShowConfig {
		showConfigValue = "true"
	}

	showAcademyValue := "false"
	if fd.Spec.FeatureEnabler.ShowAcademy {
		showAcademyValue = "true"
	}

	// Determine log level and format
	logLevel := fd.Spec.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := fd.Spec.LogFormat
	if logFormat == "" {
		logFormat = "text"
	}

	envVars := []corev1.EnvVar{
		{
			Name:  "SITE_NAME",
			Value: fd.Spec.SiteName,
		},
		{
			Name:  "SHOW_CONFIG",
			Value: showConfigValue,
		},
		{
			Name:  "SHOW_ACADEMY",
			Value: showAcademyValue,
		},
		{
			Name:  "LOG_LEVEL",
			Value: logLevel,
		},
		{
			Name:  "LOG_FORMAT",
			Value: logFormat,
		},
	}

	// Determine replicas
	replicas := int32(1)
	if fd.Spec.Replicas > 0 {
		replicas = int32(fd.Spec.Replicas)
	}

	// Determine port
	port := fd.Spec.Port
	if port == 0 {
		port = 8080
	}

	// Determine image pull policy.
	// Default to Always to ensure the latest image is pulled on each deployment.
	// This increases container startup time and network usage but ensures
	// consistency, especially important for mutable tags like 'latest'.
	// Users can override this in the Flightdeck spec if needed.
	imagePullPolicy := fd.Spec.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullAlways
	}

	// DEPLOYMENT
	targetDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fd.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          fd.KubernetesLabels(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fd, fd.GroupVersionKind())},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: fd.SelectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: fd.KubernetesLabels(),
				},
				Spec: corev1.PodSpec{
					EnableServiceLinks: ptr.To(false),
					ImagePullSecrets:   pullSecrets,
					ServiceAccountName: fd.ComponentName(),
					Containers: []corev1.Container{
						{
							Name:            "flightdeck",
							Image:           fd.Spec.Image,
							ImagePullPolicy: imagePullPolicy,
							Env:             envVars,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                ptr.To(int64(999)),
								RunAsNonRoot:             ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      3,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 3,
								TimeoutSeconds:      1,
								PeriodSeconds:       3,
								SuccessThreshold:    1,
								FailureThreshold:    2,
							},
						},
					},
				},
			},
		},
	}

	existingDeployment := &appsv1.Deployment{}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, componentKey, existingDeployment, targetDeployment); err != nil {
		l.Error(err, "failed to reconcile deployment", "deployment", componentName)
		return ctrl.Result{}, err
	}
	l.V(1).Info("reconciled deployment",
		"deployment", componentName,
		"image", fd.Spec.Image,
		"replicas", replicas,
	)

	// SERVICE
	targetService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fd.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          fd.KubernetesLabels(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fd, fd.GroupVersionKind())},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromString("http"),
				},
			},
			Selector: fd.SelectorLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	existingService := &corev1.Service{}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, componentKey, existingService, targetService); err != nil {
		l.Error(err, "failed to reconcile service", "service", componentName)
		return ctrl.Result{}, err
	}
	l.V(1).Info("reconciled service", "service", componentName)

	// INGRESS
	annotations := map[string]string{}
	for k, v := range fd.Spec.IngressAnnotations {
		annotations[k] = v
	}

	targetIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fd.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          fd.KubernetesLabels(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(fd, fd.GroupVersionKind())},
			Annotations:     annotations,
		},
		Spec: networkingv1.IngressSpec{
			TLS: nil,
			Rules: []networkingv1.IngressRule{
				{
					Host: fd.Spec.Domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: fd.ComponentName(),
											Port: networkingv1.ServiceBackendPort{
												Name: "http",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Only define the ingressClassName if it is specified
	if fd.Spec.IngressClass != "" {
		targetIngress.Spec.IngressClassName = &fd.Spec.IngressClass
	}

	existingIngress := &networkingv1.Ingress{}
	if err := internal.BasicCreateOrUpdate(ctx, r, l, componentKey, existingIngress, targetIngress); err != nil {
		l.Error(err, "failed to reconcile ingress", "ingress", componentName)
		return ctrl.Result{}, err
	}
	l.V(1).Info("reconciled ingress",
		"ingress", componentName,
		"domain", fd.Spec.Domain,
		"ingressClass", fd.Spec.IngressClass,
	)

	l.V(1).Info("all flightdeck resources reconciled successfully", "component", componentName)

	return ctrl.Result{}, nil
}

// GetLogger returns a logger with the controller name
func (r *FlightdeckReconciler) GetLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}

// SetupWithManager sets up the controller with the Manager.
func (r *FlightdeckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&positcov1beta1.Flightdeck{}).
		Named("flightdeck").
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
