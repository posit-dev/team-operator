package internal

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	ctrl "sigs.k8s.io/controller-runtime"
)

func DetermineMainDatabaseUrl(ctx context.Context, r product.SomeReconciler, req ctrl.Request, mainUrlSecret, dbCredSecret v1beta1.SecretConfig) (*url.URL, error) {

	l := r.GetLogger(ctx)

	dbUrl := &url.URL{}

	// this is the key within the secret that we will target...
	secretKey := "main-database-url"

	switch mainUrlSecret.Type {

	case product.SiteSecretAws, product.SiteSecretKubernetes, product.SiteSecretTest:
		if secretEntry, err := product.FetchSecret(ctx, r, req, mainUrlSecret.Type, mainUrlSecret.VaultName, secretKey); err != nil {
			l.Error(err, "error reading site secret")
			return nil, err
		} else {
			dbUrl, err = url.Parse(secretEntry)
			if err != nil {
				l.Error(err, "error parsing url data from secret")
				return &url.URL{}, err
			}
		}

	default:
		err := errors.New("missing database connection definition")

		// no database connection available
		l.Error(err, "should be defined in specification")
		return &url.URL{}, err
	}

	// get credentials. this overwrites any credentials on the url itself
	// NOTE: hardcoded keys "username" and "password" in this secret
	//   This is especially useful for AWS-managed RDS credentials, but can decouple
	//   secret management (url vs. username/password) in k8s as well
	if dbCredSecret.Type != product.SiteSecretNone {
		sl := l.WithValues("vault_name", dbCredSecret.VaultName)

		var user string
		var pass string
		if tmpUser, err := product.FetchSecret(ctx, r, req, dbCredSecret.Type, dbCredSecret.VaultName, "username"); err != nil {
			sl.Error(
				err, "error fetching secret for main database username",
				"secretType", dbCredSecret.Type,
				"vaultName", dbCredSecret.VaultName,
				"key", "username",
			)

			// try to fetch the user off of the dbCredSecret as the default
			if dbUrl.User != nil {
				user = dbUrl.User.String()
			}
		} else {
			user = tmpUser
		}
		if tmpPass, err := product.FetchSecret(ctx, r, req, dbCredSecret.Type, dbCredSecret.VaultName, "password"); err != nil {
			sl.Error(
				err, "error fetching secret for main database password",
				"secretType", dbCredSecret.Type,
				"vaultName", dbCredSecret.VaultName,
				"key", "password",
			)
		} else {
			pass = tmpPass
		}
		if user == "" || pass == "" {
			sl.Error(errors.New("invalid credentials "), "error ")
		}
		if user != "" || pass != "" {
			if user == "" {
				sl.Error(
					errors.New("no username defined by dbCredentialSecret"),
					"used dbCredentialSecret for password but got no username on Url or in secret",
					"secretType", dbCredSecret.Type,
					"vaultName", dbCredSecret.VaultName,
				)
			} else {
				sl.Info("using username/password retrieved from dbCredentialSecret")

				// clean trailing whitespace on user / password...
				//   for instance, if you include a newline in your base64 encoded password 😑
				user = strings.TrimSpace(user)
				pass = strings.TrimSpace(pass)

				dbUrl.User = url.UserPassword(user, pass)
			}
		}
	}

	l.Info(
		"defining site with main database connection",
		"db-url", dbUrl.Redacted(),
	)

	return dbUrl, nil
}
