package blazon

import "github.com/timzifer/blazon/canvas"

func init() {
	Register(cistercianRenderer{})
}

// cistercianRenderer writes the version in Cistercian numerals: one stave per
// component, each carrying up to four digits in its four quadrants.
//
// This is the only renderer here that is legible rather than merely
// distinctive. Nothing is hashed: the glyph is a direct encoding of the
// number, so after a few minutes of learning the notation a reader gets the
// version back out of the mark. That also makes it the only one that is
// ordered — 1.4.2 and 1.4.3 differ by exactly one stroke, which is the point,
// not a defect — so it is exempt from the distinctness thresholds and held to
// injectivity instead.
//
// The medieval numerals encode 0–9999 on a single stave: units top right, tens
// top left, hundreds bottom right, thousands bottom left. Version components
// are unbounded, so one stave per component is not enough in general; a
// component of 10000 or more takes a second stave to its left, exactly as an
// extra group of four digits.
type cistercianRenderer struct{}

func (cistercianRenderer) Name() string { return "cistercian" }

func (cistercianRenderer) Caps() Caps {
	return Caps{ByteExact: true, Ordered: true}
}

// cistercianBase is how much one stave holds.
const cistercianBase = 10000

// cistercianGroups splits a component into stave values, most significant
// first. A value below the base yields exactly one group, so the common case
// is one stave per component.
func cistercianGroups(v uint64) []uint64 {
	if v < cistercianBase {
		return []uint64{v}
	}
	var out []uint64
	for v > 0 {
		out = append(out, v%cistercianBase)
		v /= cistercianBase
	}
	// Reverse into most-significant-first order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// cistercianStaves returns the stave values for a version, together with the
// component each belongs to, so the renderer can colour them.
func cistercianStaves(v Version) (values []uint64, component []int) {
	for i, c := range []uint64{v.Major, v.Minor, v.Patch} {
		for _, g := range cistercianGroups(c) {
			values = append(values, g)
			component = append(component, i)
		}
	}
	return values, component
}

func (cistercianRenderer) Render(ctx *Context) error {
	v := ctx.Params.Version()
	values, component := cistercianStaves(v)

	box := ctx.Box
	n := len(values)

	// Each stave gets a column; the glyph occupies the middle of it, so
	// neighbouring staves cannot touch.
	colW := box.W / float64(n)
	glyphW := colW * 0.62
	h := box.H * 0.82
	top := box.Y + (box.H-h)/2
	// The quadrant is as wide as half the glyph and as tall as half the stave.
	ux := glyphW / 2
	uy := h / 2
	weight := minFloat(ux, uy) * 0.22

	for i, val := range values {
		cx := box.X + colW*(float64(i)+0.5)
		drawCistercianStave(ctx.Canvas, cx, top, h, ux, uy, val)
		ctx.Canvas.Stroke(ctx.Palette.Ink(component[i]), weight)
	}

	MarkPrerelease(ctx)
	return nil
}

// drawCistercianStave emits the stave and the four quadrant digits.
func drawCistercianStave(c canvas.Canvas, cx, top, h, ux, uy float64, value uint64) {
	c.MoveTo(cx, top)
	c.LineTo(cx, top+h)

	units := int(value % 10)
	tens := int(value / 10 % 10)
	hundreds := int(value / 100 % 10)
	thousands := int(value / 1000 % 10)

	bottom := top + h
	// The four quadrants are the same nine strokes reflected: units top right,
	// tens top left, hundreds bottom right, thousands bottom left.
	drawCistercianDigit(c, units, cx, top, ux, uy)
	drawCistercianDigit(c, tens, cx, top, -ux, uy)
	drawCistercianDigit(c, hundreds, cx, bottom, ux, -uy)
	drawCistercianDigit(c, thousands, cx, bottom, -ux, -uy)
}

// drawCistercianDigit emits one digit in the quadrant anchored at (ox,oy),
// extending by ux horizontally and uy vertically. Either may be negative,
// which is how the same nine strokes serve all four quadrants.
//
// The strokes are the traditional ones: 1 and 2 are the near and far
// horizontals, 3 and 4 the two diagonals, 6 the outer vertical, and 5, 7, 8
// and 9 combinations of those.
func drawCistercianDigit(c canvas.Canvas, d int, ox, oy, ux, uy float64) {
	if d <= 0 || d > 9 {
		return
	}
	// Quadrant corners.
	near := func() (float64, float64) { return ox, oy }
	nearFar := func() (float64, float64) { return ox, oy + uy }
	far := func() (float64, float64) { return ox + ux, oy }
	farFar := func() (float64, float64) { return ox + ux, oy + uy }

	line := func(a, b func() (float64, float64)) {
		ax, ay := a()
		bx, by := b()
		c.MoveTo(ax, ay)
		c.LineTo(bx, by)
	}

	switch d {
	case 1:
		line(near, far)
	case 2:
		line(nearFar, farFar)
	case 3:
		line(near, farFar)
	case 4:
		line(nearFar, far)
	case 5:
		line(near, far)
		line(near, farFar)
	case 6:
		line(far, farFar)
	case 7:
		line(near, far)
		line(far, farFar)
	case 8:
		line(nearFar, farFar)
		line(far, farFar)
	case 9:
		line(near, far)
		line(far, farFar)
		line(nearFar, farFar)
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
