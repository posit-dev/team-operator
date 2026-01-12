package v2alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Resource(t *testing.T) {
	grp := Resource("test")

	assert.Equal(t, "test", grp.Resource)
	assert.Equal(t, "k8s.keycloak.org", grp.Group)
}
