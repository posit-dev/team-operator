package internal_test

import (
	"testing"

	"github.com/posit-dev/team-operator/internal"
	"github.com/stretchr/testify/require"
)

func TestImageSpec(t *testing.T) {
	r := require.New(t)

	r.Equal(
		"ok.bud:7000",
		internal.ImageSpec("ok.bud", "7000"),
	)
	r.Equal(
		"real.fancy@sha1:9308fe8002c32526bd28431bd44e96430afebb65",
		internal.ImageSpec("real.fancy", "sha1:9308fe8002c32526bd28431bd44e96430afebb65"),
	)
}
