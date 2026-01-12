package product

import (
	"sort"

	v1 "k8s.io/api/core/v1"
)

func ChronicleSidecar(p Product, env []v1.EnvVar) []v1.Container {
	vm := []v1.VolumeMount{}
	// make a volume mount if using workbench
	if p.ShortName() == "dev" {
		vm = ConcatLists(vm, ChronicleVolumeMountsFromProduct(p))
	}
	url := p.GetChronicleUrl()
	img := p.GetChronicleAgentImage()
	if img == "" || url == "" {
		// TODO: warn or something so folks are aware we are skipping because of an undefined image
		return []v1.Container{}
	}
	return []v1.Container{
		{
			Name:  "chronicle",
			Image: img,
			Env: ConcatLists([]v1.EnvVar{
				{
					Name:  "CHRONICLE_CONNECT_METRICS_URL",
					Value: "http://localhost:3232/metrics",
				},
				{
					Name:  "CHRONICLE_SERVER_ADDRESS",
					Value: url,
				},
			}, env),
			VolumeMounts:    vm,
			Resources:       v1.ResourceRequirements{},
			ImagePullPolicy: "",
			SecurityContext: nil,
		},
	}
}

func ChronicleVolumes() (vols []v1.Volume) {
	vols = append(vols, v1.Volume{
		Name: "logs",
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	})
	return vols
}

func ChronicleVolumeMountsFromProduct(p Product) (vms []v1.VolumeMount) {
	vmPath := "/tmp"
	switch p.ShortName() {
	case "pub":
		vmPath = "/tmp"
	case "dev":
		vmPath = "/var/lib/rstudio-server/audit"
	}
	vms = append(vms, v1.VolumeMount{
		Name:      "logs",
		MountPath: vmPath,
	})
	return vms
}

func (v *ContainerDef) VolumeMounts() (vms []v1.VolumeMount) {
	for k, vm := range v.Mounts {
		vms = append(vms, v1.VolumeMount{
			Name:      k,
			ReadOnly:  vm.ReadOnly,
			MountPath: vm.MountPath,
			SubPath:   vm.SubPath,
		})
	}

	sort.Slice(vms, func(i, j int) bool {
		return VolumeMountHash(vms[i]) < VolumeMountHash(vms[j])
	})
	return vms
}

func CreateChronicleWorkbenchVolumeFactory(p Product, env []v1.EnvVar) MultiContainerVolumeFactory {
	vols := map[string]*VolumeDef{}
	sidecarMountDefs := map[string]*VolumeMountDef{}
	url := p.GetChronicleUrl()
	img := p.GetChronicleAgentImage()
	sidecarDefs := map[string]*ContainerDef{}

	if p.ShortName() == "dev" {
		// add a volume mount between sidecar and the main container
		vm := &VolumeMountDef{MountPath: "/var/lib/rstudio-server/audit"}
		vols["chronicle-logs"] = &VolumeDef{
			Source: &v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
			Mounts: []*VolumeMountDef{vm},
		}
		sidecarMountDefs["chronicle-logs"] = vm
	}
	// if img and url are defined, create the sidecar
	if img != "" && url != "" {
		sidecarDefs["chronicle"] = &ContainerDef{
			Image: img,
			Env: ConcatLists([]v1.EnvVar{
				{
					Name:  "CHRONICLE_SERVER_ADDRESS",
					Value: url,
				},
			}, env),
			Mounts:          sidecarMountDefs,
			Resources:       v1.ResourceRequirements{},
			ImagePullPolicy: "",
			SecurityContext: nil,
		}
	}

	return MultiContainerVolumeFactory{
		SecretVolumeFactory: SecretVolumeFactory{
			Vols: vols,
		},
		SidecarDefs: sidecarDefs,
	}
}
