package internal

import (
	networkingv1 "k8s.io/api/networking/v1"
)

// PublicInternetNetworkPolicyEgressRule generates a NetworkPolicyEgressRule that is
// suitable for untrusted workloads that needs to access public internet resources and other
// componenents within the Calico private network.
func PublicInternetNetworkPolicyEgressRule() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				IPBlock: &networkingv1.IPBlock{
					CIDR: "0.0.0.0/0",
					Except: []string{
						// NOTE: explicitly blocking these CIDRs means that we are also
						// depending on Calico using 172.16.0.0/16 so that those addresses
						// are still reachable.
						"192.168.0.0/16",
						"10.0.0.0/8",
					},
				},
			},
		},
	}
}

// PublicInternetOnlyNetworkPolicyEgressRule generates a NetworkPolicyEgressRule that is
// suitable for untrusted workloads that needs to access public internet resources only and
// NO other private resources.
func PublicInternetOnlyNetworkPolicyEgressRule() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				IPBlock: &networkingv1.IPBlock{
					CIDR: "0.0.0.0/0",
					Except: []string{
						// this blocks ALL private network communication
						"192.168.0.0/16",
						"10.0.0.0/8",
						"172.16.0.0/12",
					},
				},
			},
		},
	}
}
