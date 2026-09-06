package runner

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"
)

// StepRecord is one structured diagnostic record about a pipeline step (or the
// ship/failure of a run). The runner emits these through Config.Emitter so the
// factory's OWN narrative — per-step outcome, the green-gate verify output,
// block reasons, the agent-failure detail, the ship verdict — is queryable
// alongside Claude Code's telemetry, instead of living only in stdout. This is
// what makes a run diagnosable from a separate session.
type StepRecord struct {
	IssueNumber       int
	RunID             string // stable per run (issue + start time); shared by all of a run's records
	Stage             string // pipeline step name (e.g. "Implement"), or "Ship"
	Outcome           string // "ok" | "error" | "blocked" | "shipped"
	Completed         bool   // the agent emitted its completion marker (vs hit the turn limit)
	Commits           int
	DurationMs        int64
	GreenGate         string // "pass" | "fail" | "" (not applicable to this step)
	VerifyOutput      string // bounded tail of the verify output on a green-gate failure
	ReviewBlocking    int
	ReviewNonBlocking int
	Verdict           string // ship only: "ready" | "draft"
	PRURL             string // ship only
	Detail            string // bounded error/block detail (includes the agent stderr surfaced by internal/agent)
}

// Emitter receives StepRecords and ships them to an external sink (e.g. an OTLP
// collector → Loki). A nil Config.Emitter means no emission. Implementations
// must be best-effort: an Emit must never block a run for long or panic it.
type Emitter interface {
	Emit(ctx context.Context, rec StepRecord)
}

// emitStep sends rec to cfg.Emitter if one is configured. Best-effort: a nil
// emitter is a no-op, and a panicking emitter is recovered and logged (not
// allowed to abort the run) — telemetry must never fail the pipeline.
func emitStep(ctx context.Context, cfg Config, log io.Writer, rec StepRecord) {
	if cfg.Emitter == nil {
		return
	}
	// Scrub the free-text fields before they leave the process (the detail can
	// carry a git/transport error with an embedded credential or host).
	if cfg.Scrub != nil {
		rec.Detail = cfg.Scrub(rec.Detail)
		rec.VerifyOutput = cfg.Scrub(rec.VerifyOutput)
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(log, "warning: diagnostic emitter panicked (record dropped): %v\n", r)
		}
	}()
	cfg.Emitter.Emit(ctx, rec)
}

// emitStepFailure emits an "error" StepRecord for a step that fails on an early
// exit — a checkpoint failure, or one of Ship's base-branch/no-commits/push/
// PR-create guards — then returns err unchanged so a caller can write
// `return nil, emitStepFailure(...)`. Without it the factory emits a record only
// when a step *completes*, so exactly the failures a diagnosis needs are invisible
// in the sink: a Ship push error looked like a "silent stop" to the
// diagnose-themis-run skill, which then misattributed it to Themis. The err's
// bounded tail becomes Detail (scrubbed by emitStep before it leaves the process).
func emitStepFailure(ctx context.Context, cfg Config, log io.Writer, rid, stage string, stepStart time.Time, err error) error {
	emitStep(ctx, cfg, log, StepRecord{
		IssueNumber: cfg.IssueNumber,
		RunID:       rid,
		Stage:       stage,
		Outcome:     "error",
		DurationMs:  time.Since(stepStart).Milliseconds(),
		Detail:      lastLines(err.Error(), 30),
	})
	return err
}

// runID is a stable identifier for one factory run: issue number + the run's
// start time (Unix seconds). It survives resume (StartedAt is persisted in
// state), so every record for a run shares the same id.
func runID(issueNumber int, startedAtUnix int64) string {
	return strconv.Itoa(issueNumber) + "-" + strconv.FormatInt(startedAtUnix, 10)
}
