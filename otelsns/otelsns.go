// Package otelsns implements carrier for SNS.
package otelsns

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
)

var snsPropagator = b3.New() // b3 single header

// SetTextMapPropagator optionally replaces the default propagator (B3 with single header).
// Please notice that SNS only supports up to 10 attributes, then be careful when picking
// another propagator that might consume multiple attributes.
func SetTextMapPropagator(propagator propagation.TextMapPropagator) {
	snsPropagator = propagator
}

// InjectIntoSnsMessageAttributes inserts tracing from context into the SNS message attributes.
func InjectIntoSnsMessageAttributes(ctx context.Context, input *sns.PublishInput) {
	carrier := NewCarrierAttributes(input)
	snsPropagator.Inject(ctx, carrier)
}

// SnsCarrierAttributes is a message attribute carrier for SNS.
// https://pkg.go.dev/go.opentelemetry.io/otel/propagation#TextMapCarrier
type SnsCarrierAttributes struct {
	input *sns.PublishInput
}

// NewCarrierAttributes creates a carrier for SNS.
func NewCarrierAttributes(input *sns.PublishInput) *SnsCarrierAttributes {
	return &SnsCarrierAttributes{
		input: input,
	}
}

// Get returns the value for the key.
func (c *SnsCarrierAttributes) Get(key string) string {
	attr, found := c.input.MessageAttributes[key]
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
	if c.input.MessageAttributes == nil {
		c.input.MessageAttributes = map[string]types.MessageAttributeValue{}
	}
	c.input.MessageAttributes[key] = types.MessageAttributeValue{
		DataType:    aws.String(stringType),
		StringValue: aws.String(value),
	}
}

// Keys lists the keys in the carrier.
func (c *SnsCarrierAttributes) Keys() []string {
	if c.input.MessageAttributes == nil {
		return nil
	}
	keys := make([]string, 0, len(c.input.MessageAttributes))
	for k := range c.input.MessageAttributes {
		keys = append(keys, k)
	}
	return keys
}
