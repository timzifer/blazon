package canvas

import "math"

// Arc appends a circular arc from angle a0 to a1, in radians, measured
// clockwise from the positive x axis in the canvas's y-down coordinate system.
// The caller must have positioned the path at the arc's start point.
//
// The arc is split into segments of at most a quarter turn, because the cubic
// approximation of a circular arc only stays within a fraction of a pixel over
// spans that short.
func Arc(c Canvas, cx, cy, r, a0, a1 float64) {
	sweep := a1 - a0
	n := int(math.Ceil(math.Abs(sweep) / (math.Pi / 2)))
	if n < 1 {
		n = 1
	}
	step := sweep / float64(n)
	// Control point offset for a cubic approximating an arc of `step` radians.
	k := 4.0 / 3.0 * math.Tan(step/4)

	a := a0
	for i := 0; i < n; i++ {
		b := a + step
		x0, y0 := cx+r*math.Cos(a), cy+r*math.Sin(a)
		x1, y1 := cx+r*math.Cos(b), cy+r*math.Sin(b)
		c.CubicTo(
			x0-k*r*math.Sin(a), y0+k*r*math.Cos(a),
			x1+k*r*math.Sin(b), y1-k*r*math.Cos(b),
			x1, y1,
		)
		a = b
	}
}

// Polar converts polar coordinates to a canvas point.
func Polar(cx, cy, r, angle float64) Point {
	return Point{X: cx + r*math.Cos(angle), Y: cy + r*math.Sin(angle)}
}

// AnnularSector appends a closed wedge between two radii and two angles. It is
// the cell shape of the polar renderer, and the reason the canvas needs an arc
// primitive at all.
func AnnularSector(c Canvas, cx, cy, r0, r1, a0, a1 float64) {
	p := Polar(cx, cy, r0, a0)
	c.MoveTo(p.X, p.Y)

	q := Polar(cx, cy, r1, a0)
	c.LineTo(q.X, q.Y)
	Arc(c, cx, cy, r1, a0, a1)

	p = Polar(cx, cy, r0, a1)
	c.LineTo(p.X, p.Y)
	Arc(c, cx, cy, r0, a1, a0)

	c.Close()
}
