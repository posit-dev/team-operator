// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"go.opentelemetry.io/otel/metric"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal/observability"
)

// ConnectReconciler reconciles a ImplConnect object
type ConnectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Meter  metric.Meter
}

//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=connects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=connects/status,verbs=get;update;patch
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=connects/finalizers,verbs=update

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=events,verbs=watch
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=pods/attach,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=pods/exec,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=pods/log,verbs=get;list;watch
//+kubebuilder:rbac:namespace=posit-team,groups=metrics.k8s.io,resources=pods,verbs=get
//+kubebuilder:rbac:namespace=posit-team,groups=secrets-store.csi.x-k8s.io,resources=secretsproviderclass,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ConnectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	l := r.GetLogger(ctx).WithValues(
		"product", "connect",
		"connect", req.NamespacedName,
	)

	c := positcov1beta1.Connect{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if err := r.Get(ctx, req.NamespacedName, &c); err != nil && apierrors.IsNotFound(err) {
		l.Info("Connect not found; cleaning up resources")

		if _, err := r.CleanupConnect(ctx, req, &c); err != nil {
			l.Error(err, "error cleaning up connect")
			return ctrl.Result{}, err
		}

		// cleanup successful
		return ctrl.Result{}, nil
	} else if err != nil {
		l.Error(err, "unexpected error retrieving Connect instance")
		return ctrl.Result{}, err
	}

	l.Info("Connect found; updating resources")

	// Capture prior phase before any mutation so the metric reflects the real transition.
	priorPhase := observability.PhaseFromConditions(c.Status.Conditions)

	if res, err := r.ReconcileConnect(ctx, req, &c); err != nil {
		l.Error(err, "error reconciling product state")
		observability.RecordStatusTransition(ctx, r.Meter, "connect", req.Namespace,
			priorPhase, observability.PhaseError)
		return res, err
	}
	// reconcile successful — success metric recorded inside ReconcileConnect
	return ctrl.Result{}, nil
}

func (r *ConnectReconciler) GetLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConnectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&positcov1beta1.Connect{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
