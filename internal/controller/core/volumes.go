package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/internal"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

func (r *SiteReconciler) provisionFsxVolume(ctx context.Context, site *v1beta1.Site, name, subdir string, size resource.Quantity) error {
	l := r.GetLogger(ctx).WithValues(
		"event", fmt.Sprintf("%s-fsx-volume", name),
	)
	var subdirPath string
	if subdir == "" {
		subdirPath = "fsx/"
	} else if subdir == site.Name {
		// NOTE: this prevents a path like fsx/site.Name/site.Name
		subdirPath = fmt.Sprintf("fsx/%s", site.Name)
	} else {
		subdirPath = fmt.Sprintf("fsx/%s/%s", site.Name, subdir)
	}
	l.Info("Provisioning FSx OpenZFS Volume", "name", name, "path", subdirPath)
	targetPv := &v1.PersistentVolume{
		ObjectMeta: v12.ObjectMeta{
			Name:            name,
			Labels:          site.KubernetesLabels(),
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				"storage": size,
			},
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			StorageClassName: name,
			MountOptions: []string{
				"nfsvers=4.2", "rsize=1048576", "wsize=1048576", "timeo=600",
			},
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       "fsx.openzfs.csi.aws.com",
					VolumeHandle: site.Spec.VolumeSource.VolumeId,
					ReadOnly:     false,
					VolumeAttributes: map[string]string{
						"DNSName":      site.Spec.VolumeSource.DnsName,
						"VolumePath":   subdirPath,
						"ResourceType": "volume",
					},
				},
			},
		},
	}

	existingPv := &v1.PersistentVolume{}

	pvKey := client.ObjectKey{Name: name}

	l.WithValues("key", pvKey).Info("Creating or updating volume")

	if err := internal.BasicCreateNoUpdate(ctx, r, l, pvKey, existingPv, targetPv); err != nil {
		l.Error(err, fmt.Sprintf("Error creating PersistentVolume for %s", name))
		return err
	}
	l.Info("Done provisioning FSx OpenZFS Volume for " + name)
	return nil
}

func (r *SiteReconciler) provisionRootFsxVolume(ctx context.Context, site *v1beta1.Site) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "root-fsx-volume",
	)

	l.Info("Provisioning FSX OpenZFS root volume")
	targetRootPv := &v1.PersistentVolume{
		ObjectMeta: v12.ObjectMeta{
			Name:            site.Name,
			Labels:          site.KubernetesLabels(),
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				"storage": rootVolumeSize,
			},
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			StorageClassName: site.Name,
			MountOptions: []string{
				"nfsvers=4.2", "rsize=1048576", "wsize=1048576", "timeo=600",
			},
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       "fsx.openzfs.csi.aws.com",
					VolumeHandle: site.Spec.VolumeSource.VolumeId,
					ReadOnly:     false,
					VolumeAttributes: map[string]string{
						"DNSName":      site.Spec.VolumeSource.DnsName,
						"VolumePath":   "fsx/",
						"ResourceType": "volume",
					},
				},
			},
		},
	}

	existingRootPv := &v1.PersistentVolume{}

	key := client.ObjectKey{Name: site.Name}

	l.WithValues("key", key).Info("Creating or updating volume")

	if err := internal.BasicCreateNoUpdate(ctx, r, l, key, existingRootPv, targetRootPv); err != nil {
		l.Error(err, "Error creating site PersistentVolume")
		return err
	}

	l.Info("Done provisioning FSX OpenZFS Root volume")

	return nil
}

