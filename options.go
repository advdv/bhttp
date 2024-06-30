package bhttp

import "log"

// Option allows for customization of the (buffered) handling logic.
type Option func(*opts)

// WithBufferLimit allows limiting the buffered writer (if it buffered). This can protect against
// buffered response writers taking up too much memory per response.
func WithBufferLimit(v int) Option {
	return func(o *opts) { o.bufLimit = v }
}

// WithErrorLog will add a standard library logger to allow observing some failure situations. If not
// specified it will log to log.Default().
func WithErrorLog(v Logger) Option {
	return func(o *opts) { o.logger = v }
}

// opts for the handling.
type opts struct {
	bufLimit int
	logger   Logger
}

// applyOptions applies options and sets sensible default.
func applyOptions(olist []Option) opts {
	var merged opts
	merged.bufLimit = -1
	merged.logger = defaultLogger{log.Default()}

	for _, opt := range olist {
		opt(&merged)
	}

	return merged
}
