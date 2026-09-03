package git

import (
	"context"
	"testing"
)

func TestIdentityError(t *testing.T) {
	cases := []struct {
		name, email string
		wantErr     bool
	}{
		{"Ada", "[email protected]", false},
		{"Ada\n", "[email protected]\n", false}, // git output carries a trailing newline
		{"", "[email protected]", true},
		{"Ada", "", true},
		{"", "", true},
		{"   ", "  ", true}, // whitespace-only is not an identity
	}
	for _, c := range cases {
		err := identityError(c.name, c.email)
		if (err != nil) != c.wantErr {
			t.Errorf("identityError(%q, %q) error = %v, wantErr = %v", c.name, c.email, err, c.wantErr)
		}
	}
}

// CheckIdentity reads the repo's resolved identity: with a local identity set it
// passes regardless of the machine's global git config.
func TestCheckIdentity_PassesWhenLocalIdentitySet(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.name", "Test User"},
		{"config", "user.email", "[email protected]"},
	} {
		if _, err := runGit(ctx, dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	if err := CheckIdentity(ctx, dir); err != nil {
		t.Errorf("CheckIdentity with local identity set = %v, want nil", err)
	}
}
