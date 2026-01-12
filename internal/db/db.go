package db

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/pkg/errors"
	"github.com/rstudio/goex/crypto/randex"
	"github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var invalidCharacters = regexp.MustCompile("[^a-z0-9]") // do not glob, lest we lose uniqueness
func DbKey(req ctrl.Request, name string) client.ObjectKey {
	return client.ObjectKey{
		Name:      name,
		Namespace: req.NamespacedName.Namespace,
	}
}

func QueryParams(schema, sslmode string) string {
	q := ""

	if schema != "" {
		q += fmt.Sprintf("options=-csearch_path=%s", schema)
	}
	if sslmode != "" {
		if q != "" {
			q += "&"
		}
		q += fmt.Sprintf("sslmode=%s", sslmode)
	}
	return q
}

func DatabaseUrl(host, name, password, query string) *url.URL {
	dbName := invalidCharacters.ReplaceAllString(name, "_")
	var user *url.Userinfo

	// allow setting just a user URL if password is empty
	if password == "" {
		user = url.User(dbName)
	} else {
		user = url.UserPassword(dbName, password)
	}

	return &url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     host,
		Path:     dbName,
		RawQuery: query,
	}
}

// EnsureDatabaseExists creates a PostgresDatabase object if it does not exist, and reconciles (some) attributes
// (just URL for now) if it already exists
func EnsureDatabaseExists(
	ctx context.Context, r product.SomeReconciler, req ctrl.Request,
	owner product.OwnerProvider, dbConfig v1beta1.PostgresDatabaseConfig, name, password string,
	schemas []string,
	secret, workloadSecret, mainDatabaseCredentialSecret v1beta1.SecretConfig,
	secretKey string,
) error {

	l := r.GetLogger(ctx).WithValues(
		"event", "create-database",
		"database", name,
	)

	u := DatabaseUrl(dbConfig.Host, name, password, "")

	fmt.Printf("Database URL: %s\n", u.String())
	if u.Host == "" {
		err := errors.New("database connection hostname not provided")
		l.Error(err, "error creating database connection URL")
		return err
	}

	pgd := &v1beta1.PostgresDatabase{
		TypeMeta: v1.TypeMeta{
			Kind:       "PostgresDatabase",
			APIVersion: "core.posit.team/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				v1beta1.ManagedByLabelKey: v1beta1.ManagedByLabelValue,
			},
			OwnerReferences: owner.OwnerReferencesForChildren(),
		},
		Spec: v1beta1.PostgresDatabaseSpec{
			URL:        u.String(),
			Extensions: []string{},
			Teardown: &v1beta1.PostgresDatabaseSpecTeardown{
				Drop: dbConfig.DropOnTeardown,
			},
			Schemas:                      schemas,
			SecretVault:                  secret.VaultName,
			Secret:                       secret,
			WorkloadSecret:               workloadSecret,
			SecretPasswordKey:            secretKey,
			MainDatabaseCredentialSecret: mainDatabaseCredentialSecret,
		},
		Status: v1beta1.PostgresDatabaseStatus{},
	}

	pgdExisting := &v1beta1.PostgresDatabase{}

	err := r.Get(ctx, DbKey(req, name), pgdExisting)
	if err != nil && apierrors.IsNotFound(err) {
		// not found, creating database
		if err := r.Create(ctx, pgd); err != nil {
			// error creating database
			return err
		}
	} else if err != nil {
		// some other error
		return err
	} else {
		// no error... the thing exists
		// TODO: reconcile _all_ differences... how does this
		//   affect the state of the service...?

		pgd.ObjectMeta.ResourceVersion = pgdExisting.ObjectMeta.ResourceVersion

		if pgdExisting.Spec.URL != pgd.Spec.URL {
			l.Info("database already exists, but url is different; updating")
			if err := r.Update(ctx, pgd); err != nil {
				return err
			}
		} else if !slices.Equal(pgdExisting.Spec.Schemas, pgd.Spec.Schemas) {
			l.Info("database already exists, but schemas have changed; updating")
			if err := r.Update(ctx, pgd); err != nil {
				return err
			}

			// we can use length and the first entry as a simple comparison (just one owner)
		} else if product.OwnerReferencesHaveChanged(pgdExisting.OwnerReferences, pgd.OwnerReferences) {
			l.Info("database already exists, but owner has changed; updating")
			if err := r.Update(ctx, pgd); err != nil {
				return err
			}
		} else if pgdExisting.Spec.Secret.VaultName != pgd.Spec.Secret.VaultName ||
			pgdExisting.Spec.Secret.Type != pgd.Spec.Secret.Type ||
			pgdExisting.Spec.SecretVault != pgd.Spec.SecretVault ||
			pgdExisting.Spec.SecretPasswordKey != pgd.Spec.SecretPasswordKey {
			l.Info("database secret definition changed; updating")
			if err := r.Update(ctx, pgd); err != nil {
				return err
			}
		} else if pgdExisting.Spec.MainDatabaseCredentialSecret.VaultName != pgd.Spec.MainDatabaseCredentialSecret.VaultName ||
			pgdExisting.Spec.MainDatabaseCredentialSecret.Type != pgd.Spec.MainDatabaseCredentialSecret.Type {
			l.Info("database mainDbCredentialSecret definition changed; updating")
			if err := r.Update(ctx, pgd); err != nil {
				return err
			}
		} else if pgdExisting.Spec.WorkloadSecret.Type != pgd.Spec.WorkloadSecret.Type ||
			pgdExisting.Spec.WorkloadSecret.VaultName != pgd.Spec.WorkloadSecret.VaultName {
			l.Info("database workloadSecret definition changed; updating")
			if err := r.Update(ctx, pgd); err != nil {
				return err
			}
		} else {
			l.Info("database already exists; not modifying properties")
		}
	}

	return nil
}