func (r *SiteReconciler) provisionNfsVolume(ctx context.Context, site *v1beta1.Site, name, subdir, storageClassName string, size resource.Quantity) error {
	l := r.GetLogger(ctx).WithValues(
		"event", fmt.Sprintf("%s-nfs-volume", name),
	)

	rootNFSPath := guessRootNFSPathFromDNSName(site.Spec.VolumeSource.DnsName)
	subdirPath := rootNFSPath

	if subdir == site.Name {
		// NOTE: this prevents a path like fsx/site.Name/site.Name
		subdirPath = filepath.Join(rootNFSPath, site.Name)
	} else if subdir != "" {
		subdirPath = filepath.Join(rootNFSPath, site.Name, subdir)
	}

	l.Info("Provisioning NFS Volume", "name", name, "path", subdirPath)

	targetPv := &v1.PersistentVolume{
		ObjectMeta: v12.ObjectMeta{
			Name:            name,
			Labels:          site.KubernetesLabels(),
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				"storage": size,
			},
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			StorageClassName: storageClassName,
			MountOptions: []string{
				"nfsvers=4.1", "rsize=1048576", "wsize=1048576", "timeo=600",
			},
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Server:   site.Spec.VolumeSource.DnsName,
					Path:     subdirPath,
					ReadOnly: false,
				},
			},
		},
	}

	existingPv := &v1.PersistentVolume{}

	pvKey := client.ObjectKey{Name: name}

	l.WithValues("key", pvKey).Info("Creating or updating volume")

	if err := internal.BasicCreateNoUpdate(ctx, r, l, pvKey, existingPv, targetPv); err != nil {
		l.Error(err, fmt.Sprintf("Error creating PersistentVolume for %s", name))
		return err
	}

	l.Info("Done provisioning NFS Volume for " + name)

	return nil
}

func (r *SiteReconciler) provisionRootNfsVolume(ctx context.Context, site *v1beta1.Site) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "root-nfs-volume",
	)

	l.Info("Provisioning NFS root volume")

	targetRootPv := &v1.PersistentVolume{
		ObjectMeta: v12.ObjectMeta{
			Name:            site.Name,
			Labels:          site.KubernetesLabels(),
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				"storage": rootVolumeSize,
			},
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			StorageClassName: fmt.Sprintf("%s-nfs", site.Name),
			MountOptions: []string{
				"nfsvers=4.1", "rsize=1048576", "wsize=1048576", "timeo=600",
			},
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Server:   site.Spec.VolumeSource.DnsName,
					Path:     guessRootNFSPathFromDNSName(site.Spec.VolumeSource.DnsName),
					ReadOnly: false,
				},
			},
		},
	}

	existingRootPv := &v1.PersistentVolume{}

	key := client.ObjectKey{Name: site.Name}

	l.WithValues("key", key).Info("Creating or updating volume")

	if err := internal.BasicCreateNoUpdate(ctx, r, l, key, existingRootPv, targetRootPv); err != nil {
		l.Error(err, "Error creating site PersistentVolume")
		return err
	}

	l.Info("Done provisioning NFS Root volume")

	return nil
}

func guessRootNFSPathFromDNSName(dnsName string) string {
	if strings.HasPrefix(dnsName, "fs-") &&
		strings.Contains(dnsName, ".fsx.") {

		return "/fsx"
	}

	return "/"
}

// provisionVolumeViaPVC creates a PVC that references a StorageClass by name.
// The StorageClass must be pre-created by the infrastructure layer.
// This is the cloud-agnostic volume provisioning path.
func (r *SiteReconciler) provisionVolumeViaPVC(ctx context.Context, site *v1beta1.Site, name, subdir, storageClassName string, size resource.Quantity) error {
	l := r.GetLogger(ctx).WithValues(
		"event", fmt.Sprintf("%s-pvc-volume", name),
	)

	l.Info("Provisioning volume via PVC", "name", name, "storageClass", storageClassName, "subdir", subdir)

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: v12.ObjectMeta{
			Name:      name,
			Namespace: site.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		// nfs.io/storage-path is maintained on every reconcile so provisioner
		// can restore it if removed from the cluster.
		if pvc.Annotations == nil {
			pvc.Annotations = make(map[string]string)
		}
		pvc.Annotations["nfs.io/storage-path"] = fmt.Sprintf("%s/%s", site.Name, subdir)

		// Merge labels on every reconcile so new label keys are picked up by existing PVCs.
		// Note: this is additive only; keys removed from KubernetesLabels() will remain on
		// live PVCs until manually pruned.
		if pvc.Labels == nil {
			pvc.Labels = make(map[string]string)
		}
		for k, v := range site.KubernetesLabels() {
			pvc.Labels[k] = v
		}

		// Only set immutable fields on creation
		if pvc.ResourceVersion == "" {
			pvc.Spec = v1.PersistentVolumeClaimSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				StorageClassName: &storageClassName,
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: size,
					},
				},
			}
		}
		return controllerutil.SetControllerReference(site, pvc, r.Scheme)
	})

	if err != nil {
		l.Error(err, fmt.Sprintf("Error creating or updating PVC for %s", name))
		return err
	}

	l.Info("Done provisioning volume via PVC for " + name)
	return nil
}
