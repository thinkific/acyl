package env

import (
	"context"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const spanContextKey = "span_key"

type acylContextKey string

// NewSpanContext returns a context with the span embedded as a value
func NewSpanContext(ctx context.Context, span tracer.Span) context.Context {
	return context.WithValue(ctx, acylContextKey(spanContextKey), span)
}

// starts and returns a child span if a parent span is found in the context.
// starts and returns a root level span otherwise.
func startChildSpan(ctx context.Context, operationName string) tracer.Span {
	parentSpan, ok := ctx.Value(acylContextKey(spanContextKey)).(tracer.Span)
	if !ok {
		return tracer.StartSpan(operationName)
	}
	return tracer.StartSpan(operationName, tracer.ChildOf(parentSpan.Context()))
}
