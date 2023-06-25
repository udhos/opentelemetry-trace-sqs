[![license](http://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/udhos/opentelemetry-trace-sqs/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/udhos/opentelemetry-trace-sqs)](https://goreportcard.com/report/github.com/udhos/opentelemetry-trace-sqs)
[![Go Reference](https://pkg.go.dev/badge/github.com/udhos/opentelemetry-trace-sqs.svg)](https://pkg.go.dev/github.com/udhos/opentelemetry-trace-sqs)

# opentelemetry-trace-sqs

[opentelemetry-trace-sqs](https://github.com/udhos/opentelemetry-trace-sqs) propagates Open Telemetry tracing with SQS messages.

# B3 Propagation

https://github.com/openzipkin/b3-propagation

# Tracing propagation with SQS

## Extract trace from SQS received message

Use `otelsqs.ContextFromSqsMessageAttributes()` to extract trace context from SQS message.

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
    "github.com/udhos/opentelemetry-trace-sqs/otelsqs"
)

func handleSQSMessage(app *application, inboundSqsMessage types.Message) {
    // Extract the tracing context from a received SQS message
    ctx := otelsqs.ContextFromSqsMessageAttributes(&inboundSqsMessage)

    // Use the trace context as usual, for instance, starting a new span
    ctxNew, span := app.tracer.Start(ctx, "handleSQSMessage")
    defer span.End()

    // One could log the traceID
    log.Printf("handleSQSMessage: traceID=%s", span.SpanContext().TraceID().String())

    // Now handle the SQS message
```

## Inject trace context into SQS message before sending

Use `otelsqs.InjectIntoSqsMessageAttributes()` to inject trace context into SQS message before sending it.

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
    "github.com/udhos/opentelemetry-trace-sqs/otelsqs"
)

// 'ctx' is current tracing context
func sendSQSMessage(ctx context.Context, app *application, outboundSqsMessage types.Message) {
    // You have a trace context in 'ctx' that you need to propagate into SQS message 'outboundSqsMessage'
    ctxNew, span := app.tracer.Start(ctx, "sendSQSMessage")
    defer span.End()

    // Inject the tracing context
    otelsqs.InjectIntoSqsMessageAttributes(ctxNew, &outboundSqsMessage)

    // Now you can send the SQS message
```

# Open Telemetry tracing recipe for GIN and HTTP

1. Initialize the tracing - see main.go
2. Enable trace propagation - see internal/tracing
3. Retrieve tracing from request context

3.1. If using GIN

GIN - Use otelgin middleware

```go
// gin
import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
router.Use(otelgin.Middleware("virtual-service"))
```

GIN - Get context with c.Request.Context()

```go
// gin
func handlerRoute(c *gin.Context, app *application) {
    const me = "handlerRoute"
    ctx, span := app.tracer.Start(c.Request.Context(), me)
    defer span.End()
// ...
```

3.2. If using standard http package

HTTP - Wrap handler with otelhttp.NewHandler

```go
wrappedHandler := otelhttp.NewHandler(handler, "hello-instrumented")
http.Handle("/hello-instrumented", wrappedHandler)
```

HTTP - Get context with r.Context()

```go
func httpHandler(w http.ResponseWriter, r *http.Request) {
    const me = "httpHandler"
    ctx, span := app.tracer.Start(r.Context(), me)
    defer span.End()
// ...
```

4. For http client, create a Request from Context and wrap transport with otelhttp.NewTransport

```go
newCtx, span := app.tracer.Start(ctx, "backendHTTP.fetch")
req, errReq := http.NewRequestWithContext(newCtx, "GET", u, nil)
client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
resp, errGet := client.Do(req)
```

# Test trace propagation across SQS

```
# Jaeger
./run-jaeger-local.sh

open jaeger: http://localhost:16686

# Server 1
export QUEUE_URL_INPUT=https://sqs.us-east-1.amazonaws.com/100010001000/q1
export QUEUE_URL_OUTPUT=https://sqs.us-east-1.amazonaws.com/100010001000/q2
export OTEL_SERVICE_NAME=opentelemetry-trace-sqs-gin-1
export HTTP_ADDR=:8001
export BACKEND_URL=http://localhost:8002/send
opentelemetry-trace-sqs-gin

# Server 2
export QUEUE_URL_INPUT=https://sqs.us-east-1.amazonaws.com/100010001000/q2
export QUEUE_URL_OUTPUT=https://sqs.us-east-1.amazonaws.com/100010001000/q3
export OTEL_SERVICE_NAME=opentelemetry-trace-sqs-gin-2
export HTTP_ADDR=:8002
export BACKEND_URL=http://localhost:8003/send
opentelemetry-trace-sqs-gin

# Server 3
export QUEUE_URL_INPUT=https://sqs.us-east-1.amazonaws.com/100010001000/q3
export QUEUE_URL_INPUT=https://sqs.us-east-1.amazonaws.com/100010001000/q4
export OTEL_SERVICE_NAME=opentelemetry-trace-sqs-gin-3
export HTTP_ADDR=:8003
export BACKEND_URL=http://wrong:8002/send
opentelemetry-trace-sqs-gin

curl -d '{"a":"b"}' localhost:8001/send
```