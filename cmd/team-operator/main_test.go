package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/code-generator/cmd/conversion-gen/generators"
	"k8s.io/gengo/v2"
)

func RandomRunNameSystem() string {
	return generators.DefaultNameSystem()
}

func TestThings(t *testing.T) {
	// this should probably include a comment...
	require.Contains(t, gengo.StdGeneratedBy, "//")
}
