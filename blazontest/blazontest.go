// Package blazontest measures whether a renderer actually distinguishes
// versions, and fails the build when it does not.
//
// Every identicon library claims its marks are easy to tell apart. Here the
// claim is a test: marks are rendered across a version corpus, reduced to a
// perceptual hash, and the Hamming distances between related versions are held
// to per-renderer thresholds. A renderer whose patch bumps stop being visible
// turns the build red, in this repository and in any project that runs Suite
// against a renderer of its own.
package blazontest

import (
	"bufio"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/timzifer/blazon"
	"github.com/timzifer/blazon/canvas"
	"github.com/timzifer/blazon/internal/phash"
)

// The Check functions take a testing.TB rather than a *testing.T so that a
// caller can drive them from a benchmark, from a fuzz target, or from a
// recorder that verifies the suite itself fails when it should.

// Thresholds are the minimum perceptual distances a renderer must keep, in
// bits of the 128-bit combined hash. They are calibrated per renderer from its own
// distance histogram and then frozen as a regression bound — see the dist
// subcommand of cmd/blazon, which prints exactly the numbers these are set
// from.
type Thresholds struct {
	// MinPatch is the floor for two versions differing only in patch. This is
	// the demanding one: it is what the amplified policy exists to satisfy.
	MinPatch int
	// MinMinor is the floor for two versions differing only in minor.
	MinMinor int
	// MinMajor is the floor for two versions differing only in major. It
	// should be at least three times MinPatch, so that the visual jump at a
	// major release is unmistakable.
	MinMajor int
	// Floor is the minimum over every pair of distinct core versions in the
	// corpus. Prerelease pairs are excluded and bounded from above by
	// MaxPrerelease instead, because they are meant to look alike.
	Floor int
	// PrereleaseRatio bounds how far a prerelease may drift from its release,
	// as a fraction of the mean patch-neighbour distance. It runs the other
	// way from the minima above: 1.0.0-rc.1 must still read as a 1.0.0, so its
	// mean distance has to stay well below that of a real patch bump. Zero
	// disables the check.
	//
	// It is stated as a mean ratio rather than as an absolute maximum because
	// a perceptual hash thresholds DCT coefficients at their median, and a
	// highly symmetric mark has many coefficients sitting near that median.
	// A single stray pair can then differ by a surprising number of bits
	// without looking any different; the mean is the honest statistic here.
	PrereleaseRatio float64
	// FailFraction allows a small share of pairs to fall below the relation
	// thresholds. A perceptual hash is a coarse instrument and a handful of
	// near-misses out of thousands of pairs is not a defect; set it to zero to
	// demand that every pair clears the bar.
	FailFraction float64
}

// Suite is a renderer plus the corpus and thresholds to judge it by.
type Suite struct {
	// Renderer is the renderer under test. It need not be registered.
	Renderer blazon.Renderer
	// Corpus is the list of versions. Empty means the corpus shipped with
	// this repository.
	Corpus []string
	// Thresholds are the bounds to enforce.
	Thresholds Thresholds
	// Options override the measurement defaults. Size and Background are
	// forced unless set, because the measurement must be comparable across
	// renderers.
	Options blazon.Options
}

// MeasureSize is the edge length marks are rendered at for measurement. It is
// deliberately small: the hash works on a 32×32 reduction anyway, and a small
// render keeps a full corpus sweep fast enough to run on every push.
const MeasureSize = 128

func (s Suite) options() blazon.Options {
	o := s.Options
	if o.Size == 0 {
		o.Size = MeasureSize
	}
	if o.Background == blazon.BackgroundTransparent {
		// Measuring on a transparent ground would compare marks against
		// nothing; the hash composites over white, so make that explicit.
		o.Background = blazon.BackgroundLight
	}
	o.Renderer = s.Renderer.Name()
	// Measurement is monochrome on purpose. With a luminance-based perceptual
	// hash, two marks differing only in hue score as identical, so a coloured
	// measurement would silently credit colour for distinctness that the form
	// does not have. Judging form alone makes the guarantee stronger and
	// honest: the marks stay distinguishable in print, in a monochrome
	// terminal, and to a colour-blind reader. Hue is then a bonus on top.
	o.Palette = blazon.PaletteMono
	return o
}

