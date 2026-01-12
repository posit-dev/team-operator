// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2025 Posit Software, PBC

package core

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
)

// PackageManagerReconciler reconciles a PackageManager object
type PackageManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=packagemanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=packagemanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=packagemanagers/finalizers,verbs=update

//+kubebuilder:rbac:namespace=posit-team,groups=secrets-store.csi.x-k8s.io,resources=secretsproviderclass,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PackageManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	l := r.GetLogger(ctx).WithValues(
		"product", "package-manager",
		"package-manager", req.NamespacedName,
	)

	pm := positcov1beta1.PackageManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if err := r.Get(ctx, req.NamespacedName, &pm); err != nil && apierrors.IsNotFound(err) {
		l.Info("PackageManager not found; cleaning up resources")

		if _, err := r.CleanupPackageManager(ctx, req, &pm); err != nil {
			l.Error(err, "error cleaning up package-manager")
			return ctrl.Result{}, err
		}

		// cleanup successful
		return ctrl.Result{}, nil
	} else if err != nil {
		l.Error(err, "unexpected error retrieving PackageManager instance")
		return ctrl.Result{}, err
	}

	l.Info("PackageManager found; updating resources")

	if res, err := r.ReconcilePackageManager(ctx, req, &pm); err != nil {
		l.Error(err, "error reconciling product state")
		return res, err
	}

	// reconcile successful
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PackageManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&positcov1beta1.PackageManager{}).
		Complete(r)
}

func (r *PackageManagerReconciler) GetLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}
