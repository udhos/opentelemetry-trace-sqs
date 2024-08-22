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
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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
	server      *serverGin
	tracer      trace.Tracer
	queueInput  backend.SqsQueue
	queueOutput backend.SqsQueue
}

type serverGin struct {
	server *http.Server
	router *gin.Engine
}

func newServerGin(addr string) *serverGin {
	r := gin.New()
	return &serverGin{
		router: r,
		server: &http.Server{Addr: addr, Handler: r},
	}
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

	app.server = newServerGin(app.config.HTTPAddr)
	app.server.router.Use(otelgin.Middleware(app.me))
	app.server.router.Use(gin.Logger())
	app.server.router.Any(app.config.HTTPRoute, func(c *gin.Context) { handlerRoute(c, app) })

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
		err := app.server.server.ListenAndServe()
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

func handlerRoute(c *gin.Context, app *application) {
	const me = "handlerRoute"

	ctx, span := app.tracer.Start(c.Request.Context(), me)
	defer span.End()

	log.Printf("%s: traceID=%s from HTTP", me, span.SpanContext().TraceID().String())

	buf, errBody := io.ReadAll(c.Request.Body)
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
		c.String(500, m)
		return
	}
}
