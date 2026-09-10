package canvas

import "math"

// Simplify reduces a polyline with the Ramer–Douglas–Peucker algorithm,
// keeping every point that lies further than tol from the retained chord.
//
// Renderers that sample an analytic curve need a dense sample to catch its
// sharp features, but emitting every sample would make the SVG an order of
// magnitude larger than it needs to be. Simplifying afterwards keeps the
// detail where the curve actually bends and throws away the rest.
func Simplify(pts []Point, tol float64) []Point {
	if len(pts) < 3 || tol <= 0 {
		return pts
	}
	keep := make([]bool, len(pts))
	keep[0] = true
	keep[len(pts)-1] = true
	simplifySection(pts, 0, len(pts)-1, tol, keep)

	out := make([]Point, 0, len(pts))
	for i, k := range keep {
		if k {
			out = append(out, pts[i])
		}
	}
	return out
}

// simplifySection is iterative rather than recursive: a dense sample of a
// pathological curve can otherwise nest thousands of frames deep.
func simplifySection(pts []Point, first, last int, tol float64, keep []bool) {
	type span struct{ a, b int }
	stack := []span{{first, last}}

	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s.b <= s.a+1 {
			continue
		}
		maxDist, maxIdx := 0.0, -1
		for i := s.a + 1; i < s.b; i++ {
			if d := perpDistance(pts[i], pts[s.a], pts[s.b]); d > maxDist {
				maxDist, maxIdx = d, i
			}
		}
		if maxIdx < 0 || maxDist <= tol {
			continue
		}
		keep[maxIdx] = true
		stack = append(stack, span{s.a, maxIdx}, span{maxIdx, s.b})
	}
}

func perpDistance(p, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / l2
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}

// ClosedCurve emits a smooth closed contour through pts using a Catmull-Rom
// spline converted to cubic Béziers. The caller supplies the sample points;
// this turns them into a curve that stays smooth at every sample rather than
// showing the facets of a polyline.
func ClosedCurve(c Canvas, pts []Point) {
	n := len(pts)
	if n < 3 {
		return
	}
	at := func(i int) Point { return pts[((i%n)+n)%n] }

	c.MoveTo(pts[0].X, pts[0].Y)
	for i := 0; i < n; i++ {
		p0, p1, p2, p3 := at(i-1), at(i), at(i+1), at(i+2)
		// Catmull-Rom to Bézier: the tangent at each point is one sixth of the
		// vector between its neighbours.
		c.CubicTo(
			p1.X+(p2.X-p0.X)/6, p1.Y+(p2.Y-p0.Y)/6,
			p2.X-(p3.X-p1.X)/6, p2.Y-(p3.Y-p1.Y)/6,
			p2.X, p2.Y,
		)
	}
	c.Close()
}

// ReverseInPlace flips a point slice. A contour drawn in reverse winds the
// other way, which is how a renderer punches a hole under the non-zero fill
// rule.
func ReverseInPlace(pts []Point) {
	for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
		pts[i], pts[j] = pts[j], pts[i]
	}
}
