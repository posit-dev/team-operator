package db

import (
	"context"
	"net/url"
	"regexp"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
	postgresqlreservedwords "github.com/rstudio/postgresql-reserved-words"
)

var errInvalidPostgresLabel = errors.New("invalid postgres label")

type PostgresClient interface {
	Exec(context.Context, string, ...interface{}) error
	Ping(context.Context) error
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type errRow struct {
	Error error
}

func (er *errRow) Scan(_ ...interface{}) error {
	return er.Error
}

func NewPostgresClient(dbURL *url.URL, log logr.Logger) PostgresClient {
	return &defaultPostgresClient{
		log:   log,
		dbURL: dbURL,
	}
}

type defaultPostgresClient struct {
	log   logr.Logger
	dbURL *url.URL
}

func (dpc *defaultPostgresClient) Ping(ctx context.Context) error {
	conn, err := dpc.getConn(ctx)
	if err != nil {
		return err
	}

	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

func (dpc *defaultPostgresClient) Exec(ctx context.Context, sql string, arguments ...interface{}) error {
	conn, err := dpc.getConn(ctx)
	if err != nil {
		return err
	}

	defer conn.Close(ctx)

	arguments = append([]interface{}{pgx.QuerySimpleProtocol(true)}, arguments...)
	_, err = conn.Exec(ctx, sql, arguments...)

	return err
}

func (dpc *defaultPostgresClient) QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row {
	conn, err := dpc.getConn(ctx)
	if err != nil {
		return &errRow{Error: err}
	}

	defer conn.Close(ctx)

	return conn.QueryRow(ctx, sql, arguments...)
}

func (dpc *defaultPostgresClient) getConn(ctx context.Context) (*pgx.Conn, error) {
	pgCfg, err := pgx.ParseConfig(dpc.dbURL.String())
	if err != nil {
		return nil, err
	}

	pgCfg.Logger = newPgxLogr(dpc.log)
	pgCfg.LogLevel = pgx.LogLevelDebug

	return pgx.ConnectConfig(ctx, pgCfg)
}

// NOTE: this is likely more restrictive than what postgres
// will actually allow :shrug:
var postgresLabelRE = regexp.MustCompile("^[a-z][a-z0-9_]{2,62}$")

func ValidatePostgresLabel(label string) error {
	if postgresqlreservedwords.IsReserved(label) {
		return errors.Wrapf(errInvalidPostgresLabel, "%q is a reserved word", label)
	}

	if !postgresLabelRE.MatchString(label) {
		return errors.Wrap(errInvalidPostgresLabel, label)
	}

	return nil
}
