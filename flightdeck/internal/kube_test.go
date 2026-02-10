package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsCRDNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		isCRDAbsent bool
	}{
		{
			name: "NotFound error indicates missing CRD",
			err: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Code:   404,
					Reason: metav1.StatusReasonNotFound,
				},
			},
			isCRDAbsent: true,
		},
		{
			name:        "nil error is not a missing CRD",
			err:         nil,
			isCRDAbsent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCRDNotFoundError(tt.err)
			assert.Equal(t, tt.isCRDAbsent, result)
		})
	}
}
