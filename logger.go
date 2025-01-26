package bhttp

import (
	"log"
	"sync/atomic"
	"testing"
)

// Logger can be implemented to get informed about important states.
type Logger interface {
	LogUnhandledServeError(err error)
	LogImplicitFlushError(err error)
}

type stdLogger struct{ *log.Logger }

func (l stdLogger) LogUnhandledServeError(err error) {
	l.Logger.Printf("bhttp: unhandled server error: %s", err)
}

func (l stdLogger) LogImplicitFlushError(err error) {
	l.Logger.Printf("bhttp: error while flushing implicitly: %s", err)
}

func NewStdLogger(l *log.Logger) Logger {
	return stdLogger{l}
}

type TestLogger struct {
	tb testing.TB

	NumLogUnhandledServeError int64
	NumLogImplicitFlushError  int64
}

func NewTestLogger(tb testing.TB) *TestLogger {
	return &TestLogger{tb: tb}
}

func (l *TestLogger) LogUnhandledServeError(err error) {
	atomic.AddInt64(&l.NumLogUnhandledServeError, 1)
	l.tb.Logf("bhttp: unhandled server error: %s", err)
}

func (l *TestLogger) LogImplicitFlushError(err error) {
	atomic.AddInt64(&l.NumLogImplicitFlushError, 1)
	l.tb.Logf("bhttp: error while flushing implicitly: %s", err)
}

var _ Logger = &TestLogger{}