func (s Suite) corpus(tb testing.TB) []string {
	if len(s.Corpus) > 0 {
		return s.Corpus
	}
	c, err := DefaultCorpus()
	if err != nil {
		tb.Fatalf("blazontest: loading the default corpus: %v", err)
	}
	return c
}

// Run executes every check that applies to the renderer. Ordered renderers —
// those that encode the version legibly rather than hashing it — are held to
// injectivity instead of to the distance thresholds, because being systematic
// is the whole point of them.
func (s Suite) Run(t *testing.T) {
	t.Helper()
	corpus := s.corpus(t)

	t.Run("determinism", func(t *testing.T) { s.CheckDeterminism(t, corpus) })
	t.Run("uniqueness", func(t *testing.T) { s.CheckUniqueness(t, corpus) })

	if s.Renderer.Caps().Ordered {
		return
	}
	t.Run("distinctness", func(t *testing.T) { s.CheckDistinctness(t, corpus) })
	if s.options().Policy != blazon.PolicyIndependent {
		t.Run("family_cohesion", func(t *testing.T) { s.CheckFamilyCohesion(t, corpus) })
	}
}

// CheckDeterminism asserts that a version renders identically twice, with an
// unrelated render in between to catch state leaking through the renderer.
//
// Byte equality is only required of renderers that declare ByteExact.
// Everything else is compared by perceptual hash, because math.Sin and
// math.Pow are permitted to differ by an ulp between architectures and a
// byte-exact demand would make those renderers untestable across a CI matrix
// rather than making them more correct.
func (s Suite) CheckDeterminism(t testing.TB, corpus []string) {
	t.Helper()
	o := s.options()
	byteExact := s.Renderer.Caps().ByteExact

	for _, v := range sample(corpus, 24) {
		a, err := blazon.SVGWith(s.Renderer, v, o)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if _, err := blazon.SVGWith(s.Renderer, "9.9.9", o); err != nil {
			t.Fatalf("interleaved render: %v", err)
		}
		b, err := blazon.SVGWith(s.Renderer, v, o)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if string(a) != string(b) {
			t.Errorf("%s: rendered differently on a second call", v)
		}
	}

	if !byteExact {
		return
	}
	// A ByteExact renderer must also match its own raster output bit for bit.
	for _, v := range sample(corpus, 8) {
		a, err := blazon.ImageWith(s.Renderer, v, o)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		b, err := blazon.ImageWith(s.Renderer, v, o)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if string(a.Pix) != string(b.Pix) {
			t.Errorf("%s: raster output is not byte-exact although the renderer claims it is", v)
		}
	}
}

// CheckDistinctness enforces the distance thresholds.
func (s Suite) CheckDistinctness(t testing.TB, corpus []string) {
	t.Helper()
	rep, err := s.Report(corpus)
	if err != nil {
		t.Fatalf("blazontest: %v", err)
	}

	th := s.Thresholds
	check := func(name string, st Stats, min int) {
		if st.Count == 0 {
			t.Errorf("%s: the corpus contains no such pairs, so the threshold is untested", name)
			return
		}
		allowed := int(th.FailFraction * float64(st.Count))
		if st.Below(min) > allowed {
			t.Errorf("%s: %d of %d pairs are below the minimum distance of %d "+
				"(min %d, mean %.1f, allowed %d)\n  worst: %s",
				name, st.Below(min), st.Count, min, st.Min, st.Mean, allowed, st.WorstString(3))
		}
	}
	check("patch neighbours", rep.Patch, th.MinPatch)
	check("minor neighbours", rep.Minor, th.MinMinor)
	check("major jumps", rep.Major, th.MinMajor)

	if th.MinMajor < th.MinPatch {
		t.Errorf("thresholds are miscalibrated: MinMajor (%d) is below MinPatch (%d)",
			th.MinMajor, th.MinPatch)
	}

	if n := rep.All.Below(th.Floor); n > 0 {
		t.Errorf("global floor: %d of %d pairs in the corpus are within %d bits of each other\n  worst: %s",
			n, rep.All.Count, th.Floor, rep.All.WorstString(5))
	}
}

