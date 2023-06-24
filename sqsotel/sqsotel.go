// Package sqsotel implements carriers for SQS.
package sqsotel

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/contrib/propagators/b3"
)

var sqsPropagator = b3.New() // b3 single header

// ContextFromSqsMessageAttributes gets a tracing context from SQS message attributes.
func ContextFromSqsMessageAttributes(sqsMessage *types.Message) context.Context {
	ctxOrig := context.Background()
	carrier := NewCarrierAttributes(sqsMessage)
	ctx := sqsPropagator.Extract(ctxOrig, carrier)
	return ctx
}

// InjectIntoSqsMessageAttributes inserts tracing from context into the SQS message attributes.
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
