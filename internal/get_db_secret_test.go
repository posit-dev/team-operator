package internal

import (
	"context"
	"testing"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestDetermineMainDatabaseUrl(t *testing.T) {
	ctx := context.Background()
	r := &product.FakeReconciler{}
	req := ctrl.Request{}

	// TODO: to test this more thoroughly, our "test" site secret will need to be able to provide other values...
	mainUrlSecret := v1beta1.SecretConfig{
		VaultName: "test-url",
		Type:      product.SiteSecretTest,
	}
	dbCredSecret := v1beta1.SecretConfig{
		VaultName: "test-db-cred-secret",
		Type:      product.SiteSecretTest,
	}

	dbUrl, err := DetermineMainDatabaseUrl(ctx, r, req, mainUrlSecret, dbCredSecret)

	assert.Nil(t, err)
	assert.Equal(t, "main-database-url", dbUrl.Path)
	assert.Equal(t, "username", dbUrl.User.Username())
	pass, ok := dbUrl.User.Password()
	assert.True(t, ok)
	assert.Equal(t, "password", pass)
}
