package canvas

import (
	"image/color"
	"strconv"
	"strings"
)

// ANSI renders to a block of terminal text.
//
// Vector drawing is supported by rasterising into a buffer twice as tall as
// the character grid and emitting the upper half-block character ▀ per cell:
// the glyph's foreground paints the top pixel and its background the bottom
// one, so a character cell carries two square pixels rather than one oblong
// one. That is what makes a terminal mark look like a mark instead of like
// ASCII art.
//
// Character output from Cell is kept separately and wins over the raster, so a
// renderer that draws glyphs — the randomart field, for instance — gets what
// it asked for.
type ANSI struct {
	cols, rows int
	raster     *Raster
	glyphs     []glyphCell
	trueColor  bool
}

type glyphCell struct {
	set    bool
	r      rune
	fg, bg color.Color
}

var _ Canvas = (*ANSI)(nil)

// ansiSupersample oversamples the tiny terminal raster. The buffer is only a
// few hundred pixels across, so anti-aliasing matters more here than anywhere
// else.
const ansiSupersample = 4

// NewANSI returns a canvas of cols×rows character cells. When trueColor is
// false the output falls back to a monochrome ASCII ramp, for terminals that
// do not accept 24-bit colour or for NO_COLOR environments.
func NewANSI(cols, rows int, trueColor bool) *ANSI {
	return &ANSI{
		cols:      cols,
		rows:      rows,
		raster:    NewRasterSS(cols, rows*2, ansiSupersample),
		glyphs:    make([]glyphCell, cols*rows),
		trueColor: trueColor,
	}
}

// Size reports the pixel grid: one unit per column, two per row.
func (a *ANSI) Size() (float64, float64) { return float64(a.cols), float64(a.rows * 2) }

// SetGrid implements Canvas. The character grid is fixed at construction, so
// this only checks that a renderer is not asking for a different one.
func (a *ANSI) SetGrid(cols, rows int) {
	if cols == a.cols && rows == a.rows {
		return
	}
	// Re-size to what the renderer asked for; the CLI constructs the canvas
	// before it knows which renderer will draw on it.
	a.cols, a.rows = cols, rows
	a.raster = NewRasterSS(cols, rows*2, ansiSupersample)
	a.glyphs = make([]glyphCell, cols*rows)
}

// Background implements Canvas.
func (a *ANSI) Background(col color.Color) { a.raster.Background(col) }

// MoveTo implements Canvas.
func (a *ANSI) MoveTo(x, y float64) { a.raster.MoveTo(x, y) }

// LineTo implements Canvas.
func (a *ANSI) LineTo(x, y float64) { a.raster.LineTo(x, y) }

// CubicTo implements Canvas.
func (a *ANSI) CubicTo(x1, y1, x2, y2, x, y float64) { a.raster.CubicTo(x1, y1, x2, y2, x, y) }

// Close implements Canvas.
func (a *ANSI) Close() { a.raster.Close() }

// Fill implements Canvas.
func (a *ANSI) Fill(col color.Color) { a.raster.Fill(col) }

// Stroke implements Canvas.
func (a *ANSI) Stroke(col color.Color, width float64) { a.raster.Stroke(col, width) }

// Cell implements Canvas by recording a glyph that overrides the raster.
func (a *ANSI) Cell(col, row int, r rune, fg, bg color.Color) {
	if col < 0 || row < 0 || col >= a.cols || row >= a.rows {
		return
	}
	a.glyphs[row*a.cols+col] = glyphCell{set: true, r: r, fg: fg, bg: bg}
}

// rampChars runs from lightest to darkest. It is used when colour is
// unavailable.
const rampChars = " .:-=+*#%@"

// String renders the block, one line per row, with a trailing newline.
func (a *ANSI) String() string {
	img := a.raster.Image()
	var b strings.Builder

	for row := 0; row < a.rows; row++ {
		for col := 0; col < a.cols; col++ {
			if g := a.glyphs[row*a.cols+col]; g.set {
				a.writeGlyph(&b, g)
				continue
			}
			top := img.RGBAAt(col, row*2)
			bottom := img.RGBAAt(col, row*2+1)
			if a.trueColor {
				writeSGR(&b, 38, top.R, top.G, top.B)
				writeSGR(&b, 48, bottom.R, bottom.G, bottom.B)
				b.WriteRune('▀')
				continue
			}
			b.WriteByte(rampByte(avgLuma(top, bottom)))
		}
		if a.trueColor {
			b.WriteString("\x1b[0m")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (a *ANSI) writeGlyph(b *strings.Builder, g glyphCell) {
	if !a.trueColor {
		b.WriteRune(g.r)
		return
	}
	fr, fg2, fb, _ := g.fg.RGBA()
	br, bg2, bb, ba := g.bg.RGBA()
	writeSGR(b, 38, uint8(fr>>8), uint8(fg2>>8), uint8(fb>>8))
	if ba > 0 {
		writeSGR(b, 48, uint8(br>>8), uint8(bg2>>8), uint8(bb>>8))
	}
	b.WriteRune(g.r)
}

// writeSGR emits one 24-bit colour escape: 38 sets the foreground, 48 the
// background.
//
// Assembled by hand rather than with fmt, which the library does not import —
// reflection is the heaviest thing a caller's binary could inherit from a
// package that draws pictures, and three integers do not need it.
func writeSGR(b *strings.Builder, ground int, r, g, bl uint8) {
	b.WriteString("\x1b[")
	b.WriteString(strconv.Itoa(ground))
	b.WriteString(";2;")
	b.WriteString(strconv.Itoa(int(r)))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(int(g)))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(int(bl)))
	b.WriteByte('m')
}

// avgLuma is the mean relative luminance of the two pixels a cell covers,
// composited over white so an unpainted ground reads as light.
func avgLuma(a, b color.RGBA) float64 {
	l := func(c color.RGBA) float64 {
		af := float64(c.A) / 255
		r := float64(c.R)/255 + (1 - af)
		g := float64(c.G)/255 + (1 - af)
		bl := float64(c.B)/255 + (1 - af)
		return 0.2126*r + 0.7152*g + 0.0722*bl
	}
	return (l(a) + l(b)) / 2
}

func rampByte(luma float64) byte {
	// Dark ink maps to the dense end of the ramp.
	i := int((1 - luma) * float64(len(rampChars)-1))
	if i < 0 {
		i = 0
	}
	if i >= len(rampChars) {
		i = len(rampChars) - 1
	}
	return rampChars[i]
}
