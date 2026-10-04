package apioutcome

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Event is the safe completion data shared by the HTTP and Gin adapters.
type Event struct {
	Method           string
	Status           int
	Code             string
	ErrorAfterCommit bool
	Duration         time.Duration
}

// Report writes one safe slog record and annotates the active OTel span.
// It never logs request paths, queries, bodies, or private error causes.
func Report(ctx context.Context, logger *slog.Logger, event Event) {
	if logger == nil {
		logger = slog.Default()
	}
	level := slog.LevelInfo
	if event.Status >= http.StatusInternalServerError || event.ErrorAfterCommit {
		level = slog.LevelError
	} else if event.Status >= http.StatusBadRequest {
		level = slog.LevelWarn
	}
	attrs := []slog.Attr{
		slog.String("method", event.Method),
		slog.Int("status", event.Status),
		slog.String("code", event.Code),
		slog.Duration("duration", event.Duration),
		slog.Bool("error_after_commit", event.ErrorAfterCommit),
	}
	span := trace.SpanFromContext(ctx)
	if spanCtx := span.SpanContext(); spanCtx.IsValid() {
		attrs = append(attrs, slog.String("trace_id", spanCtx.TraceID().String()))
	}
	logger.LogAttrs(ctx, level, "http request completed", attrs...)
	span.SetAttributes(
		attribute.Int("http.response.status_code", event.Status),
		attribute.String("apioutcome.code", event.Code),
		attribute.Bool("apioutcome.error_after_commit", event.ErrorAfterCommit),
	)
	if level == slog.LevelError {
		span.RecordError(errors.New(event.Code))
		span.SetStatus(codes.Error, event.Code)
	}
}
