package bhttp

import "log"

// Logger can be implemented to get informed about important states.
type Logger interface {
	LogUnhandledServeError(err error)
	LogImplicitFlushError(err error)
}

type defaultLogger struct{ *log.Logger }

func (l defaultLogger) LogUnhandledServeError(err error) {
	l.Logger.Printf("clserve: error not handled by middleware: %s", err)
}

func (l defaultLogger) LogImplicitFlushError(err error) {
	l.Logger.Printf("clserve: error while flysing implicitly: %s", err)
}
