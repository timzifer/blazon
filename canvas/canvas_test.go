package canvas

import (
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestPathFlattenLine(t *testing.T) {
	var p Path
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	lines := p.Flatten()
	if len(lines) != 1 {
		t.Fatalf("got %d subpaths, want 1", len(lines))
	}
	if got, want := len(lines[0]), 3; got != want {
		t.Errorf("got %d points, want %d", got, want)
	}
}

func TestPathFlattenClosedRepeatsStart(t *testing.T) {
	var p Path
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.Close()
	pts := p.Flatten()[0]
	if pts[len(pts)-1] != (Point{0, 0}) {
		t.Errorf("closed subpath does not end at its start: %+v", pts[len(pts)-1])
	}
}

// TestFlattenCubicAccuracy checks the chord tolerance against a curve whose
// exact form is known: the standard quarter-circle Bézier.
func TestFlattenCubicAccuracy(t *testing.T) {
	const r = 100
	const k = 0.5522847498307936 * r
	var p Path
	p.MoveTo(r, 0)
	p.CubicTo(r, k, k, r, 0, r)
	pts := p.Flatten()[0]
	if len(pts) < 8 {
		t.Fatalf("only %d points for a quarter circle", len(pts))
	}
	for _, pt := range pts {
		d := math.Hypot(pt.X, pt.Y)
		// The Bézier itself deviates from a true circle by ~0.02%; the
		// flattening must not add more than the stated tolerance on top.
		if math.Abs(d-r) > 0.03*r+flattenTolerance {
			t.Errorf("point %+v is %v from the origin, want ~%v", pt, d, float64(r))
		}
	}
}

func TestStrokeOutlineCoversEndpoints(t *testing.T) {
	polys := StrokeOutline([][]Point{{{0, 0}, {10, 0}}}, 4)
	if len(polys) != 3 { // one quad plus two caps
		t.Fatalf("got %d polygons, want 3", len(polys))
	}
	// The quad must be two units either side of the segment.
	var minY, maxY float64
	for _, p := range polys[0] {
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	if math.Abs(minY+2) > 1e-9 || math.Abs(maxY-2) > 1e-9 {
		t.Errorf("stroke quad spans y in [%v,%v], want [-2,2]", minY, maxY)
	}
}

func TestStrokeOutlineHandlesDegenerateInput(t *testing.T) {
	if got := StrokeOutline([][]Point{{{5, 5}}}, 4); len(got) != 1 {
		t.Errorf("single point produced %d polygons, want 1 cap disc", len(got))
	}
	// Repeated points must not produce NaN normals.
	polys := StrokeOutline([][]Point{{{1, 1}, {1, 1}, {1, 1}}}, 4)
	for _, poly := range polys {
		for _, p := range poly {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) {
				t.Fatal("duplicate points produced NaN geometry")
			}
		}
	}
	if got := StrokeOutline([][]Point{{{0, 0}, {1, 0}}}, 0); got != nil {
		t.Error("zero width produced geometry")
	}
}

func TestSVGDeterministicOutput(t *testing.T) {
	build := func() string {
		s := NewSVG(100, 100)
		s.Background(color.RGBA{255, 255, 255, 255})
		s.MoveTo(10, 10)
		s.CubicTo(20, 10, 30, 20, 30, 30)
		s.Stroke(color.RGBA{0, 0, 0, 255}, 2)
		s.MoveTo(50, 50)
		s.LineTo(60, 50)
		s.LineTo(60, 60)
		s.Close()
		s.Fill(color.RGBA{255, 0, 0, 128})
		return string(s.Bytes())
	}
	a, b := build(), build()
	if a != b {
		t.Fatal("two identical draw sequences produced different SVG")
	}
	for _, want := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"`,
		`stroke-linecap="round"`,
		`fill-opacity=`,
		` Z"`,
	} {
		if !strings.Contains(a, want) {
			t.Errorf("SVG output is missing %q\n%s", want, a)
		}
	}
}

func TestSVGPathResetsBetweenOps(t *testing.T) {
	s := NewSVG(10, 10)
	s.MoveTo(0, 0)
	s.LineTo(5, 5)
	s.Stroke(color.Black, 1)
	s.Fill(color.Black) // nothing left to fill
	if n := strings.Count(string(s.Bytes()), "<path"); n != 1 {
		t.Errorf("got %d path elements, want 1", n)
	}
}

