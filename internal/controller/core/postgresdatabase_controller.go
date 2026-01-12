// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package core

import (
	"context"
	json "encoding/json"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	"github.com/jackc/pgconn"
	"github.com/pkg/errors"
	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/api/product"
	"github.com/posit-dev/team-operator/internal"
	"github.com/posit-dev/team-operator/internal/db"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=postgresdatabases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=postgresdatabases/status,verbs=get;update;patch
//+kubebuilder:rbac:namespace=posit-team,groups=core.posit.team,resources=postgresdatabases/finalizers,verbs=update

//+kubebuilder:rbac:namespace=posit-team,groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

const (
	LabelManagedByKey             = "app.kubernetes.io/managed-by"
	LabelManagedByValue           = "team-operator"
	postgresDatabaseBinaryDataKey = "postgresdatabase.json"
)

var (
	errPostgresDatabaseMismatchedDBHost  = errors.New("postgres database mismatched db host")
	errPostgresDatabaseNoMainDatabaseURL = errors.New("postgres database no main database url found")
	errPostgresDatabaseNoSpecCredentials = errors.New("postgres database no spec url credentials found")
)

// PostgresDatabaseReconciler reconciles a PostgresDatabase object
type PostgresDatabaseReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *PostgresDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	pgd := &positcov1beta1.PostgresDatabase{}

	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}

		return r.Log
	}()

	err := r.Get(ctx, req.NamespacedName, pgd)
	if err != nil && apierrors.IsNotFound(err) {
		l.Info("PostgresDatabase not found; it must have been deleted!")
		return ctrl.Result{}, nil
	} else if err != nil {
		// unexpected error
		return ctrl.Result{}, err
	}

	if pgd.ObjectMeta.DeletionTimestamp != nil {
		// deletion has been requested
		l.Info("PostgresDatabase found; deleting database")
		return r.cleanupDatabase(ctx, req, pgd)
	}

	l.Info("PostgresDatabase found; reconciling database")

	return r.createDatabase(ctx, req, pgd)
}