func CleanupDatabasePasswordSecret(ctx context.Context, r product.SomeReconciler, req ctrl.Request, name string) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-database-password-secret",
		"database", name,
	)

	// we do not know what type of secret was created, so we just check to see if one exists

	// Kubernetes
	s := &corev1.Secret{}

	if err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: req.Namespace}, s); err != nil && apierrors.IsNotFound(err) {
		// secret is missing, move on and clean up any other secrets
	} else if err != nil {
		l.Error(err, "error trying to get kubernetes secret before deletion")
		return err
	} else {
		// secret found, now we should remove it
		l.Info("deleting kubernetes secret")
		if err := r.Delete(ctx, s); err != nil && apierrors.IsNotFound(err) {
			// success! item is already deleted
		} else if err != nil {
			l.Error(err, "error trying to delete kubernetes secret")
			return err
		}
	}

	// TODO: clean up other secret mechanisms...?

	return nil

}

func generatePasswordSecretData() map[string]string {
	return map[string]string{
		"password": randex.String(25),
	}
}

// EnsureDatabasePasswordSecretAndReturnIt generates a password on the initial run / creation, or retrieves the value
// that exists in the cluster and returns it. There is currently no "update" mechanism.
func EnsureDatabasePasswordSecretAndReturnIt(ctx context.Context, r product.SomeReconciler, req ctrl.Request, owner product.OwnerProvider, secretType product.SiteSecretType, name string) (string, error) {
	l := r.GetLogger(ctx).WithValues(
		"event", "store-database-password-secret",
		"database", name,
	)

	switch secretType {
	case product.SiteSecretKubernetes:
		secretData, err := internal.EnsureSecretKubernetes(
			ctx, r, req, owner, name, generatePasswordSecretData,
		)
		if err != nil {
			l.Error(err, "error creating secret in kubernetes")
			return "", err
		}
		return secretData["password"], nil
	case product.SiteSecretAws:
		// Do nothing... this secret type will be read directly
		return "", nil
	default:
		err := errors.New("invalid site definition for secret type")
		l.Error(err, "invalid secret type", "secretType", secretType)
		return "", err
	}
}

func CleanupDatabase(ctx context.Context, r product.SomeReconciler, req ctrl.Request, name string) error {
	l := r.GetLogger(ctx).WithValues(
		"event", "cleanup-database",
		"database", name,
	)
	pgd := &v1beta1.PostgresDatabase{}
	err := r.Get(ctx, DbKey(req, name), pgd)
	if err != nil && apierrors.IsNotFound(err) {
		// not found, nothing to clean up
		l.Info("database already cleaned up")
		return nil
	} else if err != nil {
		return err
	}

	// clean up database
	l.Info("deleting database")
	// NOTE: whether this _actually_ deletes the database is up to the "teardown" policy
	// on the database resource
	if err := r.Delete(ctx, pgd); err != nil {
		return err
	}
	return nil
}
