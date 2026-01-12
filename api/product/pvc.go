package product

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// DefinePvc provisions a PVC
func DefinePvc(p KubernetesOwnerProvider, req ctrl.Request, name string, vol *VolumeSpec, defaultVolumeSize resource.Quantity) (*corev1.PersistentVolumeClaim, error) {
	if vol == nil {
		return nil, errors.New("the VolumeSpec must be defined. Got nil")
	}
	var accessModes []corev1.PersistentVolumeAccessMode
	if vol.AccessModes != nil && len(vol.AccessModes) > 0 {
		for _, a := range vol.AccessModes {
			accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(a))
		}
	} else {
		// presume ReadWriteOnce
		accessModes = append(accessModes, corev1.ReadWriteOnce)
	}

	volumeSize := defaultVolumeSize
	if vol.Size != "" {
		// TODO: could use this as spec validation to bring the error closer to the user
		if v, err := resource.ParseQuantity(vol.Size); err != nil {
			return nil, err
		} else {
			// valid, use the volume
			volumeSize = v
		}
	}

	targetPvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       req.Namespace,
			Labels:          p.KubernetesLabels(),
			OwnerReferences: p.OwnerReferencesForChildren(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": volumeSize,
				},
			},
			VolumeName: vol.VolumeName,
		},
	}

	// set storage class name if it is provided
	// this allows us to fall back to the default if not
	if vol.StorageClassName != "" {
		targetPvc.Spec.StorageClassName = &vol.StorageClassName
	}

	return targetPvc, nil
}
