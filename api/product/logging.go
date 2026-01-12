package product

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
)

type LogFormat string

const (
	// LogFormatText is empty so that it is the default...
	LogFormatText LogFormat = ""
	LogFormatJson           = "JSON"
)

// LoggerFromContext retrieves a logger from the context or initializes a new (simple) one
func LoggerFromContext(ctx context.Context) logr.Logger {
	if tmpLog, err := logr.FromContext(ctx); err != nil {
		return NewNoOpLogger()
	} else {
		return tmpLog
	}
}

func NewNoOpLogger() logr.Logger {
	return logr.New(&noOpLogger{})
}

type noOpLogger struct {
}

func (l *noOpLogger) Init(info logr.RuntimeInfo) {}

func (l *noOpLogger) Enabled(level int) bool { return true }

func (l *noOpLogger) Info(level int, msg string, keysAndValues ...interface{}) {}

func (l *noOpLogger) Error(err error, msg string, keysAndValues ...interface{}) {}

func (l *noOpLogger) V(level int) logr.LogSink                             { return l }
func (l *noOpLogger) WithValues(keysAndValues ...interface{}) logr.LogSink { return l }
func (l *noOpLogger) WithName(name string) logr.LogSink                    { return l }

func NewSimpleLogger() logr.Logger {
	return logr.New(&simpleLogger{})
}

type simpleLogger struct {
}

func (l *simpleLogger) Init(info logr.RuntimeInfo) {}

func (l *simpleLogger) Enabled(level int) bool {
	// You can add more complex level handling here if needed
	return true
}

func (l *simpleLogger) Info(level int, msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s", msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		fmt.Printf(" %s=%v", keysAndValues[i], keysAndValues[i+1])
	}
	fmt.Println()
}

func (l *simpleLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s: %v", msg, err)
	for i := 0; i < len(keysAndValues); i += 2 {
		fmt.Printf(" %s=%v", keysAndValues[i], keysAndValues[i+1])
	}
	fmt.Println()
}

func (l *simpleLogger) V(level int) logr.LogSink {
	// For simplicity, just return the same logger in this example
	return l
}

func (l *simpleLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	// For simplicity, just return the same logger in this example
	return l
}

func (l *simpleLogger) WithName(name string) logr.LogSink {
	// For simplicity, just return the same logger in this example
	return l
}
