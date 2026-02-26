package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

// baseEgressRuleCount is the number of egress rules in the baseline policy:
// one for the parent workbench host, one for public internet.
const baseEgressRuleCount = 2

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

	assert.Len(t, policy.Spec.Egress, baseEgressRuleCount+1, "expected NFS egress rule to be added")

	// Verify the NFS rule targets the correct CIDR and restricts to port 2049
	var nfsCIDR string
	var nfsPort int32
	for _, rule := range policy.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && peer.IPBlock.CIDR == "10.0.0.0/8" {
				nfsCIDR = peer.IPBlock.CIDR
				if len(rule.Ports) > 0 && rule.Ports[0].Port != nil {
					nfsPort = rule.Ports[0].Port.IntVal
				}
			}
		}
	}
	assert.Equal(t, "10.0.0.0/8", nfsCIDR, "NFS egress rule must use NFSEgressCIDR")
	assert.Equal(t, int32(2049), nfsPort, "NFS egress rule must restrict to port 2049")
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

	assert.Len(t, policy.Spec.Egress, baseEgressRuleCount+1, "expected NFS egress rule via legacy EFS path")

	// Verify the NFS rule targets the VPC CIDR and restricts to port 2049
	var nfsCIDR string
	var nfsPort int32
	for _, rule := range policy.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && peer.IPBlock.CIDR == "172.16.0.0/12" {
				nfsCIDR = peer.IPBlock.CIDR
				if len(rule.Ports) > 0 && rule.Ports[0].Port != nil {
					nfsPort = rule.Ports[0].Port.IntVal
				}
			}
		}
	}
	assert.Equal(t, "172.16.0.0/12", nfsCIDR, "NFS egress rule must use VPCCIDR via legacy path")
	assert.Equal(t, int32(2049), nfsPort, "NFS egress rule must restrict to port 2049")
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

	assert.Len(t, policy.Spec.Egress, baseEgressRuleCount, "no NFS egress rule expected")
}

// TestWorkbenchSessionNetworkPolicy_NFSCIDRTakesPrecedence verifies that NFSEgressCIDR
// takes precedence over EFSEnabled+VPCCIDR when both are set, producing exactly one NFS rule.
func TestWorkbenchSessionNetworkPolicy_NFSCIDRTakesPrecedence(t *testing.T) {
	ctx, r, cli := initSiteReconciler(t)
	ns := "posit-team"
	site := defaultSite("mysite-both")
	site.Spec.NFSEgressCIDR = "10.0.0.0/8"
	site.Spec.EFSEnabled = true
	site.Spec.VPCCIDR = "172.16.0.0/12"

	require.NoError(t, cli.Create(ctx, site))

	l := r.GetLogger(ctx)
	require.NoError(t, r.reconcileWorkbenchSessionNetworkPolicy(ctx, ns, l, site))

	policy := &networkingv1.NetworkPolicy{}
	policyName := site.Name + "-workbench-session"
	require.NoError(t, cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: policyName}, policy))

	// NFSEgressCIDR takes precedence: only one NFS rule is added
	assert.Len(t, policy.Spec.Egress, baseEgressRuleCount+1, "only one NFS egress rule expected when both paths configured")

	// The rule must use NFSEgressCIDR, not VPCCIDR
	var nfsCIDR string
	for _, rule := range policy.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && (peer.IPBlock.CIDR == "10.0.0.0/8" || peer.IPBlock.CIDR == "172.16.0.0/12") {
				nfsCIDR = peer.IPBlock.CIDR
			}
		}
	}
	assert.Equal(t, "10.0.0.0/8", nfsCIDR, "NFSEgressCIDR must take precedence over VPCCIDR")
}