func TestNumFormatting(t *testing.T) {
	cases := map[float64]string{
		0:        "0",
		-0.0001:  "0",
		1:        "1",
		1.5:      "1.5",
		1.23456:  "1.235",
		-3.25:    "-3.25",
		100.0000: "100",
	}
	for in, want := range cases {
		if got := num(in); got != want {
			t.Errorf("num(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestSVGCellNeedsGrid(t *testing.T) {
	s := NewSVG(100, 100)
	s.Cell(0, 0, 'x', color.Black, color.Transparent)
	if strings.Contains(string(s.Bytes()), "<text") {
		t.Error("Cell drew without a declared grid")
	}
	s.SetGrid(10, 10)
	s.Cell(1, 2, '&', color.Black, color.RGBA{255, 255, 255, 255})
	out := string(s.Bytes())
	if !strings.Contains(out, "&amp;") {
		t.Error("Cell did not escape its glyph")
	}
	if !strings.Contains(out, "<rect") {
		t.Error("Cell did not paint its opaque background")
	}
}

func TestRasterFillCoverage(t *testing.T) {
	r := NewRasterSS(20, 20, 1)
	r.MoveTo(5, 5)
	r.LineTo(15, 5)
	r.LineTo(15, 15)
	r.LineTo(5, 15)
	r.Close()
	r.Fill(color.RGBA{255, 0, 0, 255})
	img := r.Image()

	if c := img.RGBAAt(10, 10); c.R != 255 || c.A != 255 {
		t.Errorf("interior pixel = %+v, want opaque red", c)
	}
	if c := img.RGBAAt(1, 1); c.A != 0 {
		t.Errorf("exterior pixel = %+v, want transparent", c)
	}
}

// TestRasterWindingHole is the reason the Canvas interface has no even-odd
// mode: a reversed inner contour must punch a hole under the non-zero rule.
func TestRasterWindingHole(t *testing.T) {
	r := NewRasterSS(40, 40, 1)
	// Outer square, clockwise.
	r.MoveTo(5, 5)
	r.LineTo(35, 5)
	r.LineTo(35, 35)
	r.LineTo(5, 35)
	r.Close()
	// Inner square, counter-clockwise.
	r.MoveTo(15, 15)
	r.LineTo(15, 25)
	r.LineTo(25, 25)
	r.LineTo(25, 15)
	r.Close()
	r.Fill(color.RGBA{0, 0, 255, 255})
	img := r.Image()

	if c := img.RGBAAt(10, 20); c.A != 255 {
		t.Errorf("ring pixel = %+v, want opaque", c)
	}
	if c := img.RGBAAt(20, 20); c.A != 0 {
		t.Errorf("hole pixel = %+v, want transparent", c)
	}
}

func TestRasterStrokeIsContinuous(t *testing.T) {
	r := NewRasterSS(40, 40, 1)
	r.MoveTo(5, 20)
	r.LineTo(20, 20)
	r.LineTo(20, 35)
	r.Stroke(color.RGBA{0, 0, 0, 255}, 4)
	img := r.Image()
	// The join must be filled, not notched.
	for _, p := range [][2]int{{10, 20}, {20, 20}, {20, 30}, {21, 21}} {
		if c := img.RGBAAt(p[0], p[1]); c.A < 200 {
			t.Errorf("pixel %v on the stroke = %+v, want opaque", p, c)
		}
	}
}

func TestRasterBackgroundAndSize(t *testing.T) {
	r := NewRaster(16, 16)
	if w, h := r.Size(); w != 16 || h != 16 {
		t.Errorf("Size = %v,%v, want 16,16 in output pixels", w, h)
	}
	r.Background(color.RGBA{0, 255, 0, 255})
	if c := r.Image().RGBAAt(0, 0); c.G != 255 {
		t.Errorf("background pixel = %+v", c)
	}
	// A transparent background must leave the buffer untouched.
	r2 := NewRaster(4, 4)
	r2.Background(color.Transparent)
	if c := r2.Image().RGBAAt(0, 0); c.A != 0 {
		t.Errorf("transparent background painted %+v", c)
	}
}

func TestRasterSupersampleAntialiases(t *testing.T) {
	r := NewRaster(40, 40) // default supersampling
	r.MoveTo(5, 5)
	r.LineTo(35, 12)
	r.Stroke(color.RGBA{0, 0, 0, 255}, 3)
	img := r.Image()
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 40 {
		t.Fatalf("downsampled image is %v, want 40x40", b)
	}
	var partial, opaque int
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			switch a := img.RGBAAt(x, y).A; {
			case a == 255:
				opaque++
			case a > 0:
				partial++
			}
		}
	}
	if opaque == 0 {
		t.Error("no fully covered pixels on the stroke")
	}
	if partial == 0 {
		t.Error("no partially covered pixels: the edge is not anti-aliased")
	}
}

func TestRasterCell(t *testing.T) {
	r := NewRasterSS(70, 26, 1)
	r.SetGrid(10, 2)
	r.Cell(0, 0, 'A', color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255})
	img := r.Image()
	var lit int
	for y := 0; y < 13; y++ {
		for x := 0; x < 7; x++ {
			if c := img.RGBAAt(x, y); c.R > 128 {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Error("Cell drew no glyph pixels")
	}
}
