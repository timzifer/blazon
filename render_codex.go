package blazon

import (
	"image/color"
	"math"
	"math/bits"

	"github.com/timzifer/blazon/canvas"
)

func init() {
	Register(dialRenderer{})
	Register(orbitRenderer{})
}

// The two codex renderers take the version's punctuation literally.
//
// A semantic version is separated by dots and a dash. Here the separator is
// geometric: in dial it is an angle, in orbit a radius. Each component gets a
// field of its own, and the field boundary is the dot. Nothing is hashed
// except the prerelease, so the marks are ordered — 1.4.2 and 1.4.3 differ by
// one cell, deliberately — and both are exempt from the distinctness
// thresholds and held to uniqueness instead.
//
// Within a field the value is written in binary, one cell per bit, most
// significant first: outermost band in dial, first cell clockwise from noon in
// orbit. Binary rather than a tally because version components are unbounded
// and a tally stops being readable somewhere around sixteen; binary reads any
// component a semantic version can hold, and reads it exactly. Two minutes of
// learning buys the whole scheme.
//
// The prerelease field is the exception: it carries hash bits from the
// prerelease stream, not a number. There is no useful ordering over "rc.1"
// versus "beta.2", so the field says *that* this is a prerelease and which one,
// without pretending to be legible. An empty field means a release, which is
// the distinction that matters at a glance.

// codexMinBits is the smallest field width. Eight cells looks like a field
// even when the version is 0.1.0, where a width fitted to the value would be a
// single cell and the mark would read as a mistake.
const codexMinBits = 8

// codexFieldCount is major, minor, patch, prerelease.
const codexFieldCount = 4

// codexWidth returns how many cells each field gets. All fields share one
// width so that the mark reads as a table rather than as four ragged rows, and
// so that a decoder needs no separators beyond the geometric ones.
//
// The width is rounded up to a whole nibble, which makes counting cells in
// groups of four possible by eye — the same reason hex is grouped that way.
func codexWidth(v Version) int {
	largest := v.Major
	if v.Minor > largest {
		largest = v.Minor
	}
	if v.Patch > largest {
		largest = v.Patch
	}
	n := bits.Len64(largest)
	if n < codexMinBits {
		n = codexMinBits
	}
	n = (n + 3) / 4 * 4
	if n > 64 {
		n = 64
	}
	return n
}

// codexField is one component's cells, most significant first.
type codexField struct {
	// present is false for the prerelease field of a released version, which
	// is drawn as an empty field rather than as zero.
	present bool
	// noise marks a field whose bits are hashed rather than counted.
	noise bool
	cells []bool
}

// codexFields lays out the four fields for a version.
func codexFields(p *Params, width int) [codexFieldCount]codexField {
	v := p.Version()
	var out [codexFieldCount]codexField

	for i, value := range []uint64{v.Major, v.Minor, v.Patch} {
		out[i] = codexField{present: true, cells: codexCells(value, width)}
	}

	if v.IsPrerelease() {
		out[3] = codexField{
			present: true,
			noise:   true,
			cells:   codexCells(p.Pre().Uint(uint(width)), width),
		}
	}
	return out
}

// codexCells renders a value most-significant-bit first.
func codexCells(value uint64, width int) []bool {
	cells := make([]bool, width)
	for i := 0; i < width; i++ {
		shift := uint(width - 1 - i)
		if shift < 64 && value>>shift&1 == 1 {
			cells[i] = true
		}
	}
	return cells
}

// codexValue reads a field back. It exists for the round-trip test: an ordered
// renderer's promise is that the number can be recovered, and the honest way
// to check that is to recover it.
func codexValue(cells []bool) uint64 {
	var v uint64
	for _, on := range cells {
		v = v<<1 | boolBit(on)
	}
	return v
}

