// Package tracing provides utilities for working with open telemetry tracing.
package tracing

import (
	"log"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

/*
Open Telemetry tracing with Gin:

1) Initialize the tracing (see main.go)
2) Enable trace propagation (see tracePropagation below)
3) Use handler middleware (see main.go)
   import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
   router.Use(otelgin.Middleware("virtual-service"))
4) For http client, create a Request from Context (see backend.go)
   newCtx, span := b.tracer.Start(ctx, "backendHTTP.fetch")
   req, errReq := http.NewRequestWithContext(newCtx, "GET", u, nil)
   client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
   resp, errGet := client.Do(req)
*/

// TracerProvider creates a trace provider.
// Service name precedence from higher to lower:
// 1. OTEL_SERVICE_NAME=mysrv
// 2. OTEL_RESOURCE_ATTRIBUTES=service.name=mysrv
// 3. defaultService="mysrv"
func TracerProvider(defaultService, url string) (*tracesdk.TracerProvider, error) {
	log.Printf("tracerProvider: service=%s collector=%s", defaultService, url)

	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	var rsrc *resource.Resource

	if defaultService == "" || hasServiceEnvVar() {
		rsrc = resource.NewWithAttributes(
			semconv.SchemaURL,
			//attribute.String("environment", environment),
			//attribute.Int64("ID", id),
		)
	} else {
		rsrc = resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(defaultService),
			//attribute.String("environment", environment),
			//attribute.Int64("ID", id),
		)
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(rsrc),
	)
	return tp, nil
}

func hasServiceEnvVar() bool {
	const me = "hasServiceEnvVar"

	svc := os.Getenv("OTEL_SERVICE_NAME")

	if strings.TrimSpace(svc) != "" {
		log.Printf("%s: found OTEL_SERVICE_NAME=%s", me, svc)
		return true
	}

	attrs := os.Getenv("OTEL_RESOURCE_ATTRIBUTES")
	fields := strings.FieldsFunc(attrs, func(c rune) bool { return c == ',' })
	for _, f := range fields {
		key, val, _ := strings.Cut(f, "=")
		if key == "service.name" {
			log.Printf("%s: found OTEL_RESOURCE_ATTRIBUTES: %s=%s",
				me, key, val)
			return true
		}
	}

	return false
}

// TracePropagation enables trace propagation.
func TracePropagation() {
	// In order to propagate trace context over the wire, a propagator must be registered with the OpenTelemetry API.
	// https://opentelemetry.io/docs/instrumentation/go/manual/
	//otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)),
		//propagation.Baggage{},
		//propagation.TraceContext{},
		//ot.OT{},
	))
}
