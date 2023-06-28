// Package config loads configuration from env vars.
package config

import "github.com/udhos/opentelemetry-trace-sqs/internal/env"

// AppConfig holds application configuration.
type AppConfig struct {
	HTTPAddr               string
	HTTPRoute              string
	JaegerURL              string
	QueueURLInput          string
	QueueURLOutput         string
	QueueRoleARNInput      string
	QueueRoleARNOutput     string
	QueueTraceIDAttrInput  string
	QueueTraceIDAttrOutput string
	BackendURL             string
	EndpointURL            string
}

// New loads configuration from env vars.
func New() AppConfig {
	return AppConfig{
		HTTPAddr:               env.String("HTTP_ADDR", ":8001"),
		HTTPRoute:              env.String("HTTP_ROUTE", "/send"),
		JaegerURL:              env.String("JAEGER_URL", "http://jaeger-collector:14268/api/traces"),
		QueueURLInput:          env.String("QUEUE_URL_INPUT", ""),
		QueueURLOutput:         env.String("QUEUE_URL_OUTPUT", ""),
		QueueRoleARNInput:      env.String("QUEUE_ROLE_ARN_INPUT", ""),
		QueueRoleARNOutput:     env.String("QUEUE_ROLE_ARN_OUTPUT", ""),
		QueueTraceIDAttrInput:  env.String("QUEUE_TRACE_ID_ATTR_INPUT", "traceId"),
		QueueTraceIDAttrOutput: env.String("QUEUE_TRACE_ID_ATTR_OUTPUT", "traceId"),
		BackendURL:             env.String("BACKEND_URL", "http://localhost:8002/send"),
		EndpointURL:            env.String("ENDPOINT_URL", ""),
	}
}
