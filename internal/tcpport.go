package internal

import (
	"github.com/rstudio/goex/ptr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultPortHTTPS    TCPPort = 443
	DefaultPortPostgres TCPPort = 5432

	DefaultPortChronicleHTTP         TCPPort = 5252
	DefaultPortChronicleMetrics      TCPPort = 3030
	DefaultPortConnectHTTP           TCPPort = 3939
	DefaultPortConnectMetrics        TCPPort = 3232
	DefaultPortConnectSession        TCPPort = 50734
	DefaultPortHomeHTTP              TCPPort = 8080
	DefaultPortKeycloakHTTP          TCPPort = 8080
	DefaultPortKeycloakHTTPS         TCPPort = 8443
	DefaultPortLauncher              TCPPort = 5559
	DefaultPortPackageManagerHTTP    TCPPort = 4242
	DefaultPortPackageManagerMetrics TCPPort = 2112
	DefaultPortWorkbenchHTTP         TCPPort = 8787
	DefaultPortWorkbenchMetrics      TCPPort = 8989
	DefaultPortWorkbenchSessionHTTP  TCPPort = 8788
	DefaultPortWorkbenchSessionProxy TCPPort = 8789
)

type TCPPort int32

func (p TCPPort) NetworkPolicyPort() networkingv1.NetworkPolicyPort {
	return networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt(int(p))),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}
}

func (p TCPPort) ContainerPort(name string) corev1.ContainerPort {
	return corev1.ContainerPort{
		Name:          name,
		ContainerPort: int32(p),
	}
}
