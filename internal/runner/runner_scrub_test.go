package runner

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/saaga0h/themis/internal/scrub"
)

// A block comment must be scrubbed before it is posted to the tracker — a
// git/transport error routed through blockIssue can carry a credential. #91.
func TestBlockIssue_ScrubsComment(t *testing.T) {
	w := &stubIssueWriter{}
	cfg := Config{
		IssueNumber: 42,
		IssueWriter: w,
		Scrub:       scrub.New("s3cr3t-token"),
	}
	err := blockIssue(context.Background(), cfg, errors.New("push failed: token s3cr3t-token rejected"))
	if err != nil {
		t.Fatalf("blockIssue: %v", err)
	}
	if len(w.comments) == 0 {
		t.Fatal("expected a block comment to be posted")
	}
	if strings.Contains(w.comments[0], "s3cr3t-token") {
		t.Errorf("block comment leaked the secret: %q", w.comments[0])
	}
}

// A StepRecord's free-text detail must be scrubbed before it reaches the sink.
func TestEmitStep_ScrubsDetail(t *testing.T) {
	em := &captureEmitter{}
	cfg := Config{Emitter: em, Scrub: scrub.New("s3cr3t-token")}
	emitStep(context.Background(), cfg, io.Discard, StepRecord{
		Stage:  "Ship",
		Detail: "git push failed: remote rejected token s3cr3t-token",
	})
	if len(em.records) == 0 {
		t.Fatal("expected a record to be emitted")
	}
	if strings.Contains(em.records[0].Detail, "s3cr3t-token") {
		t.Errorf("emitted detail leaked the secret: %q", em.records[0].Detail)
	}
}
