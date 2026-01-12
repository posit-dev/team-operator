package product

import (
	"maps"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

type VolumeMountDef struct {
	MountPath string
	SubPath   string
	ReadOnly  bool
}

type VolumeDef struct {
	Name   string
	Source *corev1.VolumeSource
	Mounts []*VolumeMountDef
	// Env is stored on VolumeDef because this is often the scope that env vars are defined and thought about...
	Env []corev1.EnvVar
}

type MultiContainerVolumeFactory struct {
	SecretVolumeFactory
	SidecarDefs       map[string]*ContainerDef
	InitContainerDefs map[string]*ContainerDef
}

func (m *MultiContainerVolumeFactory) InitContainers() []corev1.Container {
	containers := []corev1.Container{}
	for k, s := range m.InitContainerDefs {
		containers = append(containers, corev1.Container{
			Name:            k,
			Image:           s.Image,
			Env:             s.Env,
			VolumeMounts:    s.VolumeMounts(),
			Resources:       s.Resources,
			ImagePullPolicy: s.ImagePullPolicy,
			SecurityContext: s.SecurityContext,
		})
	}
	return containers
}

func (m *MultiContainerVolumeFactory) Sidecars() []corev1.Container {
	containers := []corev1.Container{}
	for k, s := range m.SidecarDefs {
		containers = append(containers, corev1.Container{
			Name:            k,
			Image:           s.Image,
			Env:             s.Env,
			VolumeMounts:    s.VolumeMounts(),
			Resources:       s.Resources,
			ImagePullPolicy: s.ImagePullPolicy,
			SecurityContext: s.SecurityContext,
		})
	}
	return containers
}

type ContainerDef struct {
	Image           string
	Env             []corev1.EnvVar
	Mounts          map[string]*VolumeMountDef
	Resources       corev1.ResourceRequirements
	ImagePullPolicy corev1.PullPolicy
	SecurityContext *corev1.SecurityContext
}

type CSIDef struct {
	Driver           string
	ReadOnly         *bool
	VolumeAttributes map[string]string
	DummyVolumeMount []*VolumeMountDef
}

type VolumeFactoryInterface interface {
	Volumes() []corev1.Volume
	VolumeMounts() []corev1.VolumeMount

	EnvVars() []corev1.EnvVar
}

type VolumeFactory struct {
	Vols map[string]*VolumeDef
}

func (v *VolumeFactory) Volumes() []corev1.Volume {
	vols := []corev1.Volume{}
	for n, vol := range v.Vols {
		vols = append(vols, corev1.Volume{
			Name:         n,
			VolumeSource: *vol.Source,
		})
	}
	sort.Slice(vols, func(i, j int) bool {
		return vols[i].Name < vols[j].Name
	})
	return vols
}

func (v *VolumeFactory) VolumeMounts() []corev1.VolumeMount {
	vms := []corev1.VolumeMount{}
	for n, vol := range v.Vols {
		for _, vm := range vol.Mounts {
			vms = append(vms, corev1.VolumeMount{
				Name:      n,
				ReadOnly:  vm.ReadOnly,
				MountPath: vm.MountPath,
				SubPath:   vm.SubPath,
			})
		}
	}
	sort.Slice(vms, func(i, j int) bool {
		return vms[i].Name < vms[j].Name
	})
	return vms
}

func (v *VolumeFactory) EnvVars() []corev1.EnvVar {
	vars := []corev1.EnvVar{}
	keys := []string{}

	// ensure deterministic ordering
	for k, _ := range v.Vols {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		vars = ConcatLists(vars, v.Vols[k].Env)
	}
	return vars
}

// SecretVolumeFactory will have a special Volumes() method that ensures at least one CSI volume is created
type SecretVolumeFactory struct {
	// Vols is the "name" to VolumeDef mapping that makes volume provisioning possible
	Vols map[string]*VolumeDef

	Env []corev1.EnvVar

	CsiEntries map[string]*CSIDef

	CsiAllSecrets map[string]string

	CsiKubernetesSecrets map[string]map[string]string
}

func (v *SecretVolumeFactory) EnvVars() []corev1.EnvVar {
	vars := []corev1.EnvVar{}
	keys := []string{}

	// ensure deterministic ordering
	for k, _ := range v.Vols {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		vars = ConcatLists(vars, v.Vols[k].Env)
	}
	vars = ConcatLists(vars, v.Env)
	return vars
}

func (v *SecretVolumeFactory) Volumes() []corev1.Volume {
	csiEntriesComplete := []CSIDef{}
	vols := []corev1.Volume{}
	for n, vol := range v.Vols {
		if vol.Source != nil {
			vols = append(vols, corev1.Volume{
				Name:         n,
				VolumeSource: *vol.Source,
			})

			if vol.Source.CSI != nil {
				// add CSI entry to those completed
				csiEntriesComplete = append(csiEntriesComplete,
					CSIDef{
						Driver:           vol.Source.CSI.Driver,
						ReadOnly:         vol.Source.CSI.ReadOnly,
						VolumeAttributes: vol.Source.CSI.VolumeAttributes,
					})
			}
		}
	}

	// check that all CSI entries are accounted for
	for key, csi := range v.CsiEntries {
		isComplete := false
		for _, done := range csiEntriesComplete {
			if done.Driver == csi.Driver &&
				maps.Equal(done.VolumeAttributes, csi.VolumeAttributes) {
				// this is done!
				isComplete = true
				break
			}
		}
		if isComplete {
			// go to the next one
			continue
		}
		// otherwise, create a dummy volume
		vols = append(vols, corev1.Volume{
			Name: key,
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:           csi.Driver,
					ReadOnly:         csi.ReadOnly,
					VolumeAttributes: csi.VolumeAttributes,
				},
			},
		})
	}

	sort.Slice(vols, func(i, j int) bool {
		return vols[i].Name < vols[j].Name
	})
	return vols
}

