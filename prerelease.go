package blazon

import "image/color"

// MarkPrerelease draws the prerelease marker: a short row of dots along the
// bottom edge of the drawing box, their count and spacing taken from the
// prerelease entropy stream.
//
// The marker is deliberately small. A prerelease must still read as the
// release it precedes — 1.0.0-rc.1 is a 1.0.0 — so it may not disturb the
// main shape. But it must exist: without it, 1.0.0 and 1.0.0-rc.1 would render
// to identical bytes, and two different versions sharing one mark is the one
// thing an identicon may never do.
//
// It lives here rather than in each renderer so that the rule is the same
// everywhere and a new renderer gets it by calling one function.
func MarkPrerelease(ctx *Context) {
	v := ctx.Params.Version()
	if !v.IsPrerelease() {
		return
	}
	pre := ctx.Params.Pre()
	box := ctx.Box

	n := pre.IntRange(2, 5)
	r := box.Min() * 0.024
	gap := r * 3.0
	width := gap * float64(n-1)

	cx, _ := box.Center()
	x0 := cx - width/2
	y := box.Y + box.H - r*1.6

	for i := 0; i < n; i++ {
		// A filled dot, drawn as four quarter-circle Béziers.
		x := x0 + gap*float64(i)
		// Vary the radius slightly so the marker itself carries a little of
		// the prerelease identity: rc.1 and rc.2 differ here too.
		rr := r * pre.Range(0.7, 1.15)
		circle(ctx.Canvas, x, y, rr)
	}
	// The third ink, not the first: the marker usually lands on top of the
	// mark itself, and drawn in the mark's own colour it would be invisible —
	// which is how 1.0.0 and 1.0.0-rc.1 end up looking like the same release.
	ctx.Canvas.Fill(ctx.Palette.Ink(2))
}

// MarkPrereleaseCells is the character-grid form of the marker: a run of dots
// in a row the renderer has reserved for it. Text renderers call this instead
// of MarkPrerelease, which draws vector geometry a character canvas would have
// to rasterise.
//
// The row must be reserved rather than borrowed. Overwriting cells the
// renderer has already drawn would hide part of the mark itself, and a marker
// that destroys information is worse than no marker.
func MarkPrereleaseCells(ctx *Context, cols, row int) {
	if !ctx.Params.Version().IsPrerelease() {
		return
	}
	pre := ctx.Params.Pre()

	// A fixed-width code rather than a handful of dots. Each cell carries two
	// bits, so a band of eleven cells distinguishes four million prereleases;
	// a marker of three or four dots would draw from so few bits that two
	// prereleases of the same release could land on the same mark, and
	// uniqueness is the one guarantee that may not be probabilistic in
	// practice.
	glyphs := []rune{'-', '.', 'o', '='}
	// Half the row: sixteen bits is far more than the corpus needs to stay
	// collision-free, and a band any wider starts to compete with the mark
	// itself for attention.
	width := cols / 2
	if width < 4 {
		width = 4
	}
	if width > cols {
		width = cols
	}
	start := (cols - width) / 2

	for i := 0; i < width; i++ {
		col := start + i
		if col < 0 || col >= cols {
			continue
		}
		ctx.Canvas.Cell(col, row, glyphs[pre.Choice(len(glyphs))], ctx.Palette.Ink(1), color.Transparent)
	}
}

// circle appends a closed circle to the canvas path, wound clockwise so it
// unions with other clockwise contours under the non-zero rule.
func circle(c interface {
	MoveTo(x, y float64)
	CubicTo(x1, y1, x2, y2, x, y float64)
	Close()
}, cx, cy, r float64) {
	k := kappa * r
	c.MoveTo(cx+r, cy)
	c.CubicTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r)
	c.CubicTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy)
	c.CubicTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r)
	c.CubicTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy)
	c.Close()
}
