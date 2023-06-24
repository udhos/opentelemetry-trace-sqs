# opentelemetry-trace-sqs

opentelemetry-trace-sqs

# B3 Propagation

https://github.com/openzipkin/b3-propagation

# Tracing propagation with SQS

## Extract trace from SQS received message

Use `sqsotel.ContextFromSqsMessageAttributes()` to extract trace context from SQS message.

```go
import "github.com/udhos/opentelemetry-trace-sqs/sqsotel"

func handleSQSMessage(app *application, msg types.Message) {
	ctx := sqsotel.ContextFromSqsMessageAttributes(&msg)
	ctxNew, span := app.tracer.Start(ctx, "handleSQSMessage")
	defer span.End()
```

## Inject trace context into SQS message before sending

```go
// ctx is currenct tracing context
// msg is SQS message
sqsotel.InjectIntoSqsAttributes(ctx, &msg)
```


# Open Telemetry tracing recipe for GIN and HTTP

1) Initialize the tracing - see main.go
2) Enable trace propagation - see internal/tracing
3) Retrieve Retrieve tracing from request context

3.1) If using GIN

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

3.2) If using standard http package

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

4) For http client, create a Request from Context and wrap transport with otelhttp.NewTransport

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