# bhttp
Pragmatic utilities for better http handlers in Go. Typed context, url reversing and error returns for Go http handlers 

## features
- type-safe and modular context.Context for http handlers
- reversing of http route patterns
- return errors from handlers, and allow middleware to handle them
- benchmark seem to suggest it also reduces alloc/op but i might be doing something wrong with the benchmarks

## Downsides
- the http.Request's context should not be used in the handler
- about 2 extra allocations per request
- quiet a lot of type parameters will pop-up, this decreases readability
