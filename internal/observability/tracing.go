package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func InitTracing(serviceName string) {
	tracer = otel.Tracer(serviceName)
}

func StartSpan(ctx context.Context, sourceName string, query string) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, nil
	}
	return tracer.Start(ctx, sourceName+".search",
		trace.WithAttributes(
			attribute.String("source", sourceName),
			attribute.String("query", query),
		))
}

func EndSpan(span trace.Span, resultsCount int, err error) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.Int("results_count", resultsCount))
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}
