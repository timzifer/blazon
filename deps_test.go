package blazon_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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

// TestNoHeavyStandardLibraryImports keeps the binary small on purpose.
//
// A program whose whole job is to draw one small mark should not carry a
// general-purpose compositor, a general-purpose compressor, or the FIPS-140
// module. Each of these was measured and replaced by code in this repository
// that produces the same bytes: together they are about half a megabyte of a
// two-megabyte binary. The rule is here because the imports come back by
// accident — one fmt.Errorf on an image type, one hash.Hash — and nothing else
// would notice.
func TestNoHeavyStandardLibraryImports(t *testing.T) {
	banned := map[string]string{
		"image/png":      "use internal/pngenc",
		"image/draw":     "fill the pixels directly; see canvas.fillRect",
		"compress/zlib":  "use internal/pngenc",
		"compress/flate": "use internal/pngenc",
		"crypto/sha256":  "use internal/sha256",
		"hash/crc32":     "use internal/pngenc",
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			// Test support and the probe programs are not shipped, and the
			// replacements are checked against the standard library they
			// replace, which means importing it.
			if name := d.Name(); name == "testdata" || name == "probe" || name == ".git" {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if why, ok := banned[p]; ok {
				t.Errorf("%s imports %s: %s", path, p, why)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
