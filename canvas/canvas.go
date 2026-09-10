// Package canvas holds the render primitives — stage three of the pipeline.
// A canvas knows nothing about versions, hashes or parameters; a renderer
// knows nothing about output formats. Everything a renderer can draw is in the
// Canvas interface, and every output format implements exactly that.
package canvas

import (
	"image/color"
	"math"
)

// Canvas is a drawing target. Paths are built with MoveTo/LineTo/CubicTo and
// consumed by Fill or Stroke, which reset the path.
//
// Filling always uses the non-zero winding rule. A renderer that wants a hole
// emits the inner contour with the opposite orientation to the outer one; no
// even-odd mode exists, because the raster back end has no efficient way to
// provide one and the winding trick covers every case the renderers need.
//
// Strokes always use round caps and round joins. Every renderer here draws
// organic curves or arcs where that is the right choice, so it is fixed rather
// than configurable.
type Canvas interface {
	// Size reports the drawing area in user units. The origin is top left.
	Size() (w, h float64)

	// Background paints the whole area. A fully transparent colour is a no-op.
	Background(col color.Color)

	MoveTo(x, y float64)
	LineTo(x, y float64)
	CubicTo(x1, y1, x2, y2, x, y float64)
	// Close joins the current subpath back to its start point.
	Close()

	// Fill paints the accumulated path and resets it.
	Fill(col color.Color)
	// Stroke outlines the accumulated path and resets it.
	Stroke(col color.Color, width float64)

	// SetGrid declares a character grid. Renderers that draw with Cell must
	// call it before their first Cell; vector back ends use it to derive cell
	// metrics, and character back ends to size their buffer.
	SetGrid(cols, rows int)
	// Cell writes a single character. Canvases that cannot draw text may
	// ignore it, but all back ends here honour it.
	Cell(col, row int, r rune, fg, bg color.Color)
}

// Point is a coordinate pair.
type Point struct{ X, Y float64 }

// segKind distinguishes the two segment types a path can hold.
type segKind uint8

const (
	segLine segKind = iota
	segCubic
)

type segment struct {
	kind segKind
	// For a line only p3 is used; for a cubic, p1 and p2 are the controls.
	p1, p2, p3 Point
}

// subpath is a run of segments sharing a start point.
type subpath struct {
	start  Point
	segs   []segment
	closed bool
}

// Path accumulates geometry for both back ends. It is exported so that other
// packages in the module can build geometry once and hand it to any canvas.
type Path struct {
	subs []subpath
	cur  Point
	open bool
}

// MoveTo starts a new subpath.
func (p *Path) MoveTo(x, y float64) {
	p.subs = append(p.subs, subpath{start: Point{x, y}})
	p.cur = Point{x, y}
	p.open = true
}

// LineTo appends a straight segment, starting a subpath at the origin if none
// is open.
func (p *Path) LineTo(x, y float64) {
	p.ensureOpen()
	s := &p.subs[len(p.subs)-1]
	s.segs = append(s.segs, segment{kind: segLine, p3: Point{x, y}})
	p.cur = Point{x, y}
}

// CubicTo appends a cubic Bézier segment.
func (p *Path) CubicTo(x1, y1, x2, y2, x, y float64) {
	p.ensureOpen()
	s := &p.subs[len(p.subs)-1]
	s.segs = append(s.segs, segment{
		kind: segCubic,
		p1:   Point{x1, y1},
		p2:   Point{x2, y2},
		p3:   Point{x, y},
	})
	p.cur = Point{x, y}
}

// Close marks the current subpath closed.
func (p *Path) Close() {
	if len(p.subs) == 0 {
		return
	}
	s := &p.subs[len(p.subs)-1]
	s.closed = true
	p.cur = s.start
}

// Reset empties the path for reuse.
func (p *Path) Reset() {
	p.subs = p.subs[:0]
	p.cur = Point{}
	p.open = false
}

// Empty reports whether the path holds no geometry.
func (p *Path) Empty() bool {
	for _, s := range p.subs {
		if len(s.segs) > 0 {
			return false
		}
	}
	return true
}

func (p *Path) ensureOpen() {
	if !p.open || len(p.subs) == 0 {
		p.subs = append(p.subs, subpath{start: p.cur})
		p.open = true
	}
}

