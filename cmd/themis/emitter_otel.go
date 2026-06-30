package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/saaga0h/themis/internal/runner"
)

// otelEmitter ships the factory's per-step StepRecords as OTLP-protobuf log
// records via the otel log SDK — onto the same OTLP→Vector→Loki path Claude
// Code's own telemetry uses, so the factory's narrative is queryable alongside it.
type otelEmitter struct {
	logger otellog.Logger
}

func (e *otelEmitter) Emit(ctx context.Context, r runner.StepRecord) {
	var rec otellog.Record
	rec.SetTimestamp(time.Now())
	rec.SetBody(otellog.StringValue(fmt.Sprintf("factory %s: %s", r.Stage, r.Outcome)))
	attrs := []otellog.KeyValue{
		otellog.Int("issue_number", r.IssueNumber),
		otellog.String("run_id", r.RunID),
		otellog.String("stage", r.Stage),
		otellog.String("outcome", r.Outcome),
		otellog.Bool("completed", r.Completed),
		otellog.Int("commits", r.Commits),
		otellog.Int64("duration_ms", r.DurationMs),
	}
	if r.GreenGate != "" {
		attrs = append(attrs, otellog.String("green_gate", r.GreenGate))
	}
	if r.VerifyOutput != "" {
		attrs = append(attrs, otellog.String("verify_output", r.VerifyOutput))
	}
	if r.ReviewBlocking != 0 || r.ReviewNonBlocking != 0 {
		attrs = append(attrs,
			otellog.Int("review_blocking", r.ReviewBlocking),
			otellog.Int("review_non_blocking", r.ReviewNonBlocking),
		)
	}
	if r.Verdict != "" {
		attrs = append(attrs, otellog.String("verdict", r.Verdict))
	}
	if r.PRURL != "" {
		attrs = append(attrs, otellog.String("pr_url", r.PRURL))
	}
	if r.Detail != "" {
		attrs = append(attrs, otellog.String("detail", r.Detail))
	}
	rec.AddAttributes(attrs...)
	e.logger.Emit(ctx, rec)
}

// newOTELEmitter builds an OTLP-protobuf log emitter when an OTLP endpoint is
// configured (OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_LOGS_ENDPOINT) —
// the same env-gating Claude Code's telemetry uses. Returns (nil, nil, false)
// when no endpoint env is set, so the runner emits nothing. Best-effort: a
// failed exporter init disables emission rather than failing the run. The
// returned shutdown flushes the async batch processor; Run defers it (bounded).
func newOTELEmitter(ctx context.Context) (runner.Emitter, func(context.Context) error, bool) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT") == "" {
		return nil, nil, false
	}
	exporter, err := otlploghttp.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: OTLP log exporter init failed; factory telemetry disabled: %v\n", err)
		return nil, nil, false
	}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)))
	return &otelEmitter{logger: provider.Logger("themis/factory")}, provider.Shutdown, true
}
