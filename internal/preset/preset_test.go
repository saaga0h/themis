package preset

import (
	"reflect"
	"strings"
	"testing"
)

func TestGetGo(t *testing.T) {
	p, ok := Get("go")
	if !ok {
		t.Fatal("Get(\"go\") ok = false, want true")
	}
	wantVerify := []string{
		"go build ./...",
		`test -z "$(gofmt -l .)"`,
		"go vet ./...",
		"go test ./...",
	}
	if !reflect.DeepEqual(p.Verify, wantVerify) {
		t.Errorf("go Verify = %q, want %q", p.Verify, wantVerify)
	}
	for _, sub := range []string{"go.dev/dl/go", "dpkg --print-architecture"} {
		if !strings.Contains(p.Toolchain, sub) {
			t.Errorf("go Toolchain missing %q:\n%s", sub, p.Toolchain)
		}
	}
}

func TestGetPython(t *testing.T) {
	p, ok := Get("python")
	if !ok {
		t.Fatal("Get(\"python\") ok = false, want true")
	}
	if want := []string{"ruff check .", "pytest"}; !reflect.DeepEqual(p.Verify, want) {
		t.Errorf("python Verify = %q, want %q", p.Verify, want)
	}
	for _, sub := range []string{"pip3 install", "pytest ruff", "--break-system-packages"} {
		if !strings.Contains(p.Toolchain, sub) {
			t.Errorf("python Toolchain missing %q:\n%s", sub, p.Toolchain)
		}
	}
}

func TestGetNode(t *testing.T) {
	p, ok := Get("node")
	if !ok {
		t.Fatal("Get(\"node\") ok = false, want true")
	}
	if want := []string{"npm ci", "npm run build", "npm test"}; !reflect.DeepEqual(p.Verify, want) {
		t.Errorf("node Verify = %q, want %q", p.Verify, want)
	}
	if !strings.Contains(p.Toolchain, "npm install -g typescript") {
		t.Errorf("node Toolchain missing typescript install:\n%s", p.Toolchain)
	}
	if p.Display != "JavaScript / TypeScript" {
		t.Errorf("node Display = %q, want %q", p.Display, "JavaScript / TypeScript")
	}
}

func TestGetRust(t *testing.T) {
	p, ok := Get("rust")
	if !ok {
		t.Fatal("Get(\"rust\") ok = false, want true")
	}
	if want := []string{"cargo build", "cargo test"}; !reflect.DeepEqual(p.Verify, want) {
		t.Errorf("rust Verify = %q, want %q", p.Verify, want)
	}
	if !strings.Contains(p.Toolchain, "cargo rustc") {
		t.Errorf("rust Toolchain missing %q:\n%s", "cargo rustc", p.Toolchain)
	}
}

func TestGetUnknownReturnsZero(t *testing.T) {
	for _, id := range []string{"other", "cobol", "", "GO"} {
		p, ok := Get(id)
		if ok {
			t.Errorf("Get(%q) ok = true, want false", id)
		}
		if !reflect.DeepEqual(p, Preset{}) {
			t.Errorf("Get(%q) = %+v, want zero Preset", id, p)
		}
	}
}

func TestLanguagesDerivedFromTable(t *testing.T) {
	got := Languages()
	want := []string{"go", "node", "python", "rust"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Languages() = %q, want %q", got, want)
	}
	// Derived-from-table, not a parallel list: every id round-trips through Get,
	// and the count matches the table so a new entry surfaces in both.
	if len(got) != len(table) {
		t.Errorf("len(Languages()) = %d, len(table) = %d", len(got), len(table))
	}
	for _, id := range got {
		if _, ok := Get(id); !ok {
			t.Errorf("Languages() lists %q but Get(%q) is not ok", id, id)
		}
	}
}

func TestEveryEntryComplete(t *testing.T) {
	for _, id := range Languages() {
		p, _ := Get(id)
		if p.Stack == "" {
			t.Errorf("%q: empty Stack", id)
		}
		if p.Display == "" {
			t.Errorf("%q: empty Display", id)
		}
		if len(p.Verify) == 0 {
			t.Errorf("%q: no Verify commands", id)
		}
		if strings.TrimSpace(p.Toolchain) == "" {
			t.Errorf("%q: empty Toolchain", id)
		}
	}
}

func TestGetReturnsCopyOfVerify(t *testing.T) {
	p1, _ := Get("go")
	p1.Verify[0] = "mutated"
	p2, _ := Get("go")
	if p2.Verify[0] == "mutated" {
		t.Error("Get returned a Verify slice aliasing the shared table")
	}
}