// flattenTolerance is the maximum chord deviation, in user units, allowed when
// converting curves to polylines. It is well below one device pixel at the
// sizes these marks are rendered at.
const flattenTolerance = 0.08

// Flatten converts the path to polylines. Closed subpaths repeat their start
// point as the final point, so a consumer can treat every polyline uniformly.
func (p *Path) Flatten() [][]Point {
	var out [][]Point
	for _, s := range p.subs {
		if len(s.segs) == 0 {
			continue
		}
		pts := []Point{s.start}
		cur := s.start
		for _, seg := range s.segs {
			switch seg.kind {
			case segLine:
				pts = append(pts, seg.p3)
			case segCubic:
				pts = appendFlattenedCubic(pts, cur, seg.p1, seg.p2, seg.p3)
			}
			cur = seg.p3
		}
		if s.closed && (cur != s.start) {
			pts = append(pts, s.start)
		}
		out = append(out, pts)
	}
	return out
}

// appendFlattenedCubic subdivides a cubic into line segments. The step count
// comes from the control polygon length, which bounds the curve's arc length,
// so the chord error stays under flattenTolerance without recursion.
func appendFlattenedCubic(dst []Point, p0, p1, p2, p3 Point) []Point {
	d := dist(p0, p1) + dist(p1, p2) + dist(p2, p3)
	n := int(math.Ceil(math.Sqrt(d / flattenTolerance)))
	if n < 1 {
		n = 1
	}
	if n > 256 {
		n = 256
	}
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		x := u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X
		y := u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y
		dst = append(dst, Point{x, y})
	}
	return dst
}

func dist(a, b Point) float64 { return math.Hypot(b.X-a.X, b.Y-a.Y) }

// StrokeOutline converts polylines into fillable polygons approximating a
// stroke with round caps and joins.
//
// Each segment becomes a quad and each vertex a disc, all wound in the same
// direction. Under the non-zero rule, overlapping same-winding polygons union,
// so the pieces merge into one solid stroke without any explicit join
// geometry. That is why the raster back end needs no real stroker.
func StrokeOutline(lines [][]Point, width float64) [][]Point {
	r := width / 2
	if r <= 0 {
		return nil
	}
	var out [][]Point
	for _, pts := range lines {
		pts = dedup(pts)
		if len(pts) == 0 {
			continue
		}
		if len(pts) == 1 {
			out = append(out, disc(pts[0], r))
			continue
		}
		for i := 0; i+1 < len(pts); i++ {
			a, b := pts[i], pts[i+1]
			dx, dy := b.X-a.X, b.Y-a.Y
			l := math.Hypot(dx, dy)
			nx, ny := -dy/l*r, dx/l*r
			out = append(out, []Point{
				{a.X + nx, a.Y + ny},
				{b.X + nx, b.Y + ny},
				{b.X - nx, b.Y - ny},
				{a.X - nx, a.Y - ny},
			})
		}
		for _, p := range pts {
			out = append(out, disc(p, r))
		}
	}
	return out
}

// discSegments is the vertex count of a round cap or join. Twenty-four keeps
// the facet error below a tenth of a pixel for the stroke widths in use.
const discSegments = 24

// disc is wound in the same direction as the segment quads above. That is not
// cosmetic: under the non-zero rule, two overlapping polygons of opposite
// winding cancel, which would punch holes in the stroke exactly at its joins
// and caps.
func disc(c Point, r float64) []Point {
	pts := make([]Point, 0, discSegments)
	for i := 0; i < discSegments; i++ {
		a := -2 * math.Pi * float64(i) / discSegments
		pts = append(pts, Point{c.X + r*math.Cos(a), c.Y + r*math.Sin(a)})
	}
	return pts
}

// dedup drops consecutive duplicate points, which would otherwise produce
// zero-length segments and NaN normals.
func dedup(pts []Point) []Point {
	out := pts[:0:0]
	for i, p := range pts {
		if i > 0 && math.Abs(p.X-out[len(out)-1].X) < 1e-12 && math.Abs(p.Y-out[len(out)-1].Y) < 1e-12 {
			continue
		}
		out = append(out, p)
	}
	return out
}
