# bhttpv2
Predecessor of bhttp, ideas features

- The middleware will have different signature as "leaf" http handlers
- "leaf" http handlers take a custom (typed) context, a bhttp.ResponseWriter, a http.request and return an error
- the custom "leaf" handler context is constructed from the regular context.Context just before the leaf handler is called, share this logic and error handling
- middleware only take the bhttp.ResponseWriter, a http.request (that carries the regular context.Context) and can handle the error
- named routes and route reversing
