// Package main implements the tool.
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/udhos/opentelemetry-trace-sqs/internal/backend"
	"github.com/udhos/opentelemetry-trace-sqs/internal/config"
	"github.com/udhos/opentelemetry-trace-sqs/otelsqs"
	"github.com/udhos/otelconfig/oteltrace"
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

	debug := os.Getenv("DEBUG") == "true"

	//
	// initialize tracing
	//

	{
		options := oteltrace.TraceOptions{
			DefaultService:     app.me,
			NoopTracerProvider: false,
			Debug:              true,
		}

		tr, cancel, errTracer := oteltrace.TraceStart(options)

		if errTracer != nil {
			log.Fatalf("tracer: %v", errTracer)
		}

		defer cancel()

		app.tracer = tr
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
		Debug:       debug,
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
		Body:              &str,
		MessageAttributes: make(map[string]types.MessageAttributeValue),
	}

	if errInject := otelsqs.NewCarrier().Inject(ctx, msg.MessageAttributes); errInject != nil {
		log.Fatalf("inject error: %v", errInject)
	}

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
