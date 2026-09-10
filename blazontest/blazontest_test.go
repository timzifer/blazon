package blazontest_test

import (
	"fmt"
	"image/color"
	"runtime"
	"strings"
	"testing"

	"github.com/timzifer/blazon"
	"github.com/timzifer/blazon/blazontest"
)

// recorder is a minimal testing.TB that captures failures instead of
// reporting them, so the suite can be pointed at a deliberately broken
// renderer and checked for actually failing. A measurement harness that
// silently passes everything is worse than none.
type recorder struct {
	testing.TB
	msgs   []string
	failed bool
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failed = true
	r.msgs = append(r.msgs, sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func (r *recorder) Skip(args ...any) { runtime.Goexit() }

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }

// capture runs fn on a fresh recorder, tolerating a Fatalf that ends the
// goroutine.
func capture(fn func(tb testing.TB)) *recorder {
	r := &recorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(r)
	}()
	<-done
	return r
}

// constantRenderer draws the same mark for every version.
type constantRenderer struct{}

func (constantRenderer) Name() string { return "test-constant" }
func (constantRenderer) Caps() blazon.Caps {
	return blazon.Caps{}
}
func (constantRenderer) Render(ctx *blazon.Context) error {
	b := ctx.Box
	ctx.Canvas.MoveTo(b.X, b.Y)
	ctx.Canvas.LineTo(b.X+b.W, b.Y+b.H)
	ctx.Canvas.Stroke(color.Black, 4)
	return nil
}

// nondeterministicRenderer changes what it draws between calls.
type nondeterministicRenderer struct{ n *int }

func (nondeterministicRenderer) Name() string      { return "test-nondeterministic" }
func (nondeterministicRenderer) Caps() blazon.Caps { return blazon.Caps{} }
func (r nondeterministicRenderer) Render(ctx *blazon.Context) error {
	*r.n++
	b := ctx.Box
	ctx.Canvas.MoveTo(b.X, b.Y)
	ctx.Canvas.LineTo(b.X+b.W*float64(*r.n%7)/7, b.Y+b.H)
	ctx.Canvas.Stroke(color.Black, 4)
	return nil
}

var smallCorpus = []string{"1.0.0", "1.0.1", "1.1.0", "2.0.0", "1.0.0-rc.1"}

func TestSuiteCatchesIdenticalMarks(t *testing.T) {
	s := blazontest.Suite{Renderer: constantRenderer{}, Corpus: smallCorpus}
	r := capture(func(tb testing.TB) { s.CheckUniqueness(tb, smallCorpus) })
	if !r.failed {
		t.Error("a renderer that draws the same mark for every version passed the uniqueness check")
	}
}

func TestSuiteCatchesNondeterminism(t *testing.T) {
	n := 0
	s := blazontest.Suite{Renderer: nondeterministicRenderer{n: &n}, Corpus: smallCorpus}
	r := capture(func(tb testing.TB) { s.CheckDeterminism(tb, smallCorpus) })
	if !r.failed {
		t.Error("a renderer that changes between calls passed the determinism check")
	}
}

func TestSuiteCatchesUnreachableThresholds(t *testing.T) {
	real, _ := blazon.Lookup("truchet")
	s := blazontest.Suite{
		Renderer: real,
		Thresholds: blazontest.Thresholds{
			MinPatch: 120, MinMinor: 120, MinMajor: 120, Floor: 120,
		},
	}
	r := capture(func(tb testing.TB) { s.CheckDistinctness(tb, smallCorpus) })
	if !r.failed {
		t.Error("thresholds no renderer could meet were reported as met")
	}
}

func TestSuitePassesForARealRenderer(t *testing.T) {
	real, _ := blazon.Lookup("flowfield")
	s := blazontest.Suite{
		Renderer: real,
		Thresholds: blazontest.Thresholds{
			MinPatch: 4, MinMinor: 4, MinMajor: 4, Floor: 2, PrereleaseRatio: 0.5,
		},
	}
	r := capture(func(tb testing.TB) { s.CheckDistinctness(tb, smallCorpus) })
	if r.failed {
		t.Errorf("a real renderer failed lenient thresholds: %v", r.msgs)
	}
}

func TestDefaultCorpusIsUsable(t *testing.T) {
	corpus, err := blazontest.DefaultCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus) < 100 {
		t.Errorf("the built-in corpus has only %d versions", len(corpus))
	}
	seen := map[string]bool{}
	for _, v := range corpus {
		parsed, err := blazon.ParseVersion(v)
		if err != nil {
			t.Errorf("corpus entry %q does not parse: %v", v, err)
			continue
		}
		if seen[parsed.String()] {
			t.Errorf("corpus contains %q twice", parsed.String())
		}
		seen[parsed.String()] = true
	}
}

func TestReportRelationsAreDisjoint(t *testing.T) {
	r, _ := blazon.Lookup("truchet")
	rep, err := blazontest.Suite{Renderer: r, Corpus: smallCorpus}.Report(smallCorpus)
	if err != nil {
		t.Fatal(err)
	}
	// Ten pairs in all. The one pair sharing a core version — 1.0.0 and its
	// release candidate — is counted as a prerelease pair; the other nine
	// compare different releases and go to All.
	if rep.All.Count != 9 {
		t.Errorf("All covers %d pairs, want 9", rep.All.Count)
	}
	if rep.Prerelease.Count != 1 {
		t.Errorf("Prerelease covers %d pairs, want 1", rep.Prerelease.Count)
	}
	if rep.Patch.Count != 1 {
		t.Errorf("Patch covers %d pairs, want 1 (1.0.0~1.0.1)", rep.Patch.Count)
	}
	if rep.Versions != len(smallCorpus) {
		t.Errorf("Versions = %d, want %d", rep.Versions, len(smallCorpus))
	}
}

func TestReportRejectsADuplicateCorpus(t *testing.T) {
	r, _ := blazon.Lookup("truchet")
	_, err := blazontest.Suite{Renderer: r}.Report([]string{"1.0.0", "v1.0.0"})
	if err == nil {
		t.Fatal("a corpus containing the same version twice was accepted")
	}
}

func TestContactSheetDimensions(t *testing.T) {
	r, _ := blazon.Lookup("polar")
	img, err := blazontest.ContactSheet(r, smallCorpus, blazon.Options{}, 3, 32)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 3*32 {
		t.Errorf("sheet width %d, want %d", b.Dx(), 3*32)
	}
	// Five marks in rows of three is two rows, each a cell plus a label strip.
	if b.Dy() <= 2*32 {
		t.Errorf("sheet height %d leaves no room for labels", b.Dy())
	}
}

func TestStatsHelpers(t *testing.T) {
	r, _ := blazon.Lookup("truchet")
	rep, err := blazontest.Suite{Renderer: r, Corpus: smallCorpus}.Report(smallCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if got := rep.All.Below(0); got != 0 {
		t.Errorf("Below(0) = %d, want 0", got)
	}
	if got := rep.All.Below(129); got != rep.All.Count {
		t.Errorf("Below(129) = %d, want every pair (%d)", got, rep.All.Count)
	}
	if s := rep.All.WorstString(3); !strings.Contains(s, "~") {
		t.Errorf("WorstString did not name any pairs: %q", s)
	}
	if s := (blazontest.Stats{}).WorstString(3); s != "" {
		t.Errorf("WorstString of empty stats = %q", s)
	}
}
