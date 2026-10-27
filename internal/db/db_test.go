// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePasswordSecretData(t *testing.T) {
	first := generatePasswordSecretData()
	second := generatePasswordSecretData()

	require.Len(t, first, 1)
	password := first["password"]
	assert.Len(t, password, 26)
	assert.Regexp(t, `^[A-Z2-7]+$`, password)
	assert.NotEqual(t, password, second["password"])
}