// CheckFamilyCohesion is the counterweight to CheckDistinctness. Distance
// alone is satisfied by making every mark unrelated, which would make the
// family policy a lie; this asserts that versions sharing a major really are
// closer to each other than to versions that do not.
func (s Suite) CheckFamilyCohesion(t testing.TB, corpus []string) {
	t.Helper()
	rep, err := s.Report(corpus)
	if err != nil {
		t.Fatalf("blazontest: %v", err)
	}
	if rep.IntraMajor.Count == 0 || rep.InterMajor.Count == 0 {
		t.Skip("corpus has too few majors to measure cohesion")
	}
	if rep.IntraMajor.Mean >= rep.InterMajor.Mean {
		t.Errorf("versions sharing a major are not closer to each other than to other majors: "+
			"intra %.1f vs inter %.1f — the family policy is not doing anything",
			rep.IntraMajor.Mean, rep.InterMajor.Mean)
	}
}

// CheckMetricStability asserts that the measurement is a property of the marks
// rather than of the resolution they happened to be rendered at.
//
// This is a check on the instrument, not on the renderer. A perceptual hash
// that point-samples its grid will happily report six bits of difference
// between two marks a human cannot tell apart, purely because an anti-aliased
// edge moved by a fraction of a pixel — and thresholds calibrated against that
// noise are measuring the rasteriser. Rendering the same corpus at two
// resolutions and requiring the mean distance to agree is what keeps that
// honest.
func (s Suite) CheckMetricStability(t testing.TB, corpus []string, a, b int) {
	t.Helper()
	mean := func(size int) float64 {
		sub := s
		sub.Options.Size = size
		rep, err := sub.Report(corpus)
		if err != nil {
			t.Fatalf("blazontest: %v", err)
		}
		return rep.All.Mean
	}
	ma, mb := mean(a), mean(b)
	const tolerance = 2.0
	if diff := ma - mb; diff > tolerance || diff < -tolerance {
		t.Errorf("mean distance moved from %.1f at %dpx to %.1f at %dpx (%.1f bits); "+
			"the measurement is tracking the render resolution, not the marks",
			ma, a, mb, b, diff)
	}
}

// CheckUniqueness asserts that no two versions in the corpus render to the
// same mark. Unlike the distance checks this is exact rather than perceptual,
// and it applies to every renderer: two different versions sharing one mark is
// the single failure an identicon library may never have. It is also what
// makes a deliberately small prerelease marker safe.
func (s Suite) CheckUniqueness(t testing.TB, corpus []string) {
	t.Helper()
	o := s.options()
	seen := make(map[string]string, len(corpus))
	for _, v := range corpus {
		svg, err := blazon.SVGWith(s.Renderer, v, o)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		key := string(svg)
		if prev, dup := seen[key]; dup {
			t.Errorf("%s and %s render identically", prev, v)
			continue
		}
		seen[key] = v
	}
}

// Pair is one measured version pair.
type Pair struct {
	A, B string
	Dist int
}

// Stats summarises a set of pair distances.
type Stats struct {
	Count int
	Min   int
	Max   int
	Mean  float64
	// Hist counts pairs by distance, indexed by distance 0..phash.MaxDistance.
	Hist [phash.MaxDistance + 1]int
	// Worst holds the closest pairs, ascending, for error messages.
	Worst []Pair
}

// Below reports how many pairs fall short of d.
func (s Stats) Below(d int) int {
	n := 0
	for i := 0; i < d && i < len(s.Hist); i++ {
		n += s.Hist[i]
	}
	return n
}

// WorstString formats the n closest pairs.
func (s Stats) WorstString(n int) string {
	if len(s.Worst) < n {
		n = len(s.Worst)
	}
	parts := make([]string, 0, n)
	for _, p := range s.Worst[:n] {
		parts = append(parts, fmt.Sprintf("%s~%s=%d", p.A, p.B, p.Dist))
	}
	return strings.Join(parts, ", ")
}

const worstKept = 8

func newStats() *statsBuilder { return &statsBuilder{min: 1 << 30} }

