package main

import (
	"flag"
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

func TestManageCRDsFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var manageCRDs bool
	registerManageCRDsFlag(fs, &manageCRDs)

	// Default is true: CRD management is enabled out of the box.
	require.Equal(t, "true", fs.Lookup("manage-crds").DefValue)
	require.NoError(t, fs.Parse([]string{}))
	require.True(t, manageCRDs)

	// --manage-crds=false opts out of CRD management (e.g. for GitOps environments).
	require.NoError(t, fs.Parse([]string{"--manage-crds=false"}))
	require.False(t, manageCRDs)
}
