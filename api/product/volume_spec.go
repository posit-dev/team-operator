package product

//+k8s:openapi-gen=true
//+genclient
// +kubebuilder:object:generate=true

// VolumeSpec is a specification for a PersistentVolumeClaim to be created (and/or mounted)
type VolumeSpec struct {
	// Create determines whether the PVC should be created or not
	Create bool `json:"create,omitempty"`
	// AccessModes is the access mode for the created PVC. Only used if Create is true
	AccessModes []string `json:"accessModes,omitempty"`
	// VolumeName is the name of the PV that will be referenced by the created PVC. Only used if Create is true
	VolumeName string `json:"volumeName,omitempty"`
	// StorageClassName identifies the StorageClassName created by the PV. Only used if Create is true
	StorageClassName string `json:"storageClassName,omitempty"`
	// Size is the size of the PVC that is being created. Only used if Create is true
	Size string `json:"size,omitempty"`

	// PvcName is used only if Create is false. The idea is that the PVC has already been created. Only used if Create is false
	PvcName string `json:"pvcName,omitempty"`

	// MountPath is not always used. It is only used for volumes that are configurable... (i.e. additionalVolumes)
	MountPath string `json:"mountPath,omitempty"`

	// ReadOnly defaults to false and determines whether the volume is mounted ReadOnly
	ReadOnly bool `json:"readOnly,omitempty"`
}
