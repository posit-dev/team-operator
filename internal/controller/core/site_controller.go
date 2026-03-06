// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"fmt"
	url "net/url"
	"strings"

	"github.com/go-logr/logr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/status"
	"github.com/rstudio/goex/ptr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// isProductEnabled returns true if the product is enabled (nil defaults to enabled).
func isProductEnabled(b *bool) bool {
	return b == nil || *b
}

// isProductDisabled returns true if the product is explicitly disabled (Enabled=false).
// Returns false when Enabled is nil (default-enabled) or true.
func isProductDisabled(b *bool) bool {
	return b != nil && !*b
}

// SiteReconciler reconciles a Site object
type SiteReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=sites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=sites/status,verbs=get;update;patch
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=sites/finalizers,verbs=update

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="apps",resources=daemonsets,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:namespace=posit-team,groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:namespace=posit-team,groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:namespace=posit-team,groups="k8s.keycloak.org",resources=keycloaks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups="k8s.keycloak.org",resources=keycloakrealmimports,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *SiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	s := &positcov1beta1.Site{}

	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}

		return r.Log
	}().WithValues(
		"site", req.NamespacedName,
	)

	err := r.Get(ctx, req.NamespacedName, s)
	if err != nil && apierrors.IsNotFound(err) {
		l.Info("Site not found; cleaning up resources")
		return r.cleanupResources(ctx, req)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	l.Info("Site found; updating resources")

	// Save a copy for status patching
	patchBase := client.MergeFrom(s.DeepCopy())

	// Set observed generation and progressing condition
	s.Status.ObservedGeneration = s.Generation
	status.SetProgressing(&s.Status.Conditions, s.Generation, metav1.ConditionTrue, status.ReasonReconciling, "Reconciliation in progress")

	result, reconcileErr := r.reconcileResources(ctx, req, s)

	// Aggregate child component status
	aggregateErr := r.aggregateChildStatus(ctx, req, s, l)

	// Update status based on reconciliation result
	if reconcileErr != nil {
		status.SetReady(&s.Status.Conditions, s.Generation, metav1.ConditionFalse, status.ReasonReconcileError, reconcileErr.Error())
		status.SetProgressing(&s.Status.Conditions, s.Generation, metav1.ConditionFalse, status.ReasonReconcileError, reconcileErr.Error())
	} else {
		// Overall Ready is true only if all children are ready
		allReady := s.Status.ConnectReady && s.Status.WorkbenchReady && s.Status.PackageManagerReady && s.Status.ChronicleReady && s.Status.FlightdeckReady
		if allReady {
			status.SetReady(&s.Status.Conditions, s.Generation, metav1.ConditionTrue, status.ReasonAllComponentsReady, "All child components are ready")
		} else {
			status.SetReady(&s.Status.Conditions, s.Generation, metav1.ConditionFalse, status.ReasonComponentsNotReady, "One or more child components are not ready")
		}
		status.SetProgressing(&s.Status.Conditions, s.Generation, metav1.ConditionFalse, status.ReasonReconcileComplete, "Reconciliation complete")
	}

	// Patch status
	if patchErr := r.Status().Patch(ctx, s, patchBase); patchErr != nil {
		l.Error(patchErr, "Error patching status")
		if reconcileErr != nil {
			return result, reconcileErr
		}
		return ctrl.Result{}, patchErr
	}

	if reconcileErr != nil {
		if aggregateErr != nil {
			l.Error(aggregateErr, "Error aggregating child status (returning reconcile error instead)")
		}
		return result, reconcileErr
	}
	return result, aggregateErr
}

var rootVolumeSize = resource.MustParse("1Gi")
var connectVolumeSize = resource.MustParse("10Gi")
var workbenchSharedStorageVolumeSize = resource.MustParse("10Gi")

func prefixDomain(prefix, domain string, domainType positcov1beta1.SiteDomainType) string {
	if domainType == positcov1beta1.SiteDashDomain {
		return fmt.Sprintf("%s-%s", prefix, domain)
	}
	return fmt.Sprintf("%s.%s", prefix, domain)
}

func getEffectiveBaseDomain(baseDomain, fallbackDomain string) string {
	if baseDomain != "" {
		return baseDomain
	}
	return fallbackDomain
}

