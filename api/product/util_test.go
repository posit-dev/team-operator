package product_test

import (
	"testing"

	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestConcatLists(t *testing.T) {
	res := product.ConcatLists([]string{"one", "two"}, []string{"three", "four"}, []string{}, []string{"five"})
	require.Equal(t, []string{"one", "two", "three", "four", "five"}, res)

	resInt := product.ConcatLists([]int{1, 2, 3}, []int{4, 5}, []int{}, []int{6})
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, resInt)
}

func TestStringMapToEnvVars(t *testing.T) {
	r := require.New(t)

	r.Equal([]corev1.EnvVar{}, product.StringMapToEnvVars(nil))
	r.Equal([]corev1.EnvVar{}, product.StringMapToEnvVars(map[string]string{}))

	r.Equal([]corev1.EnvVar{
		{Name: "MODE", Value: "chinchilla"},
		{Name: "POWER_LEVEL", Value: "50"},
	},
		product.StringMapToEnvVars(
			map[string]string{
				"POWER_LEVEL": "50",
				"MODE":        "chinchilla",
			},
		),
	)
}

func TestComputeSha256(t *testing.T) {
	// test that the SHA256 computation works as expected
	input := map[string]string{"test": "some-value"}
	expected := "5c6ca8e906c214c32106706ab498650b91c0255b9115c0a3a1c3ca28c4be3a91"
	output, err := product.ComputeSha256(input)
	assert.Nil(t, err)
	assert.Equal(t, expected, output)

}

func TestComputeSha256_Ordering(t *testing.T) {
	// test that ordering does not matter and is consistent
	input := map[string]string{"one": "some-value", "two": "another-value"}
	expected := "8aee779a743ed18fb9dfd2f2cf5c2309d7b69516c2c3dbafe07aa097c0d22d79"
	output, err := product.ComputeSha256(input)
	assert.Nil(t, err)
	assert.Equal(t, expected, output)
	inputAlt := map[string]string{"two": "another-value", "one": "some-value"}
	outputAlt, err := product.ComputeSha256(inputAlt)
	assert.Nil(t, err)
	assert.Equal(t, expected, outputAlt)

	// and mix it up should be different!
	inputBad := map[string]string{"two": "some-value", "one": "another-value"}
	outputBad, err := product.ComputeSha256(inputBad)
	assert.Nil(t, err)
	assert.NotEqual(t, expected, outputBad)
}

func TestLabelMerge(t *testing.T) {
	// test that the merge works as expected
	m1 := map[string]string{"frumious": "bandersnatch"}
	m2 := map[string]string{"vorpal": "sword"}

	result := product.LabelMerge(m1, m2)
	expected := map[string]string{"frumious": "bandersnatch", "vorpal": "sword"}
	assert.Equal(t, expected, result)

	// test that the merge works with nil
	m1 = nil
	result = product.LabelMerge(m1, m2)
	expected = map[string]string{"vorpal": "sword"}
	assert.Equal(t, expected, result)
}
