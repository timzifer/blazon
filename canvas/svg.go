package canvas

import (
	"fmt"
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
	fmt.Fprintf(&s.body, "  <rect width=\"%s\" height=\"%s\" fill=\"%s\"/>\n",
		num(s.w), num(s.h), hex(col))
}

// Fill implements Canvas.
func (s *SVG) Fill(col color.Color) {
	defer s.path.Reset()
	d := s.pathData()
	if d == "" {
		return
	}
	fmt.Fprintf(&s.body, "  <path d=\"%s\" fill=\"%s\"", d, hex(col))
	if o := opacity(col); o != "" {
		fmt.Fprintf(&s.body, " fill-opacity=\"%s\"", o)
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
	fmt.Fprintf(&s.body,
		"  <path d=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"%s\""+
			" stroke-linecap=\"round\" stroke-linejoin=\"round\"",
		d, hex(col), num(width))
	if o := opacity(col); o != "" {
		fmt.Fprintf(&s.body, " stroke-opacity=\"%s\"", o)
	}
	s.body.WriteString("/>\n")
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
		fmt.Fprintf(&s.body, "  <rect x=\"%s\" y=\"%s\" width=\"%s\" height=\"%s\" fill=\"%s\"/>\n",
			num(x), num(y), num(cw), num(ch), hex(bg))
	}
	if r == ' ' || r == 0 {
		return
	}
	fmt.Fprintf(&s.body,
		"  <text x=\"%s\" y=\"%s\" font-family=\"monospace\" font-size=\"%s\""+
			" text-anchor=\"middle\" dominant-baseline=\"central\" fill=\"%s\">%s</text>\n",
		num(x+cw/2), num(y+ch/2), num(ch*0.8), hex(fg), escapeXML(string(r)))
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
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
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
