package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestWorkbenchSessionNetworkPolicy_NFSEgressCIDR verifies that setting NFSEgressCIDR
// adds an NFS egress rule for port 2049.
func TestWorkbenchSessionNetworkPolicy_NFSEgressCIDR(t *testing.T) {
	ctx, r, cli := initSiteReconciler(t)
	ns := "posit-team"
	site := defaultSite("mysite-nfs")
	site.Spec.NFSEgressCIDR = "10.0.0.0/8"

	require.NoError(t, cli.Create(ctx, site))

	l := r.GetLogger(ctx)
	require.NoError(t, r.reconcileWorkbenchSessionNetworkPolicy(ctx, ns, l, site))

	policy := &networkingv1.NetworkPolicy{}
	policyName := site.Name + "-workbench-session"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: policyName}, policy))

	// With NFSEgressCIDR set, there should be 3 egress rules (workbench host, public internet, NFS)
	assert.Len(t, policy.Spec.Egress, 3, "expected NFS egress rule to be added")

	// Verify the NFS rule targets the correct CIDR
	var nfsCIDR string
	for _, rule := range policy.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && peer.IPBlock.CIDR == "10.0.0.0/8" {
				nfsCIDR = peer.IPBlock.CIDR
			}
		}
	}
	assert.Equal(t, "10.0.0.0/8", nfsCIDR, "NFS egress rule must use NFSEgressCIDR")
}

// TestWorkbenchSessionNetworkPolicy_LegacyEFSPath verifies that EFSEnabled+VPCCIDR
// adds an NFS egress rule when NFSEgressCIDR is unset.
func TestWorkbenchSessionNetworkPolicy_LegacyEFSPath(t *testing.T) {
	ctx, r, cli := initSiteReconciler(t)
	ns := "posit-team"
	site := defaultSite("mysite-efs")
	site.Spec.EFSEnabled = true
	site.Spec.VPCCIDR = "172.16.0.0/12"

	require.NoError(t, cli.Create(ctx, site))

	l := r.GetLogger(ctx)
	require.NoError(t, r.reconcileWorkbenchSessionNetworkPolicy(ctx, ns, l, site))

	policy := &networkingv1.NetworkPolicy{}
	policyName := site.Name + "-workbench-session"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: policyName}, policy))

	// With EFSEnabled+VPCCIDR, there should be 3 egress rules
	assert.Len(t, policy.Spec.Egress, 3, "expected NFS egress rule via legacy EFS path")

	// Verify the NFS rule targets the VPC CIDR
	var nfsCIDR string
	for _, rule := range policy.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && peer.IPBlock.CIDR == "172.16.0.0/12" {
				nfsCIDR = peer.IPBlock.CIDR
			}
		}
	}
	assert.Equal(t, "172.16.0.0/12", nfsCIDR, "NFS egress rule must use VPCCIDR via legacy path")
}

// TestWorkbenchSessionNetworkPolicy_NoNFSRule verifies that no extra NFS egress rule
// is added when neither NFSEgressCIDR nor EFSEnabled+VPCCIDR is configured.
func TestWorkbenchSessionNetworkPolicy_NoNFSRule(t *testing.T) {
	ctx, r, cli := initSiteReconciler(t)
	ns := "posit-team"
	site := defaultSite("mysite-nonfs")

	require.NoError(t, cli.Create(ctx, site))

	l := r.GetLogger(ctx)
	require.NoError(t, r.reconcileWorkbenchSessionNetworkPolicy(ctx, ns, l, site))

	policy := &networkingv1.NetworkPolicy{}
	policyName := site.Name + "-workbench-session"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: policyName}, policy))

	// Without NFS config, there should be exactly 2 egress rules (workbench host, public internet)
	assert.Len(t, policy.Spec.Egress, 2, "no NFS egress rule expected")
}
