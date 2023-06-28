// Package backend forwards requests to SQS and HTTP.
package backend

import (
	"context"
	"io"
	"log"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// HTTPBackend forwards body to http backend.
func HTTPBackend(ctx context.Context, tracer trace.Tracer, backendURL string, body io.Reader) error {
	const me = "HTTPBackend"

	newCtx, span := tracer.Start(ctx, me)
	defer span.End()

	log.Printf("%s: traceID=%s", me, span.SpanContext().TraceID())

	req, errReq := http.NewRequestWithContext(newCtx, "POST", backendURL, body)
	if errReq != nil {
		log.Printf("%s: URL=%s request error: %v", me, backendURL, errReq)
		return errReq
	}

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	resp, errGet := client.Do(req)
	if errGet != nil {
		log.Printf("%s: URL=%s server error: %v", me, backendURL, errGet)
		return errGet
	}

	defer resp.Body.Close()

	return nil
}
