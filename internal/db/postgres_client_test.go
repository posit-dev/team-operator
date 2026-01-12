package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePostgresLabel(t *testing.T) {
	for label, isValid := range map[string]bool{
		"a____________":  true,
		"rad":            true,
		"cool_dogs_1312": true,
		"a":              false, // too short
		"_no":            false, // starts with [^a-z]
		"0lol":           false, // starts with [^a-z]
		"A3a2":           false, // contains "A"
		"a-zz":           false, // contains "-"
		"b5db0d8c8173b71602fcf5ba88476e531cf3e10613db47ab6ab8d3ee9436e081f": false, // too long
		"user_defined_type_catalog":                                         false, // reserved word
	} {
		if isValid {
			assert.Nilf(t, ValidatePostgresLabel(label), "label %q is not valid", label)
		} else {
			assert.NotNilf(t, ValidatePostgresLabel(label), "label %q is valid", label)
		}
	}
}
