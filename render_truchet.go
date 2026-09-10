package blazon

import "github.com/timzifer/blazon/canvas"

func init() {
	Register(truchetRenderer{})
}

// truchetRenderer draws Truchet tiles: one tile per grid cell, each carrying
// two arcs or two diagonals that meet the cell edges at their midpoints.
// Because every tile terminates on the same four edge midpoints, the marks
// join across cell boundaries whatever the rotation — the pattern reads as
// continuous loops and labyrinths rather than as a field of confetti, which is
// what makes it the cheapest escape from ordinary block identicons.
//
// All of its geometry comes from addition and multiplication on grid
// coordinates, with the quarter-circle Bézier constant as the only irrational
// term, so its output is byte-exact across architectures.
//
// Amplified parameters: whole-grid quarter turn, stroke weight, and — through
// the palette — hue.
type truchetRenderer struct{}

func (truchetRenderer) Name() string { return "truchet" }

func (truchetRenderer) Caps() Caps { return Caps{ByteExact: true} }

// kappa is the control point offset that approximates a quarter circle with a
// cubic Bézier, 4/3*(sqrt(2)-1).
const kappa = 0.5522847498307936

// Tile sets. Arcs give loops and knots; diagonals give a sharper, more
// crystalline lattice; mixing them breaks up the regularity of both.
const (
	tilesArcs = iota
	tilesDiagonals
	tilesMixed
)

func (truchetRenderer) Render(ctx *Context) error {
	p := ctx.Params
	box := ctx.Box

	n := p.Variant().IntRange(5, 9)
	tiles := p.Archetype().Choice(3)

	// Amplified: a patch bump turns the whole grid and changes line weight,
	// which separates neighbouring patches even where the cell bits happen to
	// land similarly.
	turn := p.Amplified().Choice(4)
	weight := p.Amplified().Range(0.16, 0.34)

	// Square the drawing area so cells stay square.
	s := box.Min()
	cell := s / float64(n)
	ox := box.X + (box.W-s)/2
	oy := box.Y + (box.H-s)/2

	rot := quarterTurn{turn: turn, cx: ox + s/2, cy: oy + s/2}

	detail := p.Detail()
	variant := p.Variant()

	// Two ink passes rather than one path per cell: fewer, longer paths keep
	// the SVG small and let the strokes merge cleanly.
	for ink := 0; ink < 2; ink++ {
		d := detail.Derive("cells")
		vi := variant.Derive("inks")
		any := false
		for row := 0; row < n; row++ {
			for col := 0; col < n; col++ {
				orient := int(d.Bit())
				kind := tiles
				if tiles == tilesMixed {
					kind = int(d.Bit())
				}
				if vi.Choice(2) != ink {
					continue
				}
				any = true
				x := ox + float64(col)*cell
				y := oy + float64(row)*cell
				drawTile(ctx.Canvas, rot, kind, orient, x, y, cell)
			}
		}
		if any {
			ctx.Canvas.Stroke(ctx.Palette.Ink(ink), weight*cell)
		}
	}
	MarkPrerelease(ctx)
	return nil
}

// drawTile emits one tile's two strokes.
func drawTile(c canvas.Canvas, rot quarterTurn, kind, orient int, x, y, s float64) {
	r := s / 2
	k := kappa * r

	// Edge midpoints.
	top := [2]float64{x + r, y}
	bottom := [2]float64{x + r, y + s}
	left := [2]float64{x, y + r}
	right := [2]float64{x + s, y + r}

	if kind == tilesDiagonals {
		if orient == 0 {
			strokeLine(c, rot, top, left)
			strokeLine(c, rot, bottom, right)
		} else {
			strokeLine(c, rot, top, right)
			strokeLine(c, rot, bottom, left)
		}
		return
	}

	if orient == 0 {
		// Arc centred on the top-left corner, and its mirror on the bottom
		// right.
		strokeArc(c, rot, top, [2]float64{x + r, y + k}, [2]float64{x + k, y + r}, left)
		strokeArc(c, rot, bottom, [2]float64{x + r, y + s - k}, [2]float64{x + s - k, y + r}, right)
	} else {
		strokeArc(c, rot, top, [2]float64{x + r, y + k}, [2]float64{x + s - k, y + r}, right)
		strokeArc(c, rot, bottom, [2]float64{x + r, y + s - k}, [2]float64{x + k, y + r}, left)
	}
}

func strokeLine(c canvas.Canvas, rot quarterTurn, a, b [2]float64) {
	ax, ay := rot.apply(a[0], a[1])
	bx, by := rot.apply(b[0], b[1])
	c.MoveTo(ax, ay)
	c.LineTo(bx, by)
}

func strokeArc(c canvas.Canvas, rot quarterTurn, p0, c1, c2, p3 [2]float64) {
	x0, y0 := rot.apply(p0[0], p0[1])
	x1, y1 := rot.apply(c1[0], c1[1])
	x2, y2 := rot.apply(c2[0], c2[1])
	x3, y3 := rot.apply(p3[0], p3[1])
	c.MoveTo(x0, y0)
	c.CubicTo(x1, y1, x2, y2, x3, y3)
}

// quarterTurn rotates about a centre by a multiple of 90 degrees. Multiples of
// a right angle are used rather than a free angle so that the transform stays
// exact: it only ever negates and swaps coordinates, which keeps the renderer
// byte-reproducible.
type quarterTurn struct {
	turn   int
	cx, cy float64
}

func (q quarterTurn) apply(x, y float64) (float64, float64) {
	dx, dy := x-q.cx, y-q.cy
	switch q.turn & 3 {
	case 1:
		dx, dy = -dy, dx
	case 2:
		dx, dy = -dx, -dy
	case 3:
		dx, dy = dy, -dx
	}
	return q.cx + dx, q.cy + dy
}
