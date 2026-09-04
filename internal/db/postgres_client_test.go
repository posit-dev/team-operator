package db

import (
	"testing"

	"github.com/pkg/errors"
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

func TestErrRowScanReturnsTheStoredError(t *testing.T) {
	boom := errors.New("could not connect")
	row := &errRow{Error: boom}

	var dest string
	assert.Equal(t, boom, row.Scan(&dest), "errRow should surface the connection error on Scan")
	assert.Equal(t, "", dest, "errRow should not write to the scan destination")
}