func (r *PostgresDatabaseReconciler) cleanupDatabase(ctx context.Context, req ctrl.Request, pg *positcov1beta1.PostgresDatabase) (ctrl.Result, error) {

	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}

		return r.Log
	}().WithValues(
		"postgresdatabase", req.NamespacedName,
		"event", "cleanup",
	)

	var dbName, roleName string
	if specDbUrl, err := url.Parse(pg.Spec.URL); err != nil {
		l.Error(err, "Error parsing specDbUrl")
		return ctrl.Result{}, err
	} else {
		dbName = strings.TrimPrefix(specDbUrl.Path, "/")
		roleName = specDbUrl.User.Username()
	}

	if pg.Spec.Teardown.Drop {
		mainDbUrl, err := internal.DetermineMainDatabaseUrl(ctx, r, req, pg.Spec.WorkloadSecret, pg.Spec.MainDatabaseCredentialSecret)
		if mainDbUrl == nil || err != nil {
			l.Error(errPostgresDatabaseNoMainDatabaseURL, "failed to get the main database url")
			return ctrl.Result{}, errPostgresDatabaseNoMainDatabaseURL
		}

		mainDbClient := db.NewPostgresClient(mainDbUrl, l)
		scanString := ""

		if err := mainDbClient.QueryRow(ctx, "SELECT datname FROM pg_database WHERE datname = $1", dbName).Scan(&scanString); err == nil {
			l.Info("dropping database", "db_name", dbName)

			if err := mainDbClient.Exec(ctx, "DROP DATABASE "+dbName); err != nil {
				l.Error(err, "failed to drop database", "db_name", dbName)
				return ctrl.Result{}, err
			}
		}

		if err := mainDbClient.QueryRow(ctx, "SELECT rolname FROM pg_roles WHERE rolname = $1", roleName).Scan(&scanString); err == nil {
			l.Info("dropping role", "role", roleName)

			if err := mainDbClient.Exec(ctx, "DROP ROLE "+roleName); err != nil {
				l.Error(err, "failed to drop role", "role", roleName)
				return ctrl.Result{}, err
			}
		}
	}

	l.Info("successfully cleaned up database", "db_name", roleName)

	// ensure finalizers no longer exist
	if len(pg.ObjectMeta.Finalizers) > 0 {
		finalizerPatch := []patchArrayStringValue{
			{"remove", "/metadata/finalizers", nil},
		}
		if jFinalizerPatch, err := json.Marshal(finalizerPatch); err != nil {
			l.Error(err, "Error marshaling finalizer remove patch into json")
			return ctrl.Result{}, err
		} else {
			tmpPatch := client.RawPatch(types.JSONPatchType, jFinalizerPatch)
			if err := r.Patch(ctx, pg, tmpPatch); err != nil {
				l.Error(err, "Error patching to remove finalizer")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *PostgresDatabaseReconciler) GetLogger(ctx context.Context) logr.Logger {
	if v, err := logr.FromContext(ctx); err == nil {
		return v
	}
	return r.Log
}

func quote(s string) string {
	return "\"" + s + "\""
}

type patchArrayStringValue struct {
	Op    string   `json:"op"`
	Path  string   `json:"path"`
	Value []string `json:"value,omitempty"`
}

func (r *PostgresDatabaseReconciler) createDatabase(ctx context.Context, req ctrl.Request, pgd *positcov1beta1.PostgresDatabase) (ctrl.Result, error) {

	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}
		return r.Log
	}().WithValues(
		"postgresdatabase", req.NamespacedName,
		"event", "create/update",
	)

	// ensure finalizers exist (they protect against the resource deleting before database cleanup is complete)
	if len(pgd.ObjectMeta.Finalizers) == 0 {
		finalizerPatch := []patchArrayStringValue{
			{"add", "/metadata/finalizers", []string{"posit-team.posit.co"}},
		}
		if jFinalizerPatch, err := json.Marshal(finalizerPatch); err != nil {
			l.Error(err, "Error marshaling finalizer add patch into json")
			return ctrl.Result{}, err
		} else {
			tmpPatch := client.RawPatch(types.JSONPatchType, jFinalizerPatch)
			if err := r.Patch(ctx, pgd, tmpPatch); err != nil {
				l.Error(err, "Error patching to add finalizer")
				return ctrl.Result{}, err
			}
		}
	}

	l.Info("loading validated database urls")

	mainDbUrl, specDbUrl, err := r.loadValidatedDatabaseURLs(ctx, pgd, req, pgd.Spec.Secret, pgd.Spec.SecretPasswordKey)
	if err != nil {
		l.Error(err, "failed to load validated database urls")
		return ctrl.Result{}, err
	}

	superuserDbUrl, _ := url.Parse(specDbUrl.String())
	mainDbPassword, hasPassword := mainDbUrl.User.Password()

	if !hasPassword {
		l.Error(errPostgresDatabaseNoSpecCredentials, "in main db url")
		return ctrl.Result{}, errPostgresDatabaseNoSpecCredentials
	}

	superuserDbUrl.User = url.UserPassword(mainDbUrl.User.Username(), mainDbPassword)

	// init clients
	mainDbClient := db.NewPostgresClient(mainDbUrl, l)
	specDbClient := db.NewPostgresClient(specDbUrl, l)
	superuserDbClient := db.NewPostgresClient(superuserDbUrl, l)

	needsDatabase := false
	needsRole := false

	l.Info("determining database and role creation needs")

	if err := specDbClient.Ping(ctx); err == nil {
		l.Info("database exists with specified credentials")
	} else if err := superuserDbClient.Ping(ctx); err == nil {
		l.Info("database exists with outdated credentials")

		needsRole = true
	} else {
		needsRole = true
		needsDatabase = true
	}

	if needsRole {
		if err := r.ensureCredentialsMatch(ctx, mainDbClient, specDbUrl); err != nil {
			{
				l.Error(err, "failed to update credentials")
				return ctrl.Result{}, err
			}
		}
	}

	if needsDatabase {
		if err := r.ensureDatabaseExistsWithAccess(ctx, mainDbClient, specDbUrl); err != nil {
			l.Error(err, "failed to ensure database exists with access")
			return ctrl.Result{}, err
		}
	}

	// grant product role to admin user, required in Azure to allow admin user to update the product role's owned schemas below
	if err := superuserDbClient.Exec(ctx, "GRANT "+quote(specDbUrl.User.Username())+" TO "+mainDbUrl.User.Username()); err != nil {
		l.Error(err, "error granting product role to admin user")
		return ctrl.Result{}, err
	}

	// ensure permissions on public schema
	if err := superuserDbClient.Exec(ctx, "ALTER SCHEMA public OWNER TO "+quote(specDbUrl.User.Username())); err != nil {
		l.Error(err, "error setting permissions on public schema")
		return ctrl.Result{}, err
	}

	// now check if any schemas are needed
	for _, schema := range pgd.Spec.Schemas {
		l.Info("ensuring schema access", "schema", schema)
		if err := superuserDbClient.Exec(ctx, "ALTER SCHEMA "+quote(schema)+" OWNER TO "+quote(specDbUrl.User.Username())); err != nil {
			l.Error(err, "error with alter schema...")
			var pqerr *pgconn.PgError
			if errors.As(err, &pqerr) {
				if pqerr.Code == "3F000" {
					l.Info("schema does not exist; creating")
					if err := specDbClient.Exec(ctx, "CREATE SCHEMA "+quote(schema)+" AUTHORIZATION "+quote(specDbUrl.User.Username())); err != nil {
						l.Error(err, "error creating schema", "schema", schema)
						return ctrl.Result{}, err
					}
				} else {
					l.Error(err, "unknown error altering schema", "schema", schema, "error-code", pqerr.Code)
					return ctrl.Result{}, err
				}
			} else {
				l.Error(err, "unknown error altering schema", "schema", schema)
				return ctrl.Result{}, err
			}
		}
	}

	for _, extension := range pgd.Spec.Extensions {
		l.Info("ensuring extension exists", "extension", extension)

		if err := superuserDbClient.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \""+extension+"\""); err != nil {
			l.Error(err, "failed to ensure extension exists", "extension", extension)
			return ctrl.Result{}, err
		}
	}

	if err := specDbClient.Ping(ctx); err != nil {
		l.Error(err, "despite everything we have tried")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PostgresDatabaseReconciler) ensureCredentialsMatch(ctx context.Context, mainDbClient db.PostgresClient, specDbUrl *url.URL) error {

	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err != nil {
			return v
		}

		return r.Log
	}()
	roleName := specDbUrl.User.Username()
	if err := db.ValidatePostgresLabel(roleName); err != nil {
		return err
	}

	rolePassword, hasPassword := specDbUrl.User.Password()
	if !hasPassword {
		return errPostgresDatabaseNoSpecCredentials
	}

	existingRole := ""
	if err := mainDbClient.QueryRow(ctx, "SELECT rolname FROM pg_roles where rolname = $1", roleName).Scan(&existingRole); err != nil {
		l.Info("role not found; creating")

		if err := mainDbClient.Exec(ctx, "CREATE ROLE "+roleName+" LOGIN PASSWORD $1", rolePassword); err != nil {
			return err
		}
	} else {
		l.Info("role found; updating password")
		if err := mainDbClient.Exec(ctx, "ALTER ROLE "+roleName+" LOGIN PASSWORD $1", rolePassword); err != nil {
			return err
		}
	}

	return nil
}
func (r *PostgresDatabaseReconciler) ensureDatabaseExistsWithAccess(ctx context.Context, mainDbClient db.PostgresClient, specDbUrl *url.URL) error {
	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}

		return r.Log
	}()

	roleName := specDbUrl.User.Username()
	if err := db.ValidatePostgresLabel(roleName); err != nil {
		return err
	}

	dbName := strings.TrimPrefix(specDbUrl.Path, "/")
	if err := db.ValidatePostgresLabel(dbName); err != nil {
		return err
	}

	existingDb := ""

	if err := mainDbClient.QueryRow(ctx, "SELECT datname FROM pg_database WHERE datname = $1", dbName).Scan(&existingDb); err != nil {
		l.Info("database not found; creating")

		if err := mainDbClient.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
			return err
		}
	}
	l.Info("granting database privileges")

	if err := mainDbClient.Exec(ctx, "GRANT ALL PRIVILEGES ON DATABASE "+dbName+" TO "+roleName); err != nil {
		return err
	}

	return nil
}

