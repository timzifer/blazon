package canvas

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Raster is a pixel canvas backed by an anti-aliased scanline rasterizer.
// It shares the Path model with SVG, so a renderer draws once and both back
// ends agree on the geometry.
//
// Supersampling is applied because the rasterizer anti-aliases coverage but
// not the thin, densely packed strokes the flow-field renderer produces; at
// 1:1 those alias into moiré.
type Raster struct {
	w, h       int
	ss         int // supersampling factor
	img        *image.RGBA
	cols, rows int
	path       Path
}

var _ Canvas = (*Raster)(nil)

// DefaultSupersample is the oversampling factor used by NewRaster.
const DefaultSupersample = 3

// NewRaster returns a canvas of w×h pixels.
func NewRaster(w, h int) *Raster { return NewRasterSS(w, h, DefaultSupersample) }

// NewRasterSS returns a canvas of w×h pixels rendered at ss× internally and
// box-filtered down by Image.
func NewRasterSS(w, h, ss int) *Raster {
	if ss < 1 {
		ss = 1
	}
	return &Raster{
		w:   w,
		h:   h,
		ss:  ss,
		img: image.NewRGBA(image.Rect(0, 0, w*ss, h*ss)),
	}
}

// Size implements Canvas. The reported size is in output pixels; the
// supersampling factor is an internal detail a renderer never sees.
func (r *Raster) Size() (float64, float64) { return float64(r.w), float64(r.h) }

// SetGrid implements Canvas.
func (r *Raster) SetGrid(cols, rows int) { r.cols, r.rows = cols, rows }

// MoveTo implements Canvas.
func (r *Raster) MoveTo(x, y float64) { r.path.MoveTo(x, y) }

// LineTo implements Canvas.
func (r *Raster) LineTo(x, y float64) { r.path.LineTo(x, y) }

// CubicTo implements Canvas.
func (r *Raster) CubicTo(x1, y1, x2, y2, x, y float64) { r.path.CubicTo(x1, y1, x2, y2, x, y) }

// Close implements Canvas.
func (r *Raster) Close() { r.path.Close() }

// Background implements Canvas.
func (r *Raster) Background(col color.Color) {
	if _, _, _, a := col.RGBA(); a == 0 {
		return
	}
	draw.Draw(r.img, r.img.Bounds(), image.NewUniform(col), image.Point{}, draw.Src)
}

// Fill implements Canvas.
func (r *Raster) Fill(col color.Color) {
	defer r.path.Reset()
	if r.path.Empty() {
		return
	}
	ras := r.newRasterizer()
	s := float64(r.ss)
	for _, sub := range r.path.subs {
		if len(sub.segs) == 0 {
			continue
		}
		ras.MoveTo(float32(sub.start.X*s), float32(sub.start.Y*s))
		for _, seg := range sub.segs {
			switch seg.kind {
			case segLine:
				ras.LineTo(float32(seg.p3.X*s), float32(seg.p3.Y*s))
			case segCubic:
				ras.CubeTo(
					float32(seg.p1.X*s), float32(seg.p1.Y*s),
					float32(seg.p2.X*s), float32(seg.p2.Y*s),
					float32(seg.p3.X*s), float32(seg.p3.Y*s))
			}
		}
		ras.ClosePath()
	}
	r.draw(ras, col)
}

// Stroke implements Canvas. The path is flattened and converted to fillable
// outline polygons; see StrokeOutline for why no real stroker is needed.
func (r *Raster) Stroke(col color.Color, width float64) {
	defer r.path.Reset()
	if r.path.Empty() || width <= 0 {
		return
	}
	polys := StrokeOutline(r.path.Flatten(), width)
	if len(polys) == 0 {
		return
	}
	ras := r.newRasterizer()
	s := float64(r.ss)
	for _, poly := range polys {
		if len(poly) < 3 {
			continue
		}
		ras.MoveTo(float32(poly[0].X*s), float32(poly[0].Y*s))
		for _, p := range poly[1:] {
			ras.LineTo(float32(p.X*s), float32(p.Y*s))
		}
		ras.ClosePath()
	}
	r.draw(ras, col)
}

func (r *Raster) newRasterizer() *vector.Rasterizer {
	return vector.NewRasterizer(r.w*r.ss, r.h*r.ss)
}

func (r *Raster) draw(ras *vector.Rasterizer, col color.Color) {
	ras.Draw(r.img, r.img.Bounds(), image.NewUniform(col), image.Point{})
}

// Cell implements Canvas using a bitmap font. It exists so that the character
// renderers still produce an image for the gallery, not because anyone should
// prefer a PNG of a terminal mark over the terminal itself.
func (r *Raster) Cell(col, row int, ch rune, fg, bg color.Color) {
	if r.cols <= 0 || r.rows <= 0 {
		return
	}
	cw := float64(r.w*r.ss) / float64(r.cols)
	chh := float64(r.h*r.ss) / float64(r.rows)
	x0, y0 := float64(col)*cw, float64(row)*chh

	if _, _, _, a := bg.RGBA(); a > 0 {
		rect := image.Rect(int(x0), int(y0), int(math.Ceil(x0+cw)), int(math.Ceil(y0+chh)))
		draw.Draw(r.img, rect.Intersect(r.img.Bounds()), image.NewUniform(bg), image.Point{}, draw.Src)
	}
	if ch == ' ' || ch == 0 {
		return
	}

	face := basicfont.Face7x13
	adv, ok := face.GlyphAdvance(ch)
	if !ok {
		return
	}
	// Centre the glyph in its cell. The bitmap font has one fixed size, so the
	// cell is padded rather than the glyph scaled.
	gx := x0 + (cw-float64(adv)/64)/2
	gy := y0 + (chh+float64(face.Metrics().CapHeight)/64)/2

	d := font.Drawer{
		Dst:  r.img,
		Src:  image.NewUniform(fg),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(gx * 64),
			Y: fixed.Int26_6(gy * 64),
		},
	}
	d.DrawString(string(ch))
}

// Image returns the finished image, box-filtered down from the supersampled
// buffer.
func (r *Raster) Image() *image.RGBA {
	if r.ss == 1 {
		return r.img
	}
	out := image.NewRGBA(image.Rect(0, 0, r.w, r.h))
	n := uint32(r.ss * r.ss)
	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			var sr, sg, sb, sa uint32
			for dy := 0; dy < r.ss; dy++ {
				for dx := 0; dx < r.ss; dx++ {
					i := r.img.PixOffset(x*r.ss+dx, y*r.ss+dy)
					p := r.img.Pix[i : i+4 : i+4]
					sr += uint32(p[0])
					sg += uint32(p[1])
					sb += uint32(p[2])
					sa += uint32(p[3])
				}
			}
			i := out.PixOffset(x, y)
			p := out.Pix[i : i+4 : i+4]
			p[0] = uint8(sr / n)
			p[1] = uint8(sg / n)
			p[2] = uint8(sb / n)
			p[3] = uint8(sa / n)
		}
	}
	return out
}
