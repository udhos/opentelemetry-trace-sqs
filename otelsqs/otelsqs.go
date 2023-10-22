/*
Package otelsqs implements carrier for SQS.

# Usage

Use `SqsCarrierAttributes.Extract()` to extract trace context from SQS message.

	import (
	    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	    "github.com/udhos/opentelemetry-trace-sqs/otelsqs"
	)

	// handleSQSMessage is an example function that uses SqsCarrierAttributes.Extract to
	// extract tracing context from inbound SQS message.
	func handleSQSMessage(app *application, inboundSqsMessage types.Message) {
	    // Extract the tracing context from a received SQS message
	    ctx := otelsqs.NewCarrier().Extract(inboundSqsMessage.MessageAttributes)

	    // Use the trace context as usual, for instance, starting a new span
	    ctxNew, span := app.tracer.Start(ctx, "handleSQSMessage")
	    defer span.End()

	    // One could log the traceID
	    log.Printf("handleSQSMessage: traceID=%s", span.SpanContext().TraceID().String())

	    // Now handle the SQS message

Use `SqsCarrierAttributes.Inject()` to inject trace context into SQS message before sending it.

	import (
	    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	    "github.com/udhos/opentelemetry-trace-sqs/otelsqs"
	)

	// sendSQSMessage is an example function that uses SqsCarrierAttributes.Inject to
	// propagate tracing context into outgoing SQS message.
	// 'ctx' holds current tracing context.
	func sendSQSMessage(ctx context.Context, app *application, outboundSqsMessage types.Message) {
	    // You have a trace context in 'ctx' that you need to propagate into SQS message 'outboundSqsMessage'
	    ctxNew, span := app.tracer.Start(ctx, "sendSQSMessage")
	    defer span.End()

	    // Inject the tracing context
	    otelsqs.NewCarrier().Inject(ctxNew, outboundSqsMessage.MessageAttributes)

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

const sqsMessageAttributeLimit = 10

var defaultSqsPropagator = b3.New() // b3 single header

// SetTextMapPropagator optionally replaces the default propagator (B3 with single header).
// Please notice that SQS only supports up to 10 attributes, then be careful when picking
// another propagator that might consume multiple attributes.
func SetTextMapPropagator(propagator propagation.TextMapPropagator) {
	defaultSqsPropagator = propagator
}

// ContextFromSqsMessageAttributes gets a tracing context from SQS message attributes.
// `sqsMessage` is incoming, received SQS message (possibly) carring trace information in the message attributes.
// Use ContextFromSqsMessageAttributes right after receiving an SQS message.
//
// Deprecated: Use c := NewCarrier() followed by c.Extract().
func ContextFromSqsMessageAttributes(sqsMessage *types.Message) context.Context {
	return NewCarrier().Extract(sqsMessage.MessageAttributes)
}

// InjectIntoSqsMessageAttributes inserts tracing from context into the SQS message attributes.
// `ctx` holds current context with trace information.
// `sqsMessage` is outgoing SQS message that will be set to carry trace information.
// Use InjectIntoSqsMessageAttributes right before sending out the SQS message.
//
// Deprecated: Use c := NewCarrier() followed by c.Inject().
func InjectIntoSqsMessageAttributes(ctx context.Context, sqsMessage *types.Message) {
	NewCarrier().Inject(ctx, sqsMessage.MessageAttributes)
}

// SqsCarrierAttributes is a message attribute carrier for SQS.
// https://pkg.go.dev/go.opentelemetry.io/otel/propagation#TextMapCarrier
type SqsCarrierAttributes struct {
	messageAttributes map[string]types.MessageAttributeValue
	propagator        propagation.TextMapPropagator
}

// NewCarrierAttributes creates a carrier attached to an SQS message.
//
// Deprecated: Use NewCarrier().
func NewCarrierAttributes(sqsMessage *types.Message) *SqsCarrierAttributes {
	c := NewCarrier()
	c.attach(sqsMessage.MessageAttributes)
	return c
}

// NewCarrier creates a carrier for SQS.
func NewCarrier() *SqsCarrierAttributes {
	c := &SqsCarrierAttributes{}
	return c.WithPropagator(defaultSqsPropagator)
}

// WithPropagator sets propagator for carrier. If unspecified, carrier uses default propagator defined with SetTextMapPropagator.
func (c *SqsCarrierAttributes) WithPropagator(propagator propagation.TextMapPropagator) *SqsCarrierAttributes {
	c.propagator = propagator
	return c
}

// attach attaches carrier to SQS message.
func (c *SqsCarrierAttributes) attach(messageAttributes map[string]types.MessageAttributeValue) {
	if messageAttributes == nil {
		panic("messageAttributes map is nil")
	}
	c.messageAttributes = messageAttributes
}

// Extract gets a tracing context from SQS message attributes.
// `messageAttributes` should point to incoming SQS message MessageAttributes (possibly) carring trace information.
// If `messageAttributes` is nil, a blank new empty context is returned.
// Use Extract right after receiving an SQS message.
func (c *SqsCarrierAttributes) Extract(messageAttributes map[string]types.MessageAttributeValue) context.Context {
	ctx := context.Background()
	if messageAttributes == nil {
		return ctx
	}
	c.attach(messageAttributes)
	return c.propagator.Extract(ctx, c)
}

// Inject inserts tracing from context into the SQS message attributes.
// `ctx` holds current context with trace information.
// `messageAttributes` should point to outgoing SQS message MessageAttributes which will carry the trace information.
// `messageAttributes` must not be nil.
// If `messageAttributes` holds 10 or more items, Inject will do nothing, since SQS refuses messages with more than 10 attributes.
// Use Inject right before sending out the SQS message.
func (c *SqsCarrierAttributes) Inject(ctx context.Context, messageAttributes map[string]types.MessageAttributeValue) {
	if len(messageAttributes) >= sqsMessageAttributeLimit {
		return
	}
	c.attach(messageAttributes)
	c.propagator.Inject(ctx, c)
}

// Get returns the value for the key.
func (c *SqsCarrierAttributes) Get(key string) string {
	attr, found := c.messageAttributes[key]
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
	c.messageAttributes[key] = types.MessageAttributeValue{
		DataType:    aws.String(stringType),
		StringValue: aws.String(value),
	}
}

// Keys lists the keys in the carrier.
func (c *SqsCarrierAttributes) Keys() []string {
	keys := make([]string, 0, len(c.messageAttributes))
	for k := range c.messageAttributes {
		keys = append(keys, k)
	}
	return keys
}