type statsBuilder struct {
	count int
	sum   int
	min   int
	max   int
	hist  [phash.MaxDistance + 1]int
	worst []Pair
}

func (b *statsBuilder) add(a, bb string, d int) {
	b.count++
	b.sum += d
	if d < b.min {
		b.min = d
	}
	if d > b.max {
		b.max = d
	}
	if d >= 0 && d < len(b.hist) {
		b.hist[d]++
	}
	b.worst = append(b.worst, Pair{A: a, B: bb, Dist: d})
	if len(b.worst) > 4*worstKept {
		b.trim()
	}
}

func (b *statsBuilder) trim() {
	sort.Slice(b.worst, func(i, j int) bool { return b.worst[i].Dist < b.worst[j].Dist })
	if len(b.worst) > worstKept {
		b.worst = b.worst[:worstKept]
	}
}

func (b *statsBuilder) done() Stats {
	b.trim()
	s := Stats{Count: b.count, Min: b.min, Max: b.max, Hist: b.hist, Worst: b.worst}
	if b.count == 0 {
		s.Min = 0
		return s
	}
	s.Mean = float64(b.sum) / float64(b.count)
	return s
}

// Report holds the measured distances for a renderer over a corpus. The dist
// subcommand prints it so that the thresholds can be set from real numbers
// instead of guessed.
type Report struct {
	Renderer string
	Policy   blazon.Policy
	Versions int

	// Patch, Minor and Major cover versions differing in exactly one
	// component by exactly one step.
	Patch, Minor, Major Stats
	// All covers every pair of distinct core versions. Pairs that share a core
	// version and differ only in prerelease metadata are counted in
	// Prerelease instead.
	All Stats
	// Prerelease covers pairs sharing a core version.
	Prerelease Stats
	// IntraMajor and InterMajor support the cohesion check.
	IntraMajor, InterMajor Stats
}

// Report renders the corpus and measures it.
func (s Suite) Report(corpus []string) (*Report, error) {
	o := s.options()
	if len(corpus) == 0 {
		c, err := DefaultCorpus()
		if err != nil {
			return nil, err
		}
		corpus = c
	}

	type entry struct {
		s string
		v blazon.Version
		h phash.Combined
	}
	entries := make([]entry, 0, len(corpus))
	seen := make(map[string]bool, len(corpus))
	for _, str := range corpus {
		v, err := blazon.ParseVersion(str)
		if err != nil {
			return nil, fmt.Errorf("corpus entry %q: %w", str, err)
		}
		// A repeated entry would score a distance of zero against itself and
		// quietly poison every statistic.
		if seen[v.String()] {
			return nil, fmt.Errorf("corpus contains %q twice", v.String())
		}
		seen[v.String()] = true
		img, err := blazon.ImageWith(s.Renderer, str, o)
		if err != nil {
			return nil, fmt.Errorf("rendering %q: %w", str, err)
		}
		entries = append(entries, entry{s: str, v: v, h: phash.Of(img)})
	}

	patch, minor, major := newStats(), newStats(), newStats()
	all, intra, inter, pre := newStats(), newStats(), newStats(), newStats()

	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			a, b := entries[i], entries[j]
			d := phash.Distance(a.h, b.h)

			if a.v.Core() == b.v.Core() {
				// Same release, different prerelease or build metadata. These
				// are meant to look alike, so they are bounded from above
				// rather than below and take no part in the other statistics.
				pre.add(a.s, b.s, d)
				continue
			}
			all.add(a.s, b.s, d)

			// Only released versions carry the component relations; a
			// prerelease is not the neighbour of anything.
			corePair := a.v.Prerelease == "" && b.v.Prerelease == ""

			if a.v.Major == b.v.Major {
				intra.add(a.s, b.s, d)
			} else {
				inter.add(a.s, b.s, d)
			}
			if !corePair {
				continue
			}
			switch {
			case a.v.Major == b.v.Major && a.v.Minor == b.v.Minor && step(a.v.Patch, b.v.Patch):
				patch.add(a.s, b.s, d)
			case a.v.Major == b.v.Major && a.v.Patch == b.v.Patch && step(a.v.Minor, b.v.Minor):
				minor.add(a.s, b.s, d)
			case a.v.Minor == b.v.Minor && a.v.Patch == b.v.Patch && step(a.v.Major, b.v.Major):
				major.add(a.s, b.s, d)
			}
		}
	}

	return &Report{
		Renderer:   s.Renderer.Name(),
		Policy:     o.Policy,
		Versions:   len(entries),
		Patch:      patch.done(),
		Minor:      minor.done(),
		Major:      major.done(),
		All:        all.done(),
		Prerelease: pre.done(),
		IntraMajor: intra.done(),
		InterMajor: inter.done(),
	}, nil
}

