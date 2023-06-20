// Package main implements the tool.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/udhos/opentelemetry-trace-sqs/internal/env"
	"github.com/udhos/opentelemetry-trace-sqs/internal/tracing"
)

type appConfig struct {
	httpAddr       string
	httpRoute      string
	jaegerURL      string
	queueURLInput  string
	queueURLOutput string
	backendURL     string
}

type application struct {
	me     string
	config appConfig
	server *serverGin
	tracer trace.Tracer
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

	app := application{
		me: filepath.Base(os.Args[0]),
		config: appConfig{
			httpAddr:       env.String("HTTP_ADDR", ":8001"),
			httpRoute:      env.String("HTTP_ROUTE", "/send"),
			jaegerURL:      env.String("JAEGER_URL", "http://jaeger-collector:14268/api/traces"),
			queueURLInput:  env.String("QUEUE_URL_INPUT", ""),
			queueURLOutput: env.String("QUEUE_URL_OUTPUT", ""),
			backendURL:     env.String("BACKEND_URL", "http://localhost:8002/send"),
		},
	}

	//
	// initialize tracing
	//

	{
		tp, errTracer := tracing.TracerProvider(app.me, app.config.jaegerURL)
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

	app.server = newServerGin(app.config.httpAddr)
	app.server.router.Use(otelgin.Middleware(app.me))
	app.server.router.Use(gin.Logger())
	app.server.router.Any(app.config.httpRoute, func(c *gin.Context) { handlerRoute(c, &app) })

	//
	// start http server
	//

	go func() {
		log.Printf("application server: listening on %s", app.config.httpAddr)
		err := app.server.server.ListenAndServe()
		log.Fatalf("application server: exited: %v", err)
	}()

	<-make(chan struct{}) // wait forever
}

func handlerRoute(c *gin.Context, app *application) {
	const me = "handlerRoute"

	ctx := c.Request.Context()
	newCtx, span := app.tracer.Start(ctx, me)
	defer span.End()

	log.Printf("%s: traceID=%s", me, span.SpanContext().TraceID())

	errBackend := backend(newCtx, app, c.Request.Body)
	if errBackend != nil {
		msg := fmt.Sprintf("%s: %v", me, errBackend)
		span.SetStatus(codes.Error, msg)
		c.String(500, msg)
		return
	}
}

func backend(ctx context.Context, app *application, body io.Reader) error {
	const me = "backend"

	newCtx, span := app.tracer.Start(ctx, me)
	defer span.End()

	log.Printf("%s: traceID=%s", me, span.SpanContext().TraceID())

	req, errReq := http.NewRequestWithContext(newCtx, "POST", app.config.backendURL, body)
	if errReq != nil {
		log.Printf("%s: URL=%s request error: %v", me, app.config.backendURL, errReq)
		return errReq
	}

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	resp, errGet := client.Do(req)
	if errGet != nil {
		log.Printf("%s: URL=%s server error: %v", me, app.config.backendURL, errGet)
		return errGet
	}

	defer resp.Body.Close()

	return nil
}
