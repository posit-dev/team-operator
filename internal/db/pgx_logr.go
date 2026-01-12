package db

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v4"
)

func newPgxLogr(l logr.Logger) *pgxLogr {
	return &pgxLogr{l: l}
}

type pgxLogr struct {
	l logr.Logger
}

func (pl *pgxLogr) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	log := pl.l
	values := []interface{}{}

	for k, v := range data {
		values = append(values, k, v)
	}

	log = log.WithValues(values...)

	switch level {
	case pgx.LogLevelError:
		var err error
		if dataErr, ok := data["err"]; ok {
			err = dataErr.(error)
		}

		log.Error(err, msg)
	default:
		log.WithValues("pgx_level", level).Info(msg)
	}
}
