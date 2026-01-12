// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	"fmt"
	url "net/url"
	"strings"

	"github.com/go-logr/logr"
	"github.com/rstudio/goex/ptr"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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

	return r.reconcileResources(ctx, req, s)
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
	packageManagerUrl := prefixDomain(site.Spec.PackageManager.DomainPrefix, site.Spec.Domain, domainType)
	connectUrl := prefixDomain(site.Spec.Connect.DomainPrefix, site.Spec.Domain, domainType)
	workbenchUrl := prefixDomain(site.Spec.Workbench.DomainPrefix, site.Spec.Domain, domainType)

	packageManagerRepoUrl := fmt.Sprintf("https://%s/cran/__linux__/jammy/latest", packageManagerUrl) // TODO: don't hardcode OS
	if site.Spec.PackageManagerUrl != "" {
		packageManagerRepoUrl = site.Spec.PackageManagerUrl
	}

	// VOLUMES

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

			if err := r.provisionFsxVolume(ctx, site, connectVolumeName, "connect", connectVolumeSize); err != nil {
				return ctrl.Result{}, err
			}

			if err := r.provisionFsxVolume(ctx, site, devVolumeName, "workbench", connectVolumeSize); err != nil {
				return ctrl.Result{}, err
			}

			// Provision shared storage volume for workbench load balancing
			workbenchSharedStorageVolumeName := fmt.Sprintf("%s-workbench-shared-storage", site.Name)
			// Note: provisionFsxVolume uses the volume name as the storage class name
			if err := r.provisionFsxVolume(ctx, site, workbenchSharedStorageVolumeName, "workbench-shared-storage", workbenchSharedStorageVolumeSize); err != nil {
				return ctrl.Result{}, err
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

			connectStorageClassName = fmt.Sprintf("%s-nfs", connectVolumeName)

			if err := r.provisionNfsVolume(ctx, site, connectVolumeName, "connect", connectStorageClassName, connectVolumeSize); err != nil {
				return ctrl.Result{}, err
			}

			devStorageClassName = fmt.Sprintf("%s-nfs", devVolumeName)

			if err := r.provisionNfsVolume(ctx, site, devVolumeName, "workbench", devStorageClassName, connectVolumeSize); err != nil {
				return ctrl.Result{}, err
			}

			// Provision shared storage volume for workbench load balancing
			workbenchSharedStorageVolumeName := fmt.Sprintf("%s-workbench-shared-storage", site.Name)
			workbenchSharedStorageClassName := fmt.Sprintf("%s-nfs", workbenchSharedStorageVolumeName)
			if err := r.provisionNfsVolume(ctx, site, workbenchSharedStorageVolumeName, "workbench-shared-storage", workbenchSharedStorageClassName, workbenchSharedStorageVolumeSize); err != nil {
				return ctrl.Result{}, err
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

	if err := r.reconcileFlightdeck(ctx, req, site); err != nil {
		l.Error(err, "error reconciling flightdeck")
		return ctrl.Result{}, err
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

	// PACKAGE MANAGER
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

	// WORKBENCH
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

	// CHRONICLE

	if err := r.reconcileChronicle(ctx, req, site); err != nil {
		l.Error(err, "error reconciling chronicle")
		return ctrl.Result{}, err
	}

	// KEYCLOAK

	if err := r.reconcileKeycloak(ctx, req, site, dbUrl, sslMode); err != nil {
		l.Error(err, "error reconciling keycloak")
		return ctrl.Result{}, err
	}

	// EXTRA SERVICE ACCOUNTS

	for _, s := range site.Spec.ExtraSiteServiceAccounts {
		serviceAccountName := fmt.Sprintf("%s-%s", site.Name, s.NameSuffix)
		serviceAccountKey := client.ObjectKey{
			Name:      serviceAccountName,
			Namespace: req.Namespace,
		}
		targetSa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:            serviceAccountName,
				Namespace:       req.Namespace,
				Labels:          site.KubernetesLabels(),
				Annotations:     s.Annotations,
				OwnerReferences: site.OwnerReferencesForChildren(),
			},
			Secrets:                      nil,
			ImagePullSecrets:             nil,
			AutomountServiceAccountToken: ptr.To(true),
		}
		existingSa := &corev1.ServiceAccount{}

		if err := internal.BasicCreateOrUpdate(ctx, r, l, serviceAccountKey, existingSa, targetSa); err != nil {
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
		Complete(r)
}
