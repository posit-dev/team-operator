// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// capturedLog is a single logr call recorded by captureSink.
type capturedLog struct {
	isError bool
	err     error
	msg     string
	values  map[string]interface{}
}

// captureSink records logr calls, accumulating WithValues pairs the way a real
// sink would, so tests can assert on what pgxLogr actually attached.
type captureSink struct {
	values  []interface{}
	entries *[]capturedLog
}

func newCaptureSink() (logr.Logger, *[]capturedLog) {
	entries := &[]capturedLog{}
	return logr.New(&captureSink{entries: entries}), entries
}

func (s *captureSink) Init(logr.RuntimeInfo) {}

func (s *captureSink) Enabled(int) bool { return true }

func (s *captureSink) record(isError bool, err error, msg string, keysAndValues []interface{}) {
	all := append(append([]interface{}{}, s.values...), keysAndValues...)

	values := map[string]interface{}{}
	for i := 0; i+1 < len(all); i += 2 {
		values[fmt.Sprint(all[i])] = all[i+1]
	}

	*s.entries = append(*s.entries, capturedLog{
		isError: isError,
		err:     err,
		msg:     msg,
		values:  values,
	})
}

func (s *captureSink) Info(_ int, msg string, keysAndValues ...interface{}) {
	s.record(false, nil, msg, keysAndValues)
}

func (s *captureSink) Error(err error, msg string, keysAndValues ...interface{}) {
	s.record(true, err, msg, keysAndValues)
}

func (s *captureSink) V(int) logr.LogSink { return s }

func (s *captureSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &captureSink{
		values:  append(append([]interface{}{}, s.values...), keysAndValues...),
		entries: s.entries,
	}
}

func (s *captureSink) WithName(string) logr.LogSink { return s }

func TestPgxLogrRoutesErrorLevelToLogrError(t *testing.T) {
	log, entries := newCaptureSink()
	boom := errors.New("boom")

	newPgxLogr(log).Log(context.Background(), tracelog.LogLevelError, "query failed", map[string]interface{}{
		"err": boom,
		"sql": "SELECT 1",
	})

	assert.Len(t, *entries, 1)
	e := (*entries)[0]
	assert.True(t, e.isError, "error level should route to logr.Error")
	assert.Equal(t, boom, e.err, "err from data should be passed to logr.Error")
	assert.Equal(t, "query failed", e.msg)
	assert.Equal(t, "SELECT 1", e.values["sql"], "data keys should be attached as values")
	assert.NotContains(t, e.values, "pgx_level", "error level should not attach pgx_level")
}

func TestPgxLogrErrorLevelWithoutErrKey(t *testing.T) {
	log, entries := newCaptureSink()

	newPgxLogr(log).Log(context.Background(), tracelog.LogLevelError, "no err key", map[string]interface{}{
		"sql": "SELECT 1",
	})

	assert.Len(t, *entries, 1)
	e := (*entries)[0]
	assert.True(t, e.isError)
	assert.Nil(t, e.err, "a nil error should be logged when data has no err key")
	assert.Equal(t, "no err key", e.msg)
}

func TestPgxLogrRoutesNonErrorLevelsToLogrInfo(t *testing.T) {
	for _, tc := range []struct {
		level    tracelog.LogLevel
		expected string
	}{
		{tracelog.LogLevelTrace, "trace"},
		{tracelog.LogLevelDebug, "debug"},
		{tracelog.LogLevelInfo, "info"},
		{tracelog.LogLevelWarn, "warn"},
	} {
		t.Run(tc.expected, func(t *testing.T) {
			log, entries := newCaptureSink()

			newPgxLogr(log).Log(context.Background(), tc.level, "ran a query", map[string]interface{}{
				"sql": "SELECT 1",
			})

			assert.Len(t, *entries, 1)
			e := (*entries)[0]
			assert.False(t, e.isError, "non-error levels should route to logr.Info")
			assert.Equal(t, "ran a query", e.msg)
			assert.Equal(t, "SELECT 1", e.values["sql"])
			assert.Equal(t, tc.expected, fmt.Sprint(e.values["pgx_level"]),
				"non-error levels should attach the pgx level")
		})
	}
}

func TestPgxLogrHandlesNilData(t *testing.T) {
	log, entries := newCaptureSink()

	assert.NotPanics(t, func() {
		newPgxLogr(log).Log(context.Background(), tracelog.LogLevelDebug, "no data", nil)
	})

	assert.Len(t, *entries, 1)
	assert.Equal(t, "no data", (*entries)[0].msg)
}
