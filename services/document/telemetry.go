package document

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func newTraceMiddleware(operation string) func(http.Handler) http.Handler {
	return otelhttp.NewMiddleware(operation, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		if r.Pattern != "" {
			return r.Method + " " + r.Pattern
		}
		return r.Method + " " + r.URL.Path
	}))
}

func traceRoute(route string, handler http.HandlerFunc) http.HandlerFunc {
	wrapped := otelhttp.WithRouteTag(route, http.HandlerFunc(handler))
	return func(w http.ResponseWriter, r *http.Request) {
		wrapped.ServeHTTP(w, r)
	}
}
