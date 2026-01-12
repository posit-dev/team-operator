package product

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
)

func ConcatLists[T any](slices ...[]T) []T {
	out := []T{}
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}

func MakePullSecrets(secrets []string) []corev1.LocalObjectReference {
	var pullSecrets []corev1.LocalObjectReference
	for _, s := range secrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: s})
	}
	return pullSecrets
}

func StringMapToEnvVars(sm map[string]string) []corev1.EnvVar {
	out := []corev1.EnvVar{}
	keys := maps.Keys(sm)

	sort.Strings(keys)

	for _, k := range keys {
		value := sm[k]

		out = append(out, corev1.EnvVar{
			Name:  k,
			Value: value,
		})
	}

	return out
}

func PassDefaultReplicas(replicas int, def int) int {
	if replicas == 0 {
		return def
	}
	return replicas
}

// DetermineMinAvailableReplicas returns 1 if "replicas > 1" otherwise 0. This is
// intentionally naive for the present, and we can decide better logic later...
func DetermineMinAvailableReplicas(replicas int) int {
	if replicas > 1 {
		return 1
	}
	return 0
}

func LabelMerge(m1 map[string]string, m2 map[string]string) map[string]string {
	if m1 == nil {
		m1 = map[string]string{}
	}
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func ComputeSha256(in map[string]string) (string, error) {
	h := sha256.New()

	keys := []string{}
	for key, _ := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	stringAgg := ""
	for _, key := range keys {
		stringAgg += key + ":" + in[key] + "//"
	}
	if _, err := h.Write([]byte(stringAgg)); err != nil {
		return "", err
	} else {
		// convert h.Sum to a string
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	}
}
