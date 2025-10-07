package monitoring

import (
	"io"
	"net/http"
	"unicode/utf8"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"modernc.org/mathutil"
)

func spanFormat(operation string, r *http.Request) string {
	return operation + " " + r.URL.Path
}

func NewTraceRoundTripper() http.RoundTripper {
	return NewWrappedTraceRoundTripper(http.DefaultTransport)
}

func NewWrappedTraceRoundTripper(wrap http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(
		&wrapperRoundTripper{
			wrapped: wrap,
		},
		otelhttp.WithSpanNameFormatter(spanFormat),
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
	)
}

type wrapperRoundTripper struct {
	wrapped http.RoundTripper
}

func (w *wrapperRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	span := trace.SpanFromContext(req.Context())
	span.SetAttributes(attribute.String("http.route", req.URL.Path), attribute.String("http.target", req.URL.Path))

	if req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil && body != nil {
			bodyBytes, err := io.ReadAll(body)
			if err == nil {
				length := mathutil.Min(len(bodyBytes), 512)
				if length > 0 {
					bodyStr := string(bodyBytes)[:length]
					if utf8.ValidString(bodyStr) {
						span.SetAttributes(attribute.String("http.body", bodyStr))
					}
				}
			}
		}
	}

	return w.wrapped.RoundTrip(req)
}
