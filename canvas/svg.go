package canvas

import (
	"image/color"
	"strconv"
	"strings"
)

// SVG is a vector canvas. It builds markup directly with no dependencies, and
// formats every coordinate to a fixed number of decimals so that the same
// input always yields byte-identical output.
type SVG struct {
	w, h       float64
	cols, rows int
	body       strings.Builder
	path       Path
}

var _ Canvas = (*SVG)(nil)

// NewSVG returns a canvas of the given size in user units.
func NewSVG(w, h float64) *SVG { return &SVG{w: w, h: h} }

// Size implements Canvas.
func (s *SVG) Size() (float64, float64) { return s.w, s.h }

// SetGrid implements Canvas.
func (s *SVG) SetGrid(cols, rows int) { s.cols, s.rows = cols, rows }

// MoveTo implements Canvas.
func (s *SVG) MoveTo(x, y float64) { s.path.MoveTo(x, y) }

// LineTo implements Canvas.
func (s *SVG) LineTo(x, y float64) { s.path.LineTo(x, y) }

// CubicTo implements Canvas.
func (s *SVG) CubicTo(x1, y1, x2, y2, x, y float64) { s.path.CubicTo(x1, y1, x2, y2, x, y) }

// Close implements Canvas.
func (s *SVG) Close() { s.path.Close() }

// Background implements Canvas.
func (s *SVG) Background(col color.Color) {
	if _, _, _, a := col.RGBA(); a == 0 {
		return
	}
	s.write("  <rect width=\"", num(s.w), "\" height=\"", num(s.h),
		"\" fill=\"", hex(col), "\"/>\n")
}

// Fill implements Canvas.
func (s *SVG) Fill(col color.Color) {
	defer s.path.Reset()
	d := s.pathData()
	if d == "" {
		return
	}
	s.write("  <path d=\"", d, "\" fill=\"", hex(col), "\"")
	if o := opacity(col); o != "" {
		s.write(" fill-opacity=\"", o, "\"")
	}
	s.body.WriteString("/>\n")
}

// Stroke implements Canvas.
func (s *SVG) Stroke(col color.Color, width float64) {
	defer s.path.Reset()
	d := s.pathData()
	if d == "" || width <= 0 {
		return
	}
	s.write("  <path d=\"", d, "\" fill=\"none\" stroke=\"", hex(col),
		"\" stroke-width=\"", num(width),
		"\" stroke-linecap=\"round\" stroke-linejoin=\"round\"")
	if o := opacity(col); o != "" {
		s.write(" stroke-opacity=\"", o, "\"")
	}
	s.body.WriteString("/>\n")
}

// write appends the pieces of one element to the document.
//
// The markup is assembled from parts rather than from a format string because
// fmt would be the single heaviest import in the library: it pulls in
// reflection, which on a small target is a larger download than everything
// this package does put together. Every value here is already a string by the
// time it arrives, so the format string was never doing any work.
func (s *SVG) write(parts ...string) {
	for _, p := range parts {
		s.body.WriteString(p)
	}
}

func (s *SVG) pathData() string {
	var b strings.Builder
	for _, sub := range s.path.subs {
		if len(sub.segs) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("M " + num(sub.start.X) + " " + num(sub.start.Y))
		for _, seg := range sub.segs {
			switch seg.kind {
			case segLine:
				b.WriteString(" L " + num(seg.p3.X) + " " + num(seg.p3.Y))
			case segCubic:
				b.WriteString(" C " + num(seg.p1.X) + " " + num(seg.p1.Y) +
					" " + num(seg.p2.X) + " " + num(seg.p2.Y) +
					" " + num(seg.p3.X) + " " + num(seg.p3.Y))
			}
		}
		if sub.closed {
			b.WriteString(" Z")
		}
	}
	return b.String()
}

// Cell implements Canvas by drawing a monospace glyph in the declared grid.
func (s *SVG) Cell(col, row int, r rune, fg, bg color.Color) {
	if s.cols <= 0 || s.rows <= 0 {
		return
	}
	cw, ch := s.w/float64(s.cols), s.h/float64(s.rows)
	x, y := float64(col)*cw, float64(row)*ch
	if _, _, _, a := bg.RGBA(); a > 0 {
		s.write("  <rect x=\"", num(x), "\" y=\"", num(y),
			"\" width=\"", num(cw), "\" height=\"", num(ch),
			"\" fill=\"", hex(bg), "\"/>\n")
	}
	if r == ' ' || r == 0 {
		return
	}
	s.write("  <text x=\"", num(x+cw/2), "\" y=\"", num(y+ch/2),
		"\" font-family=\"monospace\" font-size=\"", num(ch*0.8),
		"\" text-anchor=\"middle\" dominant-baseline=\"central\" fill=\"", hex(fg),
		"\">", escapeXML(string(r)), "</text>\n")
}

// Bytes renders the finished document.
func (s *SVG) Bytes() []byte {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="`)
	b.WriteString(num(s.w))
	b.WriteString(`" height="`)
	b.WriteString(num(s.h))
	b.WriteString(`" viewBox="0 0 `)
	b.WriteString(num(s.w))
	b.WriteByte(' ')
	b.WriteString(num(s.h))
	b.WriteString("\">\n")
	b.WriteString(s.body.String())
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// num formats a coordinate. Fixed precision with trailing zeroes trimmed keeps
// output stable across platforms and free of "-0".
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	if s == "-0" {
		return "0"
	}
	return s
}

func hex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	out := []byte("#000000")
	const digits = "0123456789abcdef"
	for i, v := range [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)} {
		out[1+i*2] = digits[v>>4]
		out[2+i*2] = digits[v&0xf]
	}
	return string(out)
}

// opacity returns the fill-opacity attribute value for partly transparent
// colours, or the empty string when the colour is opaque.
func opacity(c color.Color) string {
	if c == nil {
		return ""
	}
	_, _, _, a := c.RGBA()
	if a >= 0xffff {
		return ""
	}
	return num(float64(a) / 0xffff)
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}
