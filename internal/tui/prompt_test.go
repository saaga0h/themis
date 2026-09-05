package tui

import (
	"bytes"
	"strings"
	"testing"
)

func TestInput_DefaultOnEmpty(t *testing.T) {
	p := New(strings.NewReader("\n"), &bytes.Buffer{})
	got, err := p.Input("Tag", "themis-x:latest")
	if err != nil {
		t.Fatalf("Input: %v", err)
	}
	if got != "themis-x:latest" {
		t.Errorf("empty input must return default; got %q", got)
	}
}

func TestInput_ReturnsTypedValue(t *testing.T) {
	p := New(strings.NewReader("themis-custom:latest\n"), &bytes.Buffer{})
	got, _ := p.Input("Tag", "themis-x:latest")
	if got != "themis-custom:latest" {
		t.Errorf("got %q, want themis-custom:latest", got)
	}
}

func TestSelect_ChoosesNumber(t *testing.T) {
	p := New(strings.NewReader("2\n"), &bytes.Buffer{})
	idx, err := p.Select("Pick", []string{"a", "b", "c"}, 0)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if idx != 1 {
		t.Errorf("input 2 must select index 1; got %d", idx)
	}
}

func TestSelect_EmptyTakesDefault(t *testing.T) {
	p := New(strings.NewReader("\n"), &bytes.Buffer{})
	idx, _ := p.Select("Pick", []string{"a", "b", "c"}, 2)
	if idx != 2 {
		t.Errorf("empty must take default index 2; got %d", idx)
	}
}

func TestSelect_RepromptsOnInvalid(t *testing.T) {
	out := &bytes.Buffer{}
	p := New(strings.NewReader("9\nx\n1\n"), out)
	idx, err := p.Select("Pick", []string{"a", "b"}, -1)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if idx != 0 {
		t.Errorf("expected eventual index 0; got %d", idx)
	}
	if !strings.Contains(out.String(), "please enter a number 1-2") {
		t.Errorf("expected a reprompt message; got:\n%s", out.String())
	}
}

func TestSelect_ExhaustedInputErrors(t *testing.T) {
	// No default and only invalid input then EOF: must error, not loop forever.
	p := New(strings.NewReader("9\n"), &bytes.Buffer{})
	if _, err := p.Select("Pick", []string{"a", "b"}, -1); err == nil {
		t.Error("expected an error when input is exhausted without a valid choice")
	}
}
