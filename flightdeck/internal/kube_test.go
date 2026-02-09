package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCRDNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		errMsg      string
		isCRDAbsent bool
	}{
		{
			name:        "resource not found indicates missing CRD",
			errMsg:      "the server could not find the requested resource",
			isCRDAbsent: true,
		},
		{
			name:        "no matches for kind indicates missing CRD",
			errMsg:      "no matches for kind \"Site\" in version \"core.posit.team/v1beta1\"",
			isCRDAbsent: true,
		},
		{
			name:        "connection refused is not a missing CRD",
			errMsg:      "connection refused",
			isCRDAbsent: false,
		},
		{
			name:        "timeout is not a missing CRD",
			errMsg:      "context deadline exceeded",
			isCRDAbsent: false,
		},
		{
			name:        "empty string is not a missing CRD",
			errMsg:      "",
			isCRDAbsent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCRDNotFoundError(tt.errMsg)
			assert.Equal(t, tt.isCRDAbsent, result)
		})
	}
}
