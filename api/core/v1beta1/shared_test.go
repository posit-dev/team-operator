package v1beta1_test

import (
	"testing"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/stretchr/testify/require"
)

type fakeKubernetesLabeler struct{}

func (fkl *fakeKubernetesLabeler) KubernetesLabels() map[string]string {
	return map[string]string{
		v1beta1.KubernetesInstanceLabelKey: "fiona",
		v1beta1.SiteLabelKey:               "tfs",
	}
}

func (fkl *fakeKubernetesLabeler) SelectorLabels() map[string]string {
	return map[string]string{
		v1beta1.ManagedByLabelKey:      v1beta1.ManagedByLabelValue,
		v1beta1.KubernetesNameLabelKey: "fakey-mc-fakerson",
	}
}

func TestComponentSpecPodAntiAffinity(t *testing.T) {
	r := require.New(t)

	cpaa := v1beta1.ComponentSpecPodAntiAffinity(&fakeKubernetesLabeler{}, "larping")

	r.Len(cpaa.PreferredDuringSchedulingIgnoredDuringExecution, 1)

	wpat0 := cpaa.PreferredDuringSchedulingIgnoredDuringExecution[0]

	r.Equal(int32(1), wpat0.Weight)
	r.Equal("kubernetes.io/hostname", wpat0.PodAffinityTerm.TopologyKey)
	r.Equal([]string{"larping"}, wpat0.PodAffinityTerm.Namespaces)

	melsKeys := []string{}

	for _, lsr := range wpat0.PodAffinityTerm.LabelSelector.MatchExpressions {
		melsKeys = append(melsKeys, lsr.Key)
	}

	r.Contains(melsKeys, v1beta1.KubernetesInstanceLabelKey)
	r.Contains(melsKeys, v1beta1.SiteLabelKey)
}
