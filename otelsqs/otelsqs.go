/*
Package otelsqs implements carrier for SQS.

# Usage

Use `otelsqs.ContextFromSqsMessageAttributes()` to extract trace context from SQS message.

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

Use `otelsqs.InjectIntoSqsMessageAttributes()` to inject trace context into SQS message before sending it.

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
*/
package otelsqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
)

var sqsPropagator = b3.New() // b3 single header

// SetTextMapPropagator optionally replaces the default propagator (B3 with single header).
// Please notice that SQS only supports up to 10 attributes, then be careful when picking
// another propagator that might consume multiple attributes.
func SetTextMapPropagator(propagator propagation.TextMapPropagator) {
	sqsPropagator = propagator
}

// ContextFromSqsMessageAttributes gets a tracing context from SQS message attributes.
// `sqsMessage` is incoming, received SQS message (possibly) carring trace information in the message attributes.
// Use ContextFromSqsMessageAttributes right after receiving an SQS message.
func ContextFromSqsMessageAttributes(sqsMessage *types.Message) context.Context {
	ctxOrig := context.Background()
	carrier := NewCarrierAttributes(sqsMessage)
	ctx := sqsPropagator.Extract(ctxOrig, carrier)
	return ctx
}

// InjectIntoSqsMessageAttributes inserts tracing from context into the SQS message attributes.
// `ctx` holds current context with trace information.
// `sqsMessage` is outgoing SQS message that will be set to carry trace information.
// Use InjectIntoSqsMessageAttributes right before sending out the SQS message.
func InjectIntoSqsMessageAttributes(ctx context.Context, sqsMessage *types.Message) {
	carrier := NewCarrierAttributes(sqsMessage)
	sqsPropagator.Inject(ctx, carrier)
}

// SqsCarrierAttributes is a message attribute carrier for SQS.
// https://pkg.go.dev/go.opentelemetry.io/otel/propagation#TextMapCarrier
type SqsCarrierAttributes struct {
	sqsMessage *types.Message
}

// NewCarrierAttributes creates a carrier for SQS.
func NewCarrierAttributes(sqsMessage *types.Message) *SqsCarrierAttributes {
	return &SqsCarrierAttributes{
		sqsMessage: sqsMessage,
	}
}

// Get returns the value for the key.
func (c *SqsCarrierAttributes) Get(key string) string {
	attr, found := c.sqsMessage.MessageAttributes[key]
	if !found {
		return ""
	}
	if attr.StringValue == nil {
		return ""
	}
	return *attr.StringValue
}

const stringType = "String"

// Set stores a key-value pair.
func (c *SqsCarrierAttributes) Set(key, value string) {
	if c.sqsMessage.MessageAttributes == nil {
		c.sqsMessage.MessageAttributes = map[string]types.MessageAttributeValue{}
	}
	c.sqsMessage.MessageAttributes[key] = types.MessageAttributeValue{
		DataType:    aws.String(stringType),
		StringValue: aws.String(value),
	}
}

// Keys lists the keys in the carrier.
func (c *SqsCarrierAttributes) Keys() []string {
	if c.sqsMessage.MessageAttributes == nil {
		return nil
	}
	keys := make([]string, 0, len(c.sqsMessage.MessageAttributes))
	for k := range c.sqsMessage.MessageAttributes {
		keys = append(keys, k)
	}
	return keys
}
