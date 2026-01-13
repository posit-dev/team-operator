package core

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/rstudio/goex/ptr"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

var subdirProvisionerScript = `#!/bin/bash

for dir in "$@"; do
  echo "Creating directory: ${dir}"
  mkdir -p ${dir}
done
`

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func (r *SiteReconciler) provisionSubDirectoryCreator(ctx context.Context, req ctrl.Request, site *v1beta1.Site, storageClassName string) error {

	l := r.GetLogger(ctx).WithValues(
		"event", "create-subdirectory-service",
	)

	l.Info("Provisioning subdirectory creator job")

	provisionerName := site.Name + "-subdir"

	provisionerKey := client.ObjectKey{Name: provisionerName, Namespace: req.Namespace}

	targetPvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            provisionerName,
			Namespace:       req.Namespace,
			Labels:          site.KubernetesLabels(),
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteMany,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					"storage": rootVolumeSize,
				},
			},
			StorageClassName: ptr.To(storageClassName),
			VolumeName:       site.Name,
		},
	}
	existingPvc := &v1.PersistentVolumeClaim{}

	if err := internal.PvcCreateOrUpdate(ctx, r, l, provisionerKey, existingPvc, targetPvc); err != nil {
		l.Error(err, "Error creating site persistent volume claim")
		return err
	}
	targetProvisionerConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            provisionerName,
			Namespace:       req.Namespace,
			Labels:          site.KubernetesLabels(),
			OwnerReferences: site.OwnerReferencesForChildren(),
		},
		Data: map[string]string{
			"subdir-provisioner.sh": subdirProvisionerScript,
		},
	}

	existingProvisionerConfigMap := &v1.ConfigMap{}

	if err := internal.BasicCreateOrUpdate(ctx, r, l, provisionerKey, existingProvisionerConfigMap, targetProvisionerConfigMap); err != nil {
		l.Error(err, "Error creating subdir provisioner configmap")
		return err
	}

	// TODO: some way to ensure that we do not spawn _too many_ jobs...?

	if !site.Spec.VolumeSubdirJobOff {
		args := []string{
			fmt.Sprintf("/mnt/%s/connect", site.Name),
			fmt.Sprintf("/mnt/%s/workbench", site.Name),
			fmt.Sprintf("/mnt/%s/workbench-shared-storage", site.Name),
		}
		if site.Spec.SharedDirectory != "" {
			args = append(args, fmt.Sprintf("/mnt/%s/shared", site.Name))
		}
		provisionerNameTemp := provisionerName + "-" + RandStringBytes(6)
		provisionerTempKey := client.ObjectKey{Name: provisionerNameTemp, Namespace: req.Namespace}
		targetProvisionerJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:            provisionerNameTemp,
				Namespace:       req.Namespace,
				Labels:          site.KubernetesLabels(),
				OwnerReferences: site.OwnerReferencesForChildren(),
			},
			Spec: batchv1.JobSpec{
				// 2 hours to live
				TTLSecondsAfterFinished: ptr.To(int32(2 * 60 * 60)),
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						EnableServiceLinks: ptr.To(false),
						RestartPolicy:      v1.RestartPolicyOnFailure,
						Containers: []v1.Container{
							{
								Name:  "subdir-maker",
								Image: "ghcr.io/rstudio/rstudio-workbench-preview:jammy-daily",
								Command: []string{
									"/subdir-provisioner.sh",
								},
								Args: args,
								VolumeMounts: []v1.VolumeMount{
									{
										Name:      "exec-script",
										ReadOnly:  false,
										MountPath: "/subdir-provisioner.sh",
										SubPath:   "subdir-provisioner.sh",
									},
									{
										Name:      "data-volume",
										ReadOnly:  false,
										MountPath: "/mnt/",
									},
								},
							},
						},
						SecurityContext: &v1.PodSecurityContext{
							RunAsUser: ptr.To(int64(0)),
						},
						Volumes: []v1.Volume{
							{
								Name: "exec-script",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: provisionerName,
										},
										Items: []v1.KeyToPath{
											{
												Key:  "subdir-provisioner.sh",
												Path: "subdir-provisioner.sh",
											},
										},
										DefaultMode: ptr.To(product.MustParseOctal("755")),
									},
								},
							},
							{
								Name: "data-volume",
								VolumeSource: v1.VolumeSource{
									PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
										ClaimName: provisionerName,
										ReadOnly:  false,
									},
								},
							},
						},
					},
				},
			},
		}

		existingProvisionerJob := &batchv1.Job{}
		if err := internal.BasicCreateOrUpdate(ctx, r, l, provisionerTempKey, existingProvisionerJob, targetProvisionerJob); err != nil {
			l.Error(err, "Error creating provisioner job")
			return err
		}
	}
	return nil
}
