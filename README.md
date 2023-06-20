# opentelemetry-trace-sqs
opentelemetry-trace-sqs

Open Telemetry tracing:

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

```
# Jaeger
./run-jaeger-local.sh

open jaeger: http://localhost:16686

# Server 1
export OTEL_SERVICE_NAME=opentelemetry-trace-sqs-gin-1
opentelemetry-trace-sqs-gin

# Server 2
export OTEL_SERVICE_NAME=opentelemetry-trace-sqs-gin-2
export HTTP_ADDR=:8002
export BACKEND_URL=http://localhost:8003/send
opentelemetry-trace-sqs-gin

# Server 3
export OTEL_SERVICE_NAME=opentelemetry-trace-sqs-gin-3
export HTTP_ADDR=:8003
export BACKEND_URL=http://wrong:8002/send
opentelemetry-trace-sqs-gin

curl -d '{"a":"b"}' localhost:8001/send
```