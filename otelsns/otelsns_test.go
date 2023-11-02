package otelsns

import (
	"context"
	"log"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	sqs_types "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/trace"

	"github.com/udhos/opentelemetry-trace-sqs/otelsqs"
	"github.com/udhos/otelconfig/oteltrace"
)

func TestSnSInjectExtract(t *testing.T) {
	//
	// initialize tracing
	//
	const me = "TestSnSInjectExtract"

	var tracer trace.Tracer

	{
		options := oteltrace.TraceOptions{
			DefaultService:     me,
			NoopTracerProvider: true,
			Debug:              true,
		}

		tr, cancel, errTracer := oteltrace.TraceStart(options)

		if errTracer != nil {
			log.Fatalf("tracer: %v", errTracer)
		}

		defer cancel()

		tracer = tr
	}

	ctx, span := tracer.Start(context.TODO(), me)
	defer span.End()

	traceIDSent := span.SpanContext().TraceID().String()

	t.Logf("traceIDSent:%s", traceIDSent)

	//
	// Send
	//

	info := "hello"
	input := sns.PublishInput{
		Message:           aws.String(info),
		MessageAttributes: make(map[string]types.MessageAttributeValue),
	}
	carrier := NewCarrier()
	carrier.Inject(ctx, input.MessageAttributes)

	//
	// Receive
	//

	msg := sqs_types.Message{
		MessageAttributes: make(map[string]sqs_types.MessageAttributeValue),
	}

	copyAttributes(&input, &msg)

	carrierSQS := otelsqs.NewCarrier()
	ctxNew := carrierSQS.Extract(ctx, msg.MessageAttributes)

	_, span2 := tracer.Start(ctxNew, me)
	defer span2.End()

	traceIDRecv := span2.SpanContext().TraceID().String()

	t.Logf("traceIDRecv:%s", traceIDRecv)

	if traceIDSent != traceIDRecv {
		t.Errorf("traceIDSent:%s mismatches traceIDRecv:%s", traceIDSent, traceIDRecv)
	}
}

func copyAttributes(from *sns.PublishInput, to *sqs_types.Message) {
	attr := map[string]sqs_types.MessageAttributeValue{}
	for k, v := range from.MessageAttributes {
		attr[k] = sqs_types.MessageAttributeValue{
			DataType:    v.DataType,
			BinaryValue: v.BinaryValue,
			StringValue: v.StringValue,
		}
	}
	to.MessageAttributes = attr
}

func TestSqsCarrierAttributes(t *testing.T) {
	input := sns.PublishInput{
		MessageAttributes: make(map[string]types.MessageAttributeValue),
	}
	carrier := NewCarrierAttributes(&input)

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