// step reports whether two component values are exactly one apart.
func step(a, b uint64) bool {
	if a > b {
		a, b = b, a
	}
	return b-a == 1
}

// sample returns up to n evenly spaced entries, so a check covers the whole
// corpus without rendering all of it.
func sample(corpus []string, n int) []string {
	if len(corpus) <= n {
		return corpus
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, corpus[i*len(corpus)/n])
	}
	return out
}

//go:embed corpus.txt
var builtinCorpus string

// DefaultCorpus returns the corpus shipped with this package.
//
// It is embedded rather than read from testdata so that the measurement works
// from any working directory and for any project that imports this package —
// a threshold suite that only runs from the repository root would be useless
// to the callers it is meant to serve.
func DefaultCorpus() ([]string, error) {
	return parseCorpus(strings.NewReader(builtinCorpus))
}

// LoadCorpus reads a newline-separated version list. Blank lines and lines
// beginning with # are ignored.
func LoadCorpus(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseCorpus(f)
}

// parseCorpus reads a version list, ignoring blank lines and comments.
func parseCorpus(r io.Reader) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

// ContactSheet lays marks out in a labelled grid. Threshold numbers say
// whether a renderer passes; a contact sheet is how a human decides whether it
// is any good.
func ContactSheet(r blazon.Renderer, versions []string, o blazon.Options, cols, cell int) (*image.RGBA, error) {
	if cols <= 0 {
		cols = 8
	}
	if cell <= 0 {
		cell = 128
	}
	// Room for the label plus a little air above and below it.
	labelH := canvas.GlyphH*maxLabelScale + 6
	rows := (len(versions) + cols - 1) / cols
	if rows == 0 {
		rows = 1
	}

	o.Renderer = r.Name()
	o.Size = cell
	if o.Background == blazon.BackgroundTransparent {
		o.Background = blazon.BackgroundLight
	}

	sheet := image.NewRGBA(image.Rect(0, 0, cols*cell, rows*(cell+labelH)))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	for i, v := range versions {
		img, err := blazon.ImageWith(r, v, o)
		if err != nil {
			return nil, fmt.Errorf("rendering %q: %w", v, err)
		}
		cx := (i % cols) * cell
		cy := (i / cols) * (cell + labelH)
		draw.Draw(sheet, image.Rect(cx, cy, cx+cell, cy+cell), img, image.Point{}, draw.Src)
		scale := labelScaleFor(v, cell)
		// Baseline-align the label whatever scale it ended up at, so a row of
		// mixed scales still reads as one line.
		top := cy + cell + 3 + (canvas.GlyphH*maxLabelScale-canvas.GlyphH*scale)/2
		label(sheet, cx+3, top, v, scale)
	}
	return sheet, nil
}

// maxLabelScale is the largest whole multiple of the font size a label is
// drawn at. Whole multiples only: a bitmap font resampled to a fractional size
// loses strokes.
const maxLabelScale = 2

// labelScaleFor picks the largest scale whose label still fits its cell. A
// long prerelease string in a narrow cell would otherwise run into its
// neighbour, and a contact sheet whose captions overlap cannot be read.
func labelScaleFor(s string, cellW int) int {
	for scale := maxLabelScale; scale > 1; scale-- {
		if canvas.StringWidth(s, scale) <= cellW-6 {
			return scale
		}
	}
	return 1
}

func label(dst *image.RGBA, x, y int, s string, scale int) {
	canvas.DrawString(dst, x, y, s, color.RGBA{40, 40, 40, 255}, scale)
}