func (r *SiteReconciler) reconcileResources(ctx context.Context, req ctrl.Request, site *positcov1beta1.Site) (ctrl.Result, error) {

	l := r.GetLogger(ctx).WithValues(
		"event", "reconcile-resources",
	)

	var dbUrl *url.URL
	var err error
	// NOTE: this dbUrl can have the password in it!
	if dbUrl, err = internal.DetermineMainDatabaseUrl(ctx, r, req, site.Spec.WorkloadSecret, site.Spec.MainDatabaseCredentialSecret); err != nil {
		l.Error(err, "error determining database url")
		return ctrl.Result{}, err
	}

	dbQuery := dbUrl.Query()
	sslMode := ""
	for k, v := range dbQuery {
		if strings.ToLower(k) == "sslmode" {
			sslMode = v[0]
		}
	}

	// IMAGE PREPULL DAEMONSET
	if !site.Spec.DisablePrePullImages {
		if err := deployPrePullDaemonset(ctx, r, req, site); err != nil {
			l.Error(err, "error deploying pre-pull daemonset")
			return ctrl.Result{}, err
		}
	}

	// PRODUCT URLS
	// Building these here instead of in the product reconciler because packageManagerUrl is needed to
	// create the packageManagerRepoUrl which must be passed to more than one product and this keeps them all together.

	// Default to subdomain type since SiteHome is removed
	domainType := positcov1beta1.SiteSubDomain
	packageManagerUrl := prefixDomain(
		site.Spec.PackageManager.DomainPrefix,
		getEffectiveBaseDomain(site.Spec.PackageManager.BaseDomain, site.Spec.Domain),
		domainType,
	)
	connectUrl := prefixDomain(
		site.Spec.Connect.DomainPrefix,
		getEffectiveBaseDomain(site.Spec.Connect.BaseDomain, site.Spec.Domain),
		domainType,
	)
	workbenchUrl := prefixDomain(
		site.Spec.Workbench.DomainPrefix,
		getEffectiveBaseDomain(site.Spec.Workbench.BaseDomain, site.Spec.Domain),
		domainType,
	)

	packageManagerRepoUrl := fmt.Sprintf("https://%s/cran/__linux__/jammy/latest", packageManagerUrl) // TODO: don't hardcode OS
	if site.Spec.PackageManagerUrl != "" {
		packageManagerRepoUrl = site.Spec.PackageManagerUrl
	}

	// VOLUMES

	// Determine if Connect is enabled (used for volume provisioning and later for reconciliation)
	connectEnabled := isProductEnabled(site.Spec.Connect.Enabled)
	connectTeardown := site.Spec.Connect.Teardown != nil && *site.Spec.Connect.Teardown
	if connectTeardown && connectEnabled {
		l.Info("connect.teardown is set but connect.enabled is not false; teardown has no effect until enabled=false")
	}

	workbenchEnabled := isProductEnabled(site.Spec.Workbench.Enabled)
	workbenchTeardown := site.Spec.Workbench.Teardown != nil && *site.Spec.Workbench.Teardown
	if workbenchTeardown && workbenchEnabled {
		l.Info("workbench.teardown is set but workbench.enabled is not false; teardown has no effect until enabled=false")
	}

	pmEnabled := isProductEnabled(site.Spec.PackageManager.Enabled)
	pmTeardown := site.Spec.PackageManager.Teardown != nil && *site.Spec.PackageManager.Teardown
	if pmTeardown && pmEnabled {
		l.Info("packageManager.teardown is set but packageManager.enabled is not false; teardown has no effect until enabled=false")
	}

	chronicleEnabled := isProductEnabled(site.Spec.Chronicle.Enabled)
	chronicleTeardown := site.Spec.Chronicle.Teardown != nil && *site.Spec.Chronicle.Teardown
	if chronicleTeardown && chronicleEnabled {
		l.Info("chronicle.teardown is set but chronicle.enabled is not false; teardown has no effect until enabled=false")
	}

	connectVolumeName := fmt.Sprintf("%s-connect", site.Name)
	connectStorageClassName := connectVolumeName
	devVolumeName := fmt.Sprintf("%s-workbench", site.Name)
	devStorageClassName := devVolumeName
	sharedVolumeName := fmt.Sprintf("%s-shared", site.Name)
	sharedStorageClassName := sharedVolumeName

	if site.Spec.VolumeSource.Type == positcov1beta1.VolumeSourceTypeAzureNetApp {
		connectStorageClassName = string(positcov1beta1.StorageClassAzureNetApp)
		devStorageClassName = string(positcov1beta1.StorageClassAzureNetApp)
		sharedStorageClassName = string(positcov1beta1.StorageClassAzureNetApp)

	}

	if site.Spec.VolumeSource.Type != positcov1beta1.VolumeSourceTypeNone {
		l.Info("Provisioning volumes", "volume-type", site.Spec.VolumeSource.Type)

		if site.Spec.VolumeSource.Type == positcov1beta1.VolumeSourceTypeFsxZfs {
			if err := r.provisionRootFsxVolume(ctx, site); err != nil {
				return ctrl.Result{}, err
			}

			// Only provision Connect volume if Connect is enabled
			if connectEnabled {
				if err := r.provisionFsxVolume(ctx, site, connectVolumeName, "connect", connectVolumeSize); err != nil {
					return ctrl.Result{}, err
				}
			}

			if workbenchEnabled {
				if err := r.provisionFsxVolume(ctx, site, devVolumeName, "workbench", connectVolumeSize); err != nil {
					return ctrl.Result{}, err
				}

				// Provision shared storage volume for workbench load balancing
				workbenchSharedStorageVolumeName := fmt.Sprintf("%s-workbench-shared-storage", site.Name)
				// Note: provisionFsxVolume uses the volume name as the storage class name
				if err := r.provisionFsxVolume(ctx, site, workbenchSharedStorageVolumeName, "workbench-shared-storage", workbenchSharedStorageVolumeSize); err != nil {
					return ctrl.Result{}, err
				}
			}

			if site.Spec.SharedDirectory != "" {
				if err := r.provisionFsxVolume(ctx, site, sharedVolumeName, "shared", connectVolumeSize); err != nil {
					return ctrl.Result{}, err
				}
			}

			// create a job to provision subdirectories
			if err := r.provisionSubDirectoryCreator(ctx, req, site, site.Name); err != nil {
				return ctrl.Result{}, err
			}
		}

		if site.Spec.VolumeSource.Type == positcov1beta1.VolumeSourceTypeNfs {
			if err := r.provisionRootNfsVolume(ctx, site); err != nil {
				return ctrl.Result{}, err
			}

			// Only provision Connect volume if Connect is enabled
			if connectEnabled {
				connectStorageClassName = fmt.Sprintf("%s-nfs", connectVolumeName)

				if err := r.provisionNfsVolume(ctx, site, connectVolumeName, "connect", connectStorageClassName, connectVolumeSize); err != nil {
					return ctrl.Result{}, err
				}
			}

			devStorageClassName = fmt.Sprintf("%s-nfs", devVolumeName)

			if workbenchEnabled {
				if err := r.provisionNfsVolume(ctx, site, devVolumeName, "workbench", devStorageClassName, connectVolumeSize); err != nil {
					return ctrl.Result{}, err
				}

				// Provision shared storage volume for workbench load balancing
				workbenchSharedStorageVolumeName := fmt.Sprintf("%s-workbench-shared-storage", site.Name)
				workbenchSharedStorageClassName := fmt.Sprintf("%s-nfs", workbenchSharedStorageVolumeName)
				if err := r.provisionNfsVolume(ctx, site, workbenchSharedStorageVolumeName, "workbench-shared-storage", workbenchSharedStorageClassName, workbenchSharedStorageVolumeSize); err != nil {
					return ctrl.Result{}, err
				}
			}

			if site.Spec.SharedDirectory != "" {
				sharedStorageClassName = fmt.Sprintf("%s-nfs", sharedVolumeName)
				if err := r.provisionNfsVolume(ctx, site, sharedVolumeName, "shared", sharedStorageClassName, connectVolumeSize); err != nil {
					return ctrl.Result{}, err
				}
			}

			// create a job to provision subdirectories
			if err := r.provisionSubDirectoryCreator(ctx, req, site, fmt.Sprintf("%s-nfs", site.Name)); err != nil {
				return ctrl.Result{}, err
			}

		}
	}

	// CLEANUP LEGACY HOME APP
	// Remove any legacy home app resources that may exist from before the flightdeck migration
	if err := r.cleanupLegacyHomeApp(ctx, req); err != nil {
		l.Error(err, "error cleaning up legacy home app")
		return ctrl.Result{}, err
	}

	// FLIGHTDECK
	flightdeckEnabled := isProductEnabled(site.Spec.Flightdeck.Enabled)
	if flightdeckEnabled {
		if err := r.reconcileFlightdeck(ctx, req, site); err != nil {
			l.Error(err, "error reconciling flightdeck")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.disableFlightdeck(ctx, req, l); err != nil {
			l.Error(err, "error disabling flightdeck")
			return ctrl.Result{}, err
		}
	}

	// ADDITIONAL SHARED DIRECTORY

	additionalVolumes := []product.VolumeSpec{}
	if site.Spec.SharedDirectory != "" {
		vol := product.VolumeSpec{
			Create:           true,
			AccessModes:      []string{"ReadWriteMany"},
			VolumeName:       sharedVolumeName,
			StorageClassName: sharedStorageClassName,
			Size:             connectVolumeSize.String(),

			// TODO: the fact that we have no need for MountPath/ReadOnly here is kinda lame...
			//   Basically, this VolumeSpec interface does double duty for "create" and "mount"
			//   Here, we are _only_ concerned about "create"
			PvcName:   sharedVolumeName,
			MountPath: fmt.Sprintf("/mnt/%s", site.Spec.SharedDirectory),
			ReadOnly:  false,
		}
		additionalVolumes = append(additionalVolumes, vol)
		if pvc, err := product.DefinePvc(site, req, sharedVolumeName, &vol, connectVolumeSize); err != nil {
			l.Error(err, "error defining shared directory PVC")
			return ctrl.Result{}, err
		} else {
			sharedVolumeKey := client.ObjectKey{Name: sharedVolumeName, Namespace: req.Namespace}
			if err := internal.PvcCreateOrUpdate(ctx, r, l, sharedVolumeKey, &corev1.PersistentVolumeClaim{}, pvc); err != nil {
				l.Error(err, "error creating shared directory PVC")
				return ctrl.Result{}, err
			} else {
				l.Info("successfully created shared directory PVC", "pvc", sharedVolumeName)
			}
		}
	}

	// WORKBENCH ADDITIONAL VOLUMES
	// Merge workbench-specific additional volumes with shared directory volumes
	workbenchAdditionalVolumes := append([]product.VolumeSpec{}, additionalVolumes...)
	workbenchAdditionalVolumes = append(workbenchAdditionalVolumes, site.Spec.Workbench.AdditionalVolumes...)

	// CONNECT
	if connectEnabled {
		// Connect is enabled - reconcile normally
		if err := r.reconcileConnect(
			ctx,
			req,
			site,
			dbUrl.Host,
			sslMode,
			connectVolumeName,
			connectStorageClassName,
			additionalVolumes,
			packageManagerRepoUrl,
			connectUrl,
		); err != nil {
			l.Error(err, "error reconciling connect")
			return ctrl.Result{}, err
		}
	} else if connectTeardown {
		// Connect is disabled with teardown=true - DESTRUCTIVE cleanup
		// This triggers permanent deletion of the Connect CRD, which causes
		// the Connect finalizer to destroy the database, secrets, and all resources.
		if err := r.cleanupConnect(ctx, req, l); err != nil {
			l.Error(err, "error tearing down connect resources")
			return ctrl.Result{}, err
		}
	} else {
		// Connect is disabled but teardown=false - non-destructive suspend
		// Removes serving resources (Deployment/Service/Ingress) but preserves data
		if err := r.disableConnect(ctx, req, l); err != nil {
			l.Error(err, "error disabling connect")
			return ctrl.Result{}, err
		}
	}

	// PACKAGE MANAGER
	if pmEnabled {
		if err := r.reconcilePackageManager(
			ctx,
			req,
			site,
			dbUrl.Host,
			sslMode,
			packageManagerUrl,
		); err != nil {
			l.Error(err, "error reconciling package manager")
			return ctrl.Result{}, err
		}
	} else if pmTeardown {
		if err := r.cleanupPackageManager(ctx, req, l); err != nil {
			l.Error(err, "error tearing down package manager resources")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.disablePackageManager(ctx, req, l); err != nil {
			l.Error(err, "error disabling package manager")
			return ctrl.Result{}, err
		}
	}

	// WORKBENCH
	if workbenchEnabled {
		if err := r.reconcileWorkbench(
			ctx,
			req,
			site,
			dbUrl.Host,
			sslMode,
			devVolumeName,
			devStorageClassName,
			workbenchAdditionalVolumes,
			packageManagerRepoUrl,
			workbenchUrl,
		); err != nil {
			l.Error(err, "error reconciling workbench")
			return ctrl.Result{}, err
		}
	} else if workbenchTeardown {
		if err := r.cleanupWorkbench(ctx, req, l); err != nil {
			l.Error(err, "error tearing down workbench resources")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.disableWorkbench(ctx, req, l); err != nil {
			l.Error(err, "error disabling workbench")
			return ctrl.Result{}, err
		}
	}

	// CHRONICLE
	if chronicleEnabled {
		if err := r.reconcileChronicle(ctx, req, site); err != nil {
			l.Error(err, "error reconciling chronicle")
			return ctrl.Result{}, err
		}
	} else if chronicleTeardown {
		if err := r.cleanupChronicle(ctx, req, l); err != nil {
			l.Error(err, "error tearing down chronicle resources")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.disableChronicle(ctx, req, l); err != nil {
			l.Error(err, "error disabling chronicle")
			return ctrl.Result{}, err
		}
	}

	// KEYCLOAK

	if err := r.reconcileKeycloak(ctx, req, site, dbUrl, sslMode); err != nil {
		l.Error(err, "error reconciling keycloak")
		return ctrl.Result{}, err
	}

	// EXTRA SERVICE ACCOUNTS

	for _, s := range site.Spec.ExtraSiteServiceAccounts {
		serviceAccountName := fmt.Sprintf("%s-%s", site.Name, s.NameSuffix)
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: req.Namespace,
			},
		}

		annotations := s.Annotations
		if _, err := internal.CreateOrUpdateResource(ctx, r.Client, r.Scheme, l, sa, site, func() error {
			sa.Labels = site.KubernetesLabels()
			sa.Annotations = annotations
			sa.Secrets = nil
			sa.ImagePullSecrets = nil
			sa.AutomountServiceAccountToken = ptr.To(true)
			return nil
		}); err != nil {
			l.Error(err, "error creating or updating extra service account", "serviceAccount", serviceAccountName)
			return ctrl.Result{}, err
		}
	}

	// NETWORK POLICIES

	if err := r.reconcileNetworkPolicies(ctx, req, site); err != nil {
		l.Error(err, "error reconciling network policies")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// aggregateChildStatus fetches each child CR and populates per-component readiness bools on the Site status.
// Returns a non-nil error only for transient API errors (not NotFound), so the reconciler can requeue.
// On transient error, all products are still evaluated so the status snapshot is as complete as possible.
//
// Products are default-enabled (Connect, Workbench, PackageManager, Chronicle, Flightdeck):
// missing CR is ready only when explicitly disabled (Enabled != nil && !*Enabled). If Enabled
// is nil the product is expected → not ready.
func (r *SiteReconciler) aggregateChildStatus(ctx context.Context, req ctrl.Request, site *positcov1beta1.Site, _ logr.Logger) error {
	// Child CRs (Connect, Workbench, etc.) are created by reconcileResources with the same
	// name as the parent Site. See site_controller_connect.go, site_controller_workbench.go, etc.
	key := client.ObjectKey{Name: site.Name, Namespace: req.Namespace}

	var firstErr error

	// Connect
	connect := &positcov1beta1.Connect{}
	if err := r.Get(ctx, key, connect); err == nil {
		if isProductDisabled(site.Spec.Connect.Enabled) {
			site.Status.ConnectReady = true
		} else {
			site.Status.ConnectReady = status.IsReady(connect.Status.Conditions)
		}
	} else if apierrors.IsNotFound(err) {
		site.Status.ConnectReady = isProductDisabled(site.Spec.Connect.Enabled)
	} else {
		if firstErr == nil {
			firstErr = fmt.Errorf("fetching Connect for status aggregation: %w", err)
		}
		site.Status.ConnectReady = false
	}

	// Workbench
	workbench := &positcov1beta1.Workbench{}
	if err := r.Get(ctx, key, workbench); err == nil {
		if isProductDisabled(site.Spec.Workbench.Enabled) {
			site.Status.WorkbenchReady = true
		} else {
			site.Status.WorkbenchReady = status.IsReady(workbench.Status.Conditions)
		}
	} else if apierrors.IsNotFound(err) {
		site.Status.WorkbenchReady = isProductDisabled(site.Spec.Workbench.Enabled)
	} else {
		if firstErr == nil {
			firstErr = fmt.Errorf("fetching Workbench for status aggregation: %w", err)
		}
		site.Status.WorkbenchReady = false
	}

	// PackageManager
	pm := &positcov1beta1.PackageManager{}
	if err := r.Get(ctx, key, pm); err == nil {
		if isProductDisabled(site.Spec.PackageManager.Enabled) {
			site.Status.PackageManagerReady = true
		} else {
			site.Status.PackageManagerReady = status.IsReady(pm.Status.Conditions)
		}
	} else if apierrors.IsNotFound(err) {
		site.Status.PackageManagerReady = isProductDisabled(site.Spec.PackageManager.Enabled)
	} else {
		if firstErr == nil {
			firstErr = fmt.Errorf("fetching PackageManager for status aggregation: %w", err)
		}
		site.Status.PackageManagerReady = false
	}

	// Chronicle
	chronicle := &positcov1beta1.Chronicle{}
	if err := r.Get(ctx, key, chronicle); err == nil {
		if isProductDisabled(site.Spec.Chronicle.Enabled) {
			site.Status.ChronicleReady = true
		} else {
			site.Status.ChronicleReady = status.IsReady(chronicle.Status.Conditions)
		}
	} else if apierrors.IsNotFound(err) {
		site.Status.ChronicleReady = isProductDisabled(site.Spec.Chronicle.Enabled)
	} else {
		if firstErr == nil {
			firstErr = fmt.Errorf("fetching Chronicle for status aggregation: %w", err)
		}
		site.Status.ChronicleReady = false
	}

	// Flightdeck
	flightdeck := &positcov1beta1.Flightdeck{}
	if err := r.Get(ctx, key, flightdeck); err == nil {
		if isProductDisabled(site.Spec.Flightdeck.Enabled) {
			site.Status.FlightdeckReady = true
		} else {
			site.Status.FlightdeckReady = status.IsReady(flightdeck.Status.Conditions)
		}
	} else if apierrors.IsNotFound(err) {
		site.Status.FlightdeckReady = isProductDisabled(site.Spec.Flightdeck.Enabled)
	} else {
		if firstErr == nil {
			firstErr = fmt.Errorf("fetching Flightdeck for status aggregation: %w", err)
		}
		site.Status.FlightdeckReady = false
	}

	return firstErr
}

func (r *SiteReconciler) GetLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}

func (r *SiteReconciler) cleanupResources(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}

		return r.Log
	}().WithValues(
		"event", "cleanup-resources",
	)

	// clean up each product, Connect -> Workbench -> PM

	existingConnect := positcov1beta1.Connect{}
	connectKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicDelete(ctx, r, l, connectKey, &existingConnect); err != nil {
		l.Error(err, "error cleaning up connect", "product", "connect")
	}

	existingWorkbench := positcov1beta1.Workbench{}
	workbenchKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicDelete(ctx, r, l, workbenchKey, &existingWorkbench); err != nil {
		l.Error(err, "error cleaning up workbench", "product", "workbench")
	}

	existingPackageManager := positcov1beta1.PackageManager{}
	pmKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicDelete(ctx, r, l, pmKey, &existingPackageManager); err != nil {
		l.Error(err, "error cleaning up package manager", "product", "package-manager")
	}

	existingChronicle := positcov1beta1.Chronicle{}
	chronicleKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicDelete(ctx, r, l, chronicleKey, &existingChronicle); err != nil {
		l.Error(err, "error cleaning up chronicle", "product", "chronicle")
	}

	existingFlightdeck := positcov1beta1.Flightdeck{}
	flightdeckKey := client.ObjectKey{Name: req.Name, Namespace: req.Namespace}
	if err := internal.BasicDelete(ctx, r, l, flightdeckKey, &existingFlightdeck); err != nil {
		l.Error(err, "error cleaning up flightdeck", "product", "flightdeck")
	}

	if err := r.cleanupNetworkPolicies(ctx, req); err != nil {
		l.Error(err, "error cleaning up network policies")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&positcov1beta1.Site{}).
		Owns(&positcov1beta1.Connect{}).
		Owns(&positcov1beta1.Workbench{}).
		Owns(&positcov1beta1.PackageManager{}).
		Owns(&positcov1beta1.Chronicle{}).
		Owns(&positcov1beta1.Flightdeck{}).
		Complete(r)
}
