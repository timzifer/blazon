package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timzifer/blazon"
	"github.com/timzifer/blazon/blazontest"
)

func TestRunRendersToFiles(t *testing.T) {
	dir := t.TempDir()

	svgPath := filepath.Join(dir, "mark.svg")
	if err := run([]string{"-r", "truchet", "-size", "64", "-o", svgPath, "1.4.2"}); err != nil {
		t.Fatalf("svg: %v", err)
	}
	data, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("<svg")) {
		t.Error("output is not an SVG document")
	}

	pngPath := filepath.Join(dir, "mark.png")
	if err := run([]string{"-r", "polar", "-size", "64", "-o", pngPath, "1.4.2"}); err != nil {
		t.Fatalf("png: %v", err)
	}
	raw, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("png does not decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 64 {
		t.Errorf("png is %v, want 64 wide", b)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	cases := [][]string{
		{"1.0.0", "2.0.0"},                  // two versions
		{"-r", "nope", "1.0.0"},             // unknown renderer
		{"-policy", "nope", "1.0.0"},        // unknown policy
		{"-palette", "nope", "1.0.0"},       // unknown palette
		{"-bg", "nope", "1.0.0"},            // unknown background
		{"-o", "mark.txt", "1.0.0"},         // unknown extension
		{"dist", "-r", "no-such-renderer"},  // unknown renderer for dist
		{"sheet", "-r", "no-such-renderer"}, // unknown renderer for sheet
	}
	for _, args := range cases {
		if err := run(args); err == nil {
			t.Errorf("run(%v) succeeded, want an error", args)
		}
	}
}

func TestCmdSheetWritesAContactSheet(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sheet.png")
	err := run([]string{"sheet", "-r", "truchet", "-n", "8", "-grid", "4", "-cell", "32", "-o", out})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 4*32 {
		t.Errorf("sheet is %v, want 4 columns of 32 pixels", b)
	}
}

// TestGalleryIsIdempotent is the property the CI job depends on: a second run
// over unchanged code must produce no diff, or the gallery workflow would
// commit on every push.
func TestGalleryIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	imgDir := filepath.Join(dir, "gallery")
	readme := filepath.Join(dir, "README.md")

	const skeleton = "# Title\n\nintro\n\n" + galleryStartMarker + "\n" + galleryEndMarker + "\n\ntail\n"
	if err := os.WriteFile(readme, []byte(skeleton), 0o644); err != nil {
		t.Fatal(err)
	}

	corpus := filepath.Join(dir, "corpus.txt")
	if err := os.WriteFile(corpus, []byte("1.0.0\n1.0.1\n2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{"gallery", "-o", imgDir, "-readme", readme, "-corpus", corpus, "-cell", "32", "-grid", "3"}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	first := snapshot(t, dir)

	if err := run(args); err != nil {
		t.Fatal(err)
	}
	second := snapshot(t, dir)

	if len(first) != len(second) {
		t.Fatalf("second run changed the file set: %d then %d files", len(first), len(second))
	}
	for name, sum := range first {
		if second[name] != sum {
			t.Errorf("%s changed on the second run", name)
		}
	}

	text, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "intro") || !strings.Contains(string(text), "tail") {
		t.Error("the gallery rewrite clobbered content outside the markers")
	}
	for _, name := range blazon.Renderers() {
		if !strings.Contains(string(text), name+"-sheet.png") {
			t.Errorf("README does not reference the %s sheet", name)
		}
	}
}

func TestGalleryNeedsMarkers(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# no markers here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	corpus := filepath.Join(dir, "corpus.txt")
	if err := os.WriteFile(corpus, []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"gallery", "-o", filepath.Join(dir, "g"), "-readme", readme, "-corpus", corpus, "-cell", "16"})
	if err == nil {
		t.Fatal("a README without markers was accepted")
	}
}

// snapshot records the size of every file under dir, keyed by relative path.
// Sizes are enough to catch a regenerated image that differs.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReportMentionsEveryRelation(t *testing.T) {
	r, _ := blazon.Lookup("truchet")
	rep, err := blazontest.Suite{Renderer: r, Corpus: []string{
		"1.0.0", "1.0.1", "1.1.0", "2.0.0", "1.0.0-rc.1",
	}}.Report(nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	printReport(&buf, rep)
	out := buf.String()
	for _, want := range []string{"patch", "minor", "major", "prerelease", "histogram", "suggested"} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not mention %q:\n%s", want, out)
		}
	}
}
