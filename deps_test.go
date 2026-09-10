package blazon_test

import (
	"os"
	"strings"
	"testing"
)

// TestNoDependencies makes the zero-dependency claim a build rule.
//
// It is not asceticism. This library's premise is that a version always
// produces the same mark, and the two things it would otherwise import — a
// rasteriser and a font — are exactly the two that decide what its pixels are.
// A dependency that sharpened its anti-aliasing or adjusted a glyph would
// change every mark ever generated, and the change would arrive as a routine
// version bump. Owning both means the marks can only move when this repository
// decides they move.
func TestNoDependencies(t *testing.T) {
	// A go.sum only exists to pin dependency hashes. With none to pin, its
	// presence means either a stale file or a dependency that slipped in
	// without go.mod being tidied.
	if data, err := os.ReadFile("go.sum"); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		t.Errorf("go.sum is not empty, so something is being depended on:\n%s", data)
	}

	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "//"):
		case strings.HasPrefix(line, "module "),
			strings.HasPrefix(line, "go "),
			strings.HasPrefix(line, "toolchain "):
		default:
			t.Errorf("go.mod is no longer dependency-free: %q\n"+
				"If this is deliberate, say in the pull request what the dependency "+
				"can change about the rendered marks, and how a bump would be caught.",
				line)
		}
	}
}