func boolBit(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// ghostAlpha is how strongly an unset cell is drawn.
//
// Drawing the empty cells at all is what makes these renderers readable. With
// only the set bits shown, a small version lights a handful of cells at one end
// of its field and the rest of the mark is blank: there is no visible grid, so
// there is nothing to count against and no way to see which bit position a
// mark belongs to. The ghosts turn the mark into a form with boxes to tick.
const ghostAlpha = 36

// fade returns a colour at the given alpha, premultiplied as image/color
// expects.
func fade(c color.Color, alpha uint8) color.RGBA {
	r, g, b, _ := c.RGBA()
	a := uint32(alpha)
	return color.RGBA{
		R: uint8(r * a / 0xffff >> 8),
		G: uint8(g * a / 0xffff >> 8),
		B: uint8(b * a / 0xffff >> 8),
		A: alpha,
	}
}

// codexInk maps a field to a palette slot. Major, minor and patch each get
// their own, so the eye can find the field it wants without counting.
func codexInk(field int) int {
	if field >= 3 {
		return 2
	}
	return field
}

// dialRenderer gives each component a quadrant, and writes its bits as radial
// bands. The separator is the angle: the spokes at the quadrant boundaries are
// the dots of the version string.
//
// Reading order is clockwise from noon, the way a clock is read: major in the
// first quadrant, then minor, patch and prerelease. Within a quadrant the
// outermost band is the most significant bit, so a larger number reaches
// further out.
type dialRenderer struct{}

func (dialRenderer) Name() string { return "dial" }

func (dialRenderer) Caps() Caps { return Caps{Ordered: true} }

func (dialRenderer) Render(ctx *Context) error {
	p := ctx.Params
	cx, cy := ctx.Box.Center()
	outer := ctx.Box.Min() / 2

	width := codexWidth(p.Version())
	fields := codexFields(p, width)

	// The hub is where the spokes meet; the bands start outside it.
	hub := outer * 0.15
	inner := outer * 0.24
	// A gap on each side of a quadrant boundary, so the separator reads as a
	// separator rather than as two touching fields.
	const quadrantGap = 0.06

	bandStep := (outer - inner) / float64(width)

	for f, field := range fields {
		if !field.present {
			continue
		}
		// Quadrants run clockwise from noon. In canvas coordinates y grows
		// downwards, so increasing angle is already clockwise.
		base := -math.Pi/2 + math.Pi/2*float64(f)
		a0 := base + math.Pi/2*quadrantGap
		a1 := base + math.Pi/2*(1-quadrantGap)

		ink := ctx.Palette.Ink(codexInk(f))

		// The grid first, then the bits on top of it.
		for i := range field.cells {
			band := width - 1 - i
			r0 := inner + bandStep*float64(band)
			canvas.AnnularSector(ctx.Canvas, cx, cy, r0, r0+bandStep*0.82, a0, a1)
		}
		ctx.Canvas.Fill(fade(ink, ghostAlpha))

		any := false
		for i, on := range field.cells {
			if !on {
				continue
			}
			any = true
			// Cell 0 is the most significant bit and sits outermost.
			band := width - 1 - i
			r0 := inner + bandStep*float64(band)
			canvas.AnnularSector(ctx.Canvas, cx, cy, r0, r0+bandStep*0.82, a0, a1)
		}
		if any {
			ctx.Canvas.Fill(ink)
		}
	}

	// The spokes, drawn last so they sit on top of the bands: these are the
	// punctuation.
	for f := 0; f < codexFieldCount; f++ {
		a := -math.Pi/2 + math.Pi/2*float64(f)
		ctx.Canvas.MoveTo(cx+hub*math.Cos(a), cy+hub*math.Sin(a))
		ctx.Canvas.LineTo(cx+outer*math.Cos(a), cy+outer*math.Sin(a))
	}
	ctx.Canvas.Stroke(ctx.Palette.Ink(0), outer*0.016)

	circle(ctx.Canvas, cx, cy, hub*0.6)
	ctx.Canvas.Fill(ctx.Palette.Ink(0))

	MarkPrerelease(ctx)
	return nil
}

// orbitRenderer gives each component a ring, and writes its bits as cells
// around it. The separator is the radius: crossing from one ring to the next
// is the dot.
//
// Major is innermost, so the version reads outwards in the order it is
// written. Each ring starts at noon and runs clockwise, most significant bit
// first; a tick at noon marks where to start reading.
type orbitRenderer struct{}

func (orbitRenderer) Name() string { return "orbit" }

func (orbitRenderer) Caps() Caps { return Caps{Ordered: true} }

func (orbitRenderer) Render(ctx *Context) error {
	p := ctx.Params
	cx, cy := ctx.Box.Center()
	outer := ctx.Box.Min() / 2

	width := codexWidth(p.Version())
	fields := codexFields(p, width)

	// A hole in the middle, then one ring per field.
	hole := outer * 0.22
	ringStep := (outer - hole) / codexFieldCount
	// A share of each ring left blank, so the radius reads as a separator.
	const ringGap = 0.22
	// A share of each cell left blank, so neighbouring bits stay countable.
	const cellGap = 0.12

	for f, field := range fields {
		if !field.present {
			continue
		}
		r0 := hole + ringStep*float64(f)
		r1 := r0 + ringStep*(1-ringGap)

		step := 2 * math.Pi / float64(width)
		ink := ctx.Palette.Ink(codexInk(f))

		for i := range field.cells {
			a0 := -math.Pi/2 + step*float64(i)
			canvas.AnnularSector(ctx.Canvas, cx, cy, r0, r1, a0, a0+step*(1-cellGap))
		}
		ctx.Canvas.Fill(fade(ink, ghostAlpha))

		any := false
		for i, on := range field.cells {
			if !on {
				continue
			}
			any = true
			// Cell 0 is the most significant bit and starts at noon.
			a0 := -math.Pi/2 + step*float64(i)
			canvas.AnnularSector(ctx.Canvas, cx, cy, r0, r1, a0, a0+step*(1-cellGap))
		}
		if any {
			ctx.Canvas.Fill(ink)
		}
	}

	// The noon tick: without it a ring of bits has no beginning and the mark
	// is decorative rather than readable.
	ctx.Canvas.MoveTo(cx, cy-hole*0.55)
	ctx.Canvas.LineTo(cx, cy-outer)
	ctx.Canvas.Stroke(ctx.Palette.Ink(0), outer*0.02)

	MarkPrerelease(ctx)
	return nil
}
