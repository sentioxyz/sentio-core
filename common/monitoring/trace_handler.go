package monitoring

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type wrapperHandler struct {
	wrapped http.Handler
}

func NewWrappedHandler(handler http.Handler, serverName string) http.Handler {
	return &wrapperHandler{otelhttp.NewHandler(handler, serverName,
		otelhttp.WithSpanNameFormatter(spanFormat),
		otelhttp.WithTracerProvider(otel.GetTracerProvider()))}
}

func (h *wrapperHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	span := trace.SpanFromContext(req.Context())
	span.SetAttributes(attribute.String("http.route", req.URL.Path),
		attribute.String("http.target", req.URL.Path))
	h.wrapped.ServeHTTP(writer, req)
}
