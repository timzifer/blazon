package blazon

import "image/color"

func init() {
	Register(bishopRenderer{})
}

// bishopRenderer draws the drunken bishop walk that OpenSSH uses for key
// randomart.
//
// It is here because the problem it solves is exactly the one this library
// exists for — comparing two fingerprints at a glance — and it has been solved
// in the field for years. The algorithm and the character ramp are kept
// compatible with OpenSSH's so that the marks look familiar to anyone who has
// ever confirmed a host key.
//
// A bishop moves diagonally, two bits per step choosing the direction, and
// bounces off the edges of a 17×9 field. Each square counts its visits, and
// the count selects a character from a ramp of increasing density. Everything
// is integer arithmetic on a grid, so the output is byte-exact.
//
// Amplified parameters: the walk length, and — through the palette — hue.
type bishopRenderer struct{}

func (bishopRenderer) Name() string { return "bishop" }

// Bishop field size, as in OpenSSH, plus one reserved row.
const (
	bishopCols = 17
	bishopRows = 9
	// bishopMarkerRow sits below the field and carries the prerelease marker.
	// It is reserved rather than shared so the marker can never overwrite a
	// square the walk visited.
	bishopMarkerRow = bishopRows
	bishopGridRows  = bishopRows + 1
)

func (bishopRenderer) Caps() Caps {
	return Caps{TextOnly: true, GridCols: bishopCols, GridRows: bishopGridRows, ByteExact: true}
}

// bishopRamp maps a visit count to a character. The two extra entries are the
// start and end markers, which are written over the ramp.
const bishopRamp = " .o+=*BOX@%&#/^"

func (bishopRenderer) Render(ctx *Context) error {
	p := ctx.Params
	ctx.Canvas.SetGrid(bishopCols, bishopGridRows)

	counts := make([]int, bishopCols*bishopRows)
	x, y := bishopCols/2, bishopRows/2
	start := y*bishopCols + x

	// Walk length is amplified: a patch bump changes how long the bishop
	// staggers, which redistributes the whole field rather than nudging one
	// square.
	steps := p.Amplified().IntRange(96, 160)

	// The walk is split between two streams. The opening moves come from the
	// variant stream, so every version sharing a major and minor starts by
	// staggering the same way and its field keeps a familiar centre of mass;
	// the rest comes from the detail stream, so a patch bump still rewrites
	// most of the path. Drawing the whole walk from the detail stream would
	// make every version equally unrelated to every other, and the family
	// policy would be a claim the marks do not support.
	opening := steps * 2 / 5
	family := p.Variant()
	walk := p.Detail()
	for i := 0; i < steps; i++ {
		src := walk
		if i < opening {
			src = family
		}
		bits := src.Uint(2)
		if bits&1 == 0 {
			x--
		} else {
			x++
		}
		if bits&2 == 0 {
			y--
		} else {
			y++
		}
		x = clampInt(x, 0, bishopCols-1)
		y = clampInt(y, 0, bishopRows-1)
		counts[y*bishopCols+x]++
	}
	end := y*bishopCols + x

	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	for i, c := range counts {
		col, row := i%bishopCols, i/bishopCols
		ch := bishopChar(c, i, start, end)
		ctx.Canvas.Cell(col, row, ch, ctx.Palette.Ink(inkForCount(c, maxCount)), color.Transparent)
	}

	MarkPrereleaseCells(ctx, bishopCols, bishopMarkerRow)
	return nil
}

// bishopChar picks the glyph for a square. Start and end markers override the
// density ramp, exactly as OpenSSH does, because knowing where the walk began
// and ended is what lets two fields be compared structurally rather than as
// noise.
func bishopChar(count, idx, start, end int) rune {
	switch idx {
	case start:
		return 'S'
	case end:
		return 'E'
	}
	if count <= 0 {
		return ' '
	}
	if count >= len(bishopRamp) {
		count = len(bishopRamp) - 1
	}
	return rune(bishopRamp[count])
}

// inkForCount spreads the palette across the density range, so the field has
// tonal structure rather than one flat colour.
func inkForCount(count, max int) int {
	if max <= 1 || count <= 0 {
		return 0
	}
	switch {
	case count*3 >= max*2:
		return 2
	case count*3 >= max:
		return 1
	}
	return 0
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
