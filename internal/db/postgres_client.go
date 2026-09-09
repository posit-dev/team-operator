package db

import (
	"context"
	"net/url"
	"regexp"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/tracelog"
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

// closingRow keeps the connection that produced a row alive until the caller
// has scanned it. pgx returns a lazy row reader from QueryRow, and as of pgx v5
// scanning it also drains and closes the underlying result stream, so closing
// the connection first makes every Scan fail with "conn closed" -- regardless
// of whether the row actually exists.
type closingRow struct {
	row   pgx.Row
	close func()
}

func (cr *closingRow) Scan(dest ...interface{}) error {
	defer cr.close()

	return cr.row.Scan(dest...)
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

	arguments = append([]interface{}{pgx.QueryExecModeSimpleProtocol}, arguments...)
	_, err = conn.Exec(ctx, sql, arguments...)

	return err
}

func (dpc *defaultPostgresClient) QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row {
	conn, err := dpc.getConn(ctx)
	if err != nil {
		return &errRow{Error: err}
	}

	return &closingRow{
		row:   conn.QueryRow(ctx, sql, arguments...),
		close: func() { _ = conn.Close(ctx) },
	}
}

func (dpc *defaultPostgresClient) getConn(ctx context.Context) (*pgx.Conn, error) {
	pgCfg, err := pgx.ParseConfig(dpc.dbURL.String())
	if err != nil {
		return nil, err
	}

	pgCfg.Tracer = &tracelog.TraceLog{
		Logger:   newPgxLogr(dpc.log),
		LogLevel: tracelog.LogLevelDebug,
	}

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
