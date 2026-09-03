package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/agent"
	"github.com/saaga0h/themis/internal/pipeline"
)

// captureEmitter records every StepRecord it receives so tests can assert the
// factory's diagnostic narrative.
type captureEmitter struct{ records []StepRecord }

func (c *captureEmitter) Emit(_ context.Context, r StepRecord) { c.records = append(c.records, r) }

type panicEmitter struct{}

func (panicEmitter) Emit(context.Context, StepRecord) { panic("boom") }

type failingInvoker struct{ err error }

func (f *failingInvoker) Invoke(context.Context, agent.InvokeOptions) (*agent.InvokeResult, error) {
	return nil, f.err
}

func TestEmitStep_NilEmitterIsNoOp(t *testing.T) {
	// A nil emitter must be a silent no-op, never a panic.
	emitStep(context.Background(), Config{}, io.Discard, StepRecord{Stage: "X"})
}

func TestEmitStep_RecoversAndLogsPanic(t *testing.T) {
	// A panicking emitter must be recovered and logged — telemetry must never
	// abort a run (addresses the run-1 silent-panic-swallow finding).
	var buf bytes.Buffer
	emitStep(context.Background(), Config{Emitter: panicEmitter{}}, &buf, StepRecord{Stage: "X"})
	if !strings.Contains(buf.String(), "panicked") {
		t.Errorf("expected the panic recovered and logged; got %q", buf.String())
	}
}

func TestRunID_Format(t *testing.T) {
	if got := runID(42, 1000); got != "42-1000" {
		t.Errorf("runID = %q, want 42-1000", got)
	}
}

func emitterTemplateDir(t *testing.T) string {
	return makeTemplateDir(t, map[string]string{
		"implement.md":   "Implement {{ISSUE_NUMBER}}",
		"review.md":      "Review {{ISSUE_NUMBER}}",
		"update-docs.md": "Docs {{ISSUE_NUMBER}}",
		"ship.md":        "Ship {{ISSUE_NUMBER}}",
	})
}

// A full run emits per-step "ok" records and a "shipped" record, each tagged
// with the issue number and a stable run id.
func TestRunner_EmitsStepAndShipRecords(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)
	em := &captureEmitter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &recordingInvoker{},
		IssueWriter:  &stubIssueWriter{prURL: "https://example.com/pr/88"},
		TemplateDir:  emitterTemplateDir(t),
		CheckpointFn: noopCheckpoint,
		Emitter:      em,
	}
	if _, err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var sawOK, sawShipped bool
	var rid string
	for _, r := range em.records {
		if r.IssueNumber != 42 || r.RunID == "" {
			t.Errorf("record missing issue_number/run_id: %+v", r)
		}
		if rid == "" {
			rid = r.RunID
		} else if r.RunID != rid {
			t.Errorf("run_id not stable across records: %q vs %q", rid, r.RunID)
		}
		switch r.Outcome {
		case "ok":
			sawOK = true
		case "shipped":
			sawShipped = true
			if r.PRURL == "" {
				t.Error("shipped record missing PRURL")
			}
			if r.Verdict != "ready" && r.Verdict != "draft" {
				t.Errorf("shipped record verdict = %q, want ready|draft", r.Verdict)
			}
		}
	}
	if !sawOK {
		t.Error("expected at least one ok step record")
	}
	if !sawShipped {
		t.Error("expected a shipped record")
	}
}

// When an agent invocation fails, an "error" record is emitted carrying the
// failure detail (the stderr surfaced by internal/agent) — so a step crash is
// diagnosable from the sink, not just stdout.
func TestRunner_EmitsErrorRecordOnInvokeFailure(t *testing.T) {
	workDir := t.TempDir()
	saveStateAt(t, workDir, pipeline.StepImplement)
	em := &captureEmitter{}
	cfg := Config{
		WorkDir:      workDir,
		IssueNumber:  42,
		Fetcher:      &stubFetcher{issue: sampleIssue()},
		Invoker:      &failingInvoker{err: errors.New("claude exited with error: exit status 1\nstderr (last lines):\nboom-cause")},
		IssueWriter:  &stubIssueWriter{},
		TemplateDir:  emitterTemplateDir(t),
		CheckpointFn: noopCheckpoint,
		Emitter:      em,
	}
	if _, err := Run(context.Background(), cfg); err == nil {
		t.Fatal("expected Run to return the invocation error")
	}
	var errRec *StepRecord
	for i := range em.records {
		if em.records[i].Outcome == "error" {
			errRec = &em.records[i]
		}
	}
	if errRec == nil {
		t.Fatal("expected an error record to be emitted")
	}
	if !strings.Contains(errRec.Detail, "boom-cause") {
		t.Errorf("error record must carry the failure cause; got %q", errRec.Detail)
	}
	if errRec.Stage == "" || errRec.RunID == "" {
		t.Error("error record missing stage/run_id")
	}
}
