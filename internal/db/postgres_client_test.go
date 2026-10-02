package db

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	dest := "original"
	assert.Equal(t, boom, row.Scan(&dest), "errRow should surface the connection error on Scan")
	assert.Equal(t, "original", dest, "errRow should not write to the scan destination")
}

type fakeRow struct {
	scan func() error
}

func (fr *fakeRow) Scan(_ ...interface{}) error { return fr.scan() }

func TestClosingRowClosesTheConnectionOnlyAfterScan(t *testing.T) {
	for name, scanErr := range map[string]error{
		"scan succeeds": nil,
		"scan fails":    errors.New("scan blew up"),
	} {
		t.Run(name, func(t *testing.T) {
			closed := false
			closedDuringScan := false

			row := &closingRow{
				row: &fakeRow{scan: func() error {
					closedDuringScan = closed
					return scanErr
				}},
				close: func() { closed = true },
			}

			assert.Equal(t, scanErr, row.Scan(), "closingRow should surface the underlying Scan result")
			assert.False(t, closedDuringScan, "the connection must still be open while the row is scanned")
			assert.True(t, closed, "the connection must be closed once Scan returns")
		})
	}
}

// TestQueryRowScanAgainstLivePostgres is the regression test for the pgx v4 ->
// v5 migration. QueryRow used to close its connection before returning the lazy
// row reader, so by the time the caller called Scan the connection was gone. In
// pgx v5 that surfaces as "conn closed" from Scan whether or not the row exists,
// which breaks every existence check in the PostgresDatabase controller.
//
// This needs a real postgres; set TEST_POSTGRES_URL to run it, e.g.
//
//	docker run --rm -d -p 55432:5432 -e POSTGRES_PASSWORD=testpw postgres:16-alpine
//	TEST_POSTGRES_URL='postgres://postgres:testpw@127.0.0.1:55432/postgres?sslmode=disable' go test ./internal/db/...
func TestQueryRowScanAgainstLivePostgres(t *testing.T) {
	raw := os.Getenv("TEST_POSTGRES_URL")
	if raw == "" {
		t.Skip("TEST_POSTGRES_URL is not set; skipping live postgres test")
	}

	dbURL, err := url.Parse(raw)
	require.NoError(t, err, "TEST_POSTGRES_URL must be a valid URL")

	ctx := context.Background()
	client := NewPostgresClient(dbURL, logr.Discard())

	t.Run("an existing row scans without error", func(t *testing.T) {
		dest := "original"
		err := client.QueryRow(ctx,
			"SELECT datname FROM pg_database WHERE datname = $1", "postgres").Scan(&dest)

		assert.NoError(t, err, "scanning an existing row must not error")
		assert.Equal(t, "postgres", dest, "the scan destination should hold the row value")
	})

	t.Run("a missing row reports ErrNoRows", func(t *testing.T) {
		dest := "original"
		err := client.QueryRow(ctx,
			"SELECT datname FROM pg_database WHERE datname = $1", "no_such_database_here").Scan(&dest)

		assert.ErrorIs(t, err, pgx.ErrNoRows, "scanning a missing row must report ErrNoRows")
		assert.Equal(t, "original", dest, "a missing row should not write to the scan destination")
	})
}
