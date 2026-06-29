package labels_test

import (
	"testing"

	"github.com/saaga0h/themis/internal/labels"
)

func TestReadyForAgentValue(t *testing.T) {
	if labels.ReadyForAgent != "ready-for-agent" {
		t.Errorf("ReadyForAgent = %q; want %q", labels.ReadyForAgent, "ready-for-agent")
	}
}

func TestBlockedValue(t *testing.T) {
	if labels.Blocked != "blocked" {
		t.Errorf("Blocked = %q; want %q", labels.Blocked, "blocked")
	}
}
