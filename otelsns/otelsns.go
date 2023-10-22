/*
Package otelsns implements carrier for SNS.

# Usage

Use `SnsCarrierAttributes.Inject` to inject trace context into SNS publishing.

	import (
	    "github.com/aws/aws-sdk-go-v2/service/sns"
	    "github.com/aws/aws-sdk-go-v2/service/sns/types"
	    "github.com/udhos/opentelemetry-trace-sqs/otelsns"
	)

	// publish is an example function that uses SnsCarrierAttributes.Inject to
	// propagate tracing context with SNS publishing.
	// 'ctx' holds current tracing context.
	func publish(ctx context.Context, topicArn, msg string) {
	    input := &sns.PublishInput{
	        TopicArn:          aws.String(topicArn),
	        Message:           aws.String(msg),
	        MessageAttributes: make(map[string]types.MessageAttributeValue),
	    }

	    // Inject the tracing context
	    otelsns.NewCarrier().Inject(ctx, input.MessageAttributes)

	    // Now invoke SNS publish for input
*/
package otelsns

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
)

var defaultSnsPropagator = b3.New() // b3 single header

// SetTextMapPropagator optionally replaces the default propagator (B3 with single header).
// Please notice that SNS only supports up to 10 attributes, then be careful when picking
// another propagator that might consume multiple attributes.
func SetTextMapPropagator(propagator propagation.TextMapPropagator) {
	defaultSnsPropagator = propagator
}

// InjectIntoSnsMessageAttributes inserts tracing from context into the SNS message attributes.
//
// Deprecated: Use c := NewCarrier() followed by c.Inject()
func InjectIntoSnsMessageAttributes(ctx context.Context, input *sns.PublishInput) {
	NewCarrier().Inject(ctx, input.MessageAttributes)
}

// SnsCarrierAttributes is a message attribute carrier for SNS.
// https://pkg.go.dev/go.opentelemetry.io/otel/propagation#TextMapCarrier
type SnsCarrierAttributes struct {
	messageAttributes map[string]types.MessageAttributeValue
	propagator        propagation.TextMapPropagator
}

// NewCarrierAttributes creates a carrier attached to an SNS input.
//
// Deprecated: Use c := NewCarrier()
func NewCarrierAttributes(input *sns.PublishInput) *SnsCarrierAttributes {
	c := NewCarrier()
	c.attach(input.MessageAttributes)
	return c
}

// NewCarrier creates a carrier for SNS.
func NewCarrier() *SnsCarrierAttributes {
	c := &SnsCarrierAttributes{}
	return c.WithPropagator(defaultSnsPropagator)
}

// WithPropagator sets propagator for carrier. If unspecified, carrier uses default propagator defined with SetTextMapPropagator.
func (c *SnsCarrierAttributes) WithPropagator(propagator propagation.TextMapPropagator) *SnsCarrierAttributes {
	c.propagator = propagator
	return c
}

// attach attaches carrier to SNS input.
func (c *SnsCarrierAttributes) attach(messageAttributes map[string]types.MessageAttributeValue) {
	if messageAttributes == nil {
		panic("messageAttributes map is nil")
	}
	c.messageAttributes = messageAttributes
}

// Inject inserts tracing from context into the SNS message attributes.
// `ctx` holds current context with trace information.
// `messageAttributes` should point to outgoing SNS publish MessageAttributes which will carry the trace information.
// `messageAttributes` must not be nil.
// Use Inject right before publishing out to SNS.
func (c *SnsCarrierAttributes) Inject(ctx context.Context, messageAttributes map[string]types.MessageAttributeValue) {
	c.attach(messageAttributes)
	c.propagator.Inject(ctx, c)
}

// Get returns the value for the key.
func (c *SnsCarrierAttributes) Get(key string) string {
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
func (c *SnsCarrierAttributes) Set(key, value string) {
	c.messageAttributes[key] = types.MessageAttributeValue{
		DataType:    aws.String(stringType),
		StringValue: aws.String(value),
	}
}

// Keys lists the keys in the carrier.
func (c *SnsCarrierAttributes) Keys() []string {
	keys := make([]string, 0, len(c.messageAttributes))
	for k := range c.messageAttributes {
		keys = append(keys, k)
	}
	return keys
}
