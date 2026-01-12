// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/rstudio/goex/ptr"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
)

// ChronicleReconciler reconciles a Chronicle object
type ChronicleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=chronicles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=chronicles/status,verbs=get;update;patch
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=chronicles/finalizers,verbs=update

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ChronicleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	l := r.GetLogger(ctx).WithValues(
		"product", "chronicle",
		"chronicle", req.NamespacedName,
	)

	c := positcov1beta1.Chronicle{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if err := r.Get(ctx, req.NamespacedName, &c); err != nil && apierrors.IsNotFound(err) {
		l.Info("Chronicle not found; cleaning up resources")

		if _, err := r.CleanupChronicle(ctx, req, &c); err != nil {
			l.Error(err, "error cleaning up chronicle")
			return ctrl.Result{}, err
		}

		// cleanup successful
		return ctrl.Result{}, nil
	} else if err != nil {
		l.Error(err, "unexpected error retrieving Chronicle instance")
		return ctrl.Result{}, err
	}

	l.Info("Chronicle found; updating resources")

	if res, err := r.ReconcileChronicle(ctx, req, &c); err != nil {
		l.Error(err, "error reconciling product state")
		return res, err
	}

	// reconcile successful
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChronicleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&positcov1beta1.Chronicle{}).
		Complete(r)
}

