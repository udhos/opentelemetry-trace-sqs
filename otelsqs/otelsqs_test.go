package otelsqs

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/udhos/opentelemetry-trace-sqs/internal/env"
	"github.com/udhos/opentelemetry-trace-sqs/internal/tracing"
)

func TestSqsInjectExtract(t *testing.T) {
	//
	// initialize tracing
	//
	const me = "TestSqsInjectExtract"

	jaegerURL := env.String("JAEGER_URL", "http://jaeger-collector:14268/api/traces")

	var tracer trace.Tracer

	{
		tp, errTracer := tracing.TracerProvider(me, jaegerURL)
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

		tracer = tp.Tracer(me)
	}

	ctx, span := tracer.Start(context.TODO(), me)
	defer span.End()

	traceIDSent := span.SpanContext().TraceID().String()

	t.Logf("traceIDSent:%s", traceIDSent)

	//
	// Send
	//

	info := "hello"
	msg := types.Message{
		Body: aws.String(info),
	}
	carrier := NewCarrier()
	carrier.Inject(ctx, &msg)

	//
	// Receive
	//

	ctxNew := carrier.Extract(&msg)

	_, span2 := tracer.Start(ctxNew, me)
	defer span2.End()

	traceIDRecv := span2.SpanContext().TraceID().String()

	t.Logf("traceIDRecv:%s", traceIDRecv)

	if traceIDSent != traceIDRecv {
		t.Errorf("traceIDSent:%s mismatches traceIDRecv:%s", traceIDSent, traceIDRecv)
	}
}

func TestSqsCarrierAttributes(t *testing.T) {
	sqsMessage := types.Message{}
	carrier := NewCarrierAttributes(&sqsMessage)

	// no keys

	if len(carrier.Keys()) != 0 {
		t.Errorf("expected empty carrier")
	}

	if value1 := carrier.Get("key1"); value1 != "" {
		t.Errorf("found unexpected key key1")
	}

	// add key1

	carrier.Set("key1", "value1")

	if len(carrier.Keys()) != 1 {
		t.Errorf("expected only one key")
	}

	if carrier.Get("key1") != "value1" {
		t.Errorf("wrong value for key1")
	}

	// change key1

	carrier.Set("key1", "value2")

	if len(carrier.Keys()) != 1 {
		t.Errorf("expected only one key")
	}

	if carrier.Get("key1") != "value2" {
		t.Errorf("wrong value for key1")
	}

	// add key2

	carrier.Set("key2", "value3")

	if len(carrier.Keys()) != 2 {
		t.Errorf("expected two keys")
	}

	if carrier.Get("key1") != "value2" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value3" {
		t.Errorf("wrong value for key2")
	}

	// change key1

	carrier.Set("key1", "value11")

	if len(carrier.Keys()) != 2 {
		t.Errorf("expected two keys")
	}

	if carrier.Get("key1") != "value11" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value3" {
		t.Errorf("wrong value for key2")
	}

	// change key2

	carrier.Set("key2", "value22")

	if len(carrier.Keys()) != 2 {
		t.Errorf("expected two keys")
	}

	if carrier.Get("key1") != "value11" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value22" {
		t.Errorf("wrong value for key2")
	}

	// add key3

	carrier.Set("key3", "value3")

	if len(carrier.Keys()) != 3 {
		t.Errorf("expected three keys")
	}

	if carrier.Get("key1") != "value11" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value22" {
		t.Errorf("wrong value for key2")
	}

	if carrier.Get("key3") != "value3" {
		t.Errorf("wrong value for key3")
	}
}
