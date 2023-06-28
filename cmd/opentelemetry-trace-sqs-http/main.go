// Package main implements the tool.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/udhos/opentelemetry-trace-sqs/internal/backend"
	"github.com/udhos/opentelemetry-trace-sqs/internal/config"
	"github.com/udhos/opentelemetry-trace-sqs/internal/tracing"
	"github.com/udhos/opentelemetry-trace-sqs/otelsqs"
)

type application struct {
	me          string
	config      config.AppConfig
	server      *http.Server
	tracer      trace.Tracer
	queueInput  backend.SqsQueue
	queueOutput backend.SqsQueue
}

func main() {

	app := &application{
		me:     filepath.Base(os.Args[0]),
		config: config.New(),
	}

	//
	// initialize tracing
	//

	{
		tp, errTracer := tracing.TracerProvider(app.me, app.config.JaegerURL)
		if errTracer != nil {
			log.Fatalf("tracer provider: %v", errTracer)
		}

		// Register our TracerProvider as the global so any imported
		// instrumentation in the future will default to using it.
		otel.SetTracerProvider(tp)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Cleanly shutdown and flush telemetry when the application exits.
		defer func(ctx context.Context) {
			// Do not make the application hang when it is shutdown.
			ctx, cancel = context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			if err := tp.Shutdown(ctx); err != nil {
				log.Fatalf("trace shutdown: %v", err)
			}
		}(ctx)

		tracing.TracePropagation()

		app.tracer = tp.Tracer(fmt.Sprintf("%s-main", app.me))
	}

	//
	// initialize http
	//

	mux := http.NewServeMux()
	app.server = &http.Server{
		Addr:    app.config.HTTPAddr,
		Handler: mux,
	}

	register(mux, app.server.Addr, "handlerRoot", "/", func(w http.ResponseWriter, r *http.Request) { handlerRoot(app, w, r) })
	register(mux, app.server.Addr, "handlerRoute", app.config.HTTPRoute, func(w http.ResponseWriter, r *http.Request) { handlerRoute(app, w, r) })

	//
	// initialize sqs
	//

	app.queueInput = backend.NewSqsClient("input sqs queue", app.config.QueueURLInput, app.config.QueueRoleARNInput, app.me, app.config.EndpointURL)
	app.queueOutput = backend.NewSqsClient("output sqs queue", app.config.QueueURLOutput, app.config.QueueRoleARNOutput, app.me, app.config.EndpointURL)

	//
	// start http server
	//

	go func() {
		log.Printf("application server: listening on %s", app.config.HTTPAddr)
		err := app.server.ListenAndServe()
		log.Fatalf("application server: exited: %v", err)
	}()

	//
	// start sqs
	//

	sqsApp := &backend.SqsApplication{
		QueueInput:  app.queueInput,
		QueueOutput: app.queueOutput,
		Tracer:      app.tracer,
		BackendURL:  app.config.BackendURL,
	}

	go backend.SqsListener(sqsApp)

	<-make(chan struct{}) // wait forever
}

type handler struct {
	f http.HandlerFunc
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.f(w, r)
}

func register(mux *http.ServeMux, operation, addr, path string, handlerFunc http.HandlerFunc) {
	h := &handler{f: handlerFunc}
	mux.Handle(path, otelhttp.NewHandler(h, operation))
	log.Printf("registered %s on port %s path %s", operation, addr, path)
}

func handlerRoot(app *application, w http.ResponseWriter, r *http.Request) {
	const me = "handlerRoot"

	_, span := app.tracer.Start(r.Context(), me)
	defer span.End()

	log.Printf("%s: %s %s %s - 404 not found",
		me, r.RemoteAddr, r.Method, r.RequestURI)

	span.SetStatus(codes.Error, "404 not found")

	http.Error(w, "not found\n", 404)
}

func handlerRoute(app *application, w http.ResponseWriter, r *http.Request) {
	const me = "handlerRoute"

	ctx, span := app.tracer.Start(r.Context(), me)
	defer span.End()

	log.Printf("%s: traceID=%s from HTTP", me, span.SpanContext().TraceID().String())

	buf, errBody := io.ReadAll(r.Body)
	if errBody != nil {
		return
	}

	str := string(buf)

	msg := types.Message{
		Body: &str,
	}

	otelsqs.InjectIntoSqsMessageAttributes(ctx, &msg)

	//
	// send to SQS
	//

	backend.SqsSend(ctx, app.tracer, app.queueOutput, msg)

	//
	// send to HTTP
	//

	errBackend := backend.HTTPBackend(ctx, app.tracer, app.config.BackendURL, bytes.NewBuffer(buf))
	if errBackend != nil {
		m := fmt.Sprintf("%s: %v\n", me, errBackend)
		log.Print(m)
		span.SetStatus(codes.Error, m)

		http.Error(w, m, 500)

		return
	}
}