// VolumeMountHash unique-ifies a mount, which does not need a unique name
func VolumeMountHash(mount corev1.VolumeMount) string {
	return mount.Name + "||" + mount.MountPath + "||" + mount.SubPath + "||" + strconv.FormatBool(mount.ReadOnly)
}

func (v *SecretVolumeFactory) VolumeMounts() []corev1.VolumeMount {
	csiEntriesComplete := []CSIDef{}
	vms := []corev1.VolumeMount{}
	for n, vol := range v.Vols {
		for _, vm := range vol.Mounts {
			vms = append(vms, corev1.VolumeMount{
				Name:      n,
				ReadOnly:  vm.ReadOnly,
				MountPath: vm.MountPath,
				SubPath:   vm.SubPath,
			})
		}
		if vol.Source != nil {
			if vol.Source.CSI != nil {
				csiEntriesComplete = append(csiEntriesComplete,
					CSIDef{
						Driver:           vol.Source.CSI.Driver,
						ReadOnly:         vol.Source.CSI.ReadOnly,
						VolumeAttributes: vol.Source.CSI.VolumeAttributes,
					})
			}
		}
	}

	// check that all CSI entries are accounted for
	for key, csi := range v.CsiEntries {
		isComplete := false
		for _, done := range csiEntriesComplete {
			if done.Driver == csi.Driver &&
				maps.Equal(done.VolumeAttributes, csi.VolumeAttributes) {
				// this is done!
				isComplete = true
				break
			}
		}
		if isComplete {
			// go to the next one
			continue
		}
		// otherwise, create a dummy volume mount. We do not know what secret to mount here...

		if len(csi.DummyVolumeMount) > 0 {
			for _, dum := range csi.DummyVolumeMount {
				vms = append(vms, corev1.VolumeMount{
					Name:      key,
					ReadOnly:  dum.ReadOnly,
					MountPath: dum.MountPath,
					SubPath:   dum.SubPath,
				})
			}
		} else {
			// we don't have dummies... so just mount everything (no subpath)
			vms = append(vms, corev1.VolumeMount{
				Name:      key,
				ReadOnly:  true,
				MountPath: "/mnt/dummy",
			})
		}
	}

	sort.Slice(vms, func(i, j int) bool {
		return VolumeMountHash(vms[i]) < VolumeMountHash(vms[j])
	})
	return vms
}

func EmptyDirVolumeDef(mountPath string) *VolumeDef {
	return &VolumeDef{
		Source: &corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
		Mounts: []*VolumeMountDef{
			{mountPath, "", false},
		},
	}
}