func (r *ChronicleReconciler) ReconcileChronicle(ctx context.Context, req ctrl.Request, c *positcov1beta1.Chronicle) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-chronicle",
		"product", "chronicle",
	)

	// default config settings not in the original object
	// ...

	// then create the service itself
	res, err := r.ensureDeployedService(ctx, req, c)
	if err != nil {
		l.Error(err, "error deploying service")
		return res, err
	}

	// set to ready if it is not set yet...
	if !c.Status.Ready {
		c.Status.Ready = true
		if err := r.Status().Update(ctx, c); err != nil {
			l.Error(err, "Error setting ready status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ChronicleReconciler) ensureDeployedService(ctx context.Context, req ctrl.Request, c *positcov1beta1.Chronicle) (ctrl.Result, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "deploy-service",
		"product", "chronicle",
	)

	// this key is used by the configmap, statefulset, and service
	key := client.ObjectKey{
		Name:      c.ComponentName(),
		Namespace: req.Namespace,
	}

	// set a storage volume if using local storage... (just a local emptydir...)
	// TODO: more persistent local storage option?
	var storageVolume []corev1.Volume
	var storageVolumeMount []corev1.VolumeMount
	if c.Spec.Config.LocalStorage != nil && c.Spec.Config.LocalStorage.Enabled {
		storageVolume = []corev1.Volume{{
			Name: "local-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}}
		storageVolumeMount = []corev1.VolumeMount{{
			Name:      "local-storage",
			ReadOnly:  false,
			MountPath: c.Spec.Config.LocalStorage.Location,
		}}
	}

	// CONFIGMAP

	if rawConfig, err := c.Spec.Config.GenerateGcfg(); err != nil {
		l.Error(err, "error generating gcfg values")
		return ctrl.Result{}, err
	} else {
		targetConfigmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            c.ComponentName(),
				Namespace:       req.Namespace,
				Labels:          c.KubernetesLabels(),
				OwnerReferences: c.OwnerReferencesForChildren(),
			},
			Data: map[string]string{
				"posit-chronicle.gcfg": rawConfig,
			},
		}
		existingConfigmap := &corev1.ConfigMap{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingConfigmap, targetConfigmap); err != nil {
			return ctrl.Result{}, err
		}
	}

	// SERVICE ACCOUNT

	annotations := internal.AddIamAnnotation("", req.Namespace, c.SiteName(), map[string]string{}, c)

	targetServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			Annotations:     annotations,
			OwnerReferences: c.OwnerReferencesForChildren(),
		},
	}

	existingServiceAccount := &corev1.ServiceAccount{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingServiceAccount, targetServiceAccount); err != nil {
		return ctrl.Result{}, err
	}

	readOnlyName := fmt.Sprintf("%s-read-only", c.ComponentName())
	readOnlyKey := client.ObjectKey{Name: readOnlyName, Namespace: req.Namespace}
	readOnlyAnnotations := internal.AddIamAnnotation(
		fmt.Sprintf("%s-ro", c.ShortName()), req.Namespace, c.SiteName(), map[string]string{}, c,
	)
	readOnlyServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            readOnlyName,
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			Annotations:     readOnlyAnnotations,
			OwnerReferences: c.OwnerReferencesForChildren(),
		}}

	existingReadOnlyServiceAccount := &corev1.ServiceAccount{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, readOnlyKey, existingReadOnlyServiceAccount, readOnlyServiceAccount); err != nil {
		return ctrl.Result{}, err
	}

	// TODO: volume claim... leaving off and just using s3 for now...

	// STATEFULSET

	var pullSecrets []corev1.LocalObjectReference
	for _, s := range c.Spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: s})
	}

	targetStatefulset := &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			OwnerReferences: c.OwnerReferencesForChildren(),
		},
		Spec: v1.StatefulSetSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: c.SelectorLabels(),
			},
			ServiceName: "chronicle-server",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: c.KubernetesLabels(),
				},
				Spec: corev1.PodSpec{
					EnableServiceLinks: ptr.To(false),
					NodeSelector:       c.Spec.NodeSelector,
					ImagePullSecrets:   pullSecrets,
					ServiceAccountName: c.ComponentName(),
					Containers: []corev1.Container{
						{
							Name:    "chronicle-server",
							Image:   c.Spec.Image,
							Command: []string{"/chronicle"},
							Args:    []string{"start", "-c", "/etc/posit-chronicle/posit-chronicle.gcfg"},
							Env:     product.StringMapToEnvVars(c.Spec.AddEnv),
							Ports: []corev1.ContainerPort{
								internal.DefaultPortChronicleHTTP.ContainerPort("http"),
								internal.DefaultPortChronicleMetrics.ContainerPort("profile"),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{},
								Limits:   corev1.ResourceList{},
							},
							VolumeMounts: product.ConcatLists(
								[]corev1.VolumeMount{{
									Name:      "config",
									ReadOnly:  true,
									MountPath: "/etc/posit-chronicle/posit-chronicle.gcfg",
									SubPath:   "posit-chronicle.gcfg",
								}},
								storageVolumeMount,
							),
							LivenessProbe:   nil,
							ReadinessProbe:  nil, // TODO: readiness...
							StartupProbe:    nil,
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: nil,
						},
					},
					Volumes: product.ConcatLists(
						[]corev1.Volume{{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: c.ComponentName()},
									Items: []corev1.KeyToPath{
										{Key: "posit-chronicle.gcfg", Path: "posit-chronicle.gcfg"},
									},
									DefaultMode: nil,
								},
							},
						}},
						storageVolume,
					),
					SecurityContext: nil,
				},
			},
			PodManagementPolicy: "",
			UpdateStrategy:      v1.StatefulSetUpdateStrategy{},
		},
	}

	existingStatefulset := &v1.StatefulSet{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingStatefulset, targetStatefulset); err != nil {
		return ctrl.Result{}, err
	}

	// SERVICE

	targetService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            c.ComponentName(),
			Namespace:       req.Namespace,
			Labels:          c.KubernetesLabels(),
			OwnerReferences: c.OwnerReferencesForChildren(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   1,
						StrVal: "http",
					},
				},
			},
			Selector:                 c.KubernetesLabels(),
			Type:                     "ClusterIP",
			PublishNotReadyAddresses: false,
		},
	}

	existingService := &corev1.Service{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, key, existingService, targetService); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ChronicleReconciler) CleanupChronicle(ctx context.Context, req ctrl.Request, c *positcov1beta1.Chronicle) (ctrl.Result, error) {
	// TODO: some cleanup...?
	return ctrl.Result{}, nil
}

func (r *ChronicleReconciler) GetLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}