// loadValidatedDatabaseURLs will parse the mainDB and specDB URLs and return them. It grabs information from (1) the PostgresDatabase
// object, (2) the configured mainDatabaseSecret, (3) the configured mainDatabaseUrl, and (4) an optional secret vault
func (r *PostgresDatabaseReconciler) loadValidatedDatabaseURLs(ctx context.Context, pgd *positcov1beta1.PostgresDatabase, req ctrl.Request, secret positcov1beta1.SecretConfig, key string) (*url.URL, *url.URL, error) {
	l := func() logr.Logger {
		if v, err := logr.FromContext(ctx); err == nil {
			return v
		}

		return r.Log
	}()
	specDbUrl, err := url.Parse(pgd.Spec.URL)
	if err != nil {
		return nil, nil, err
	}
	mainDBURL, err := internal.DetermineMainDatabaseUrl(ctx, r, req, pgd.Spec.WorkloadSecret, pgd.Spec.MainDatabaseCredentialSecret)
	if err != nil {
		l.Error(err, "error determining database url")
		return nil, nil, errPostgresDatabaseNoMainDatabaseURL
	}

	// copy query params to the spec db url (if not present)
	mainDbQueryParams := mainDBURL.Query()
	specDbQueryParams := specDbUrl.Query()
	for k, _ := range mainDbQueryParams {
		// only overwrite if specDbUrl keys do not exist
		if specDbQueryParams.Get(k) == "" {
			v := mainDbQueryParams.Get(k)
			specDbQueryParams.Set(k, v)
		}
	}

	specDbUrl.RawQuery = specDbQueryParams.Encode()

	if specDbUrl.Host != mainDBURL.Host {
		l.Info("mismatched db host", "main-db", mainDBURL.Host, "spec-db", specDbUrl.Host)
		return nil, nil, errPostgresDatabaseMismatchedDBHost
	}
	if specDbUrl.User == nil || specDbUrl.User.Username() == "" {
		return nil, nil, errPostgresDatabaseNoSpecCredentials
	}

	// if vault and key are specified, try to retrieve the spec db password as a secret
	// if this fails, we exit. We do not try to recover or check whether a password
	// is already defined on the specDbUrl. vault and key are configured, so we must use them
	if secret.VaultName != "" && key != "" {
		if secretPassword, err := product.FetchSecret(ctx, r, req, secret.Type, secret.VaultName, key); err != nil {
			return nil, nil, err
		} else {
			// SUCCESS!! Got the password
			// overwrite the specDb User definition
			specDbUrl.User = url.UserPassword(
				specDbUrl.User.Username(),
				secretPassword,
			)
		}
	}

	if _, hasPassword := specDbUrl.User.Password(); !hasPassword {
		return nil, nil, errPostgresDatabaseNoSpecCredentials
	}

	return mainDBURL, specDbUrl, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&positcov1beta1.PostgresDatabase{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
