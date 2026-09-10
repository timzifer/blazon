package canvas

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// Raster is a pixel canvas backed by an anti-aliased scanline rasteriser.
// It shares the Path model with SVG, so a renderer draws once and both back
// ends agree on the geometry.
//
// Supersampling is applied because the rasteriser anti-aliases coverage but
// not the thin, densely packed strokes the flow-field renderer produces; at
// 1:1 those alias into moiré.
type Raster struct {
	w, h       int
	ss         int // supersampling factor
	img        *image.RGBA
	cols, rows int
	path       Path
	fill       *filler
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
		w:    w,
		h:    h,
		ss:   ss,
		img:  image.NewRGBA(image.Rect(0, 0, w*ss, h*ss)),
		fill: newFiller(w*ss, h*ss),
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
	r.rasterise(r.path.Flatten(), col)
}

// Stroke implements Canvas. The path is flattened and converted to fillable
// outline polygons; see StrokeOutline for why no real stroker is needed.
func (r *Raster) Stroke(col color.Color, width float64) {
	defer r.path.Reset()
	if r.path.Empty() || width <= 0 {
		return
	}
	r.rasterise(StrokeOutline(r.path.Flatten(), width), col)
}

// rasterise accumulates polygons at the supersampled scale and composites
// them in one pass, so overlapping pieces of the same shape merge rather than
// compositing over each other and darkening at the seams.
func (r *Raster) rasterise(polys [][]Point, col color.Color) {
	if len(polys) == 0 {
		return
	}
	s := float64(r.ss)
	r.fill.reset()
	scaled := make([]Point, 0, 64)
	for _, poly := range polys {
		if len(poly) < 2 {
			continue
		}
		scaled = scaled[:0]
		for _, p := range poly {
			scaled = append(scaled, Point{X: p.X * s, Y: p.Y * s})
		}
		r.fill.addPolygon(scaled)
	}
	r.fill.blit(r.img, col)
}

// Cell implements Canvas using the built-in bitmap font. It exists so that the
// character renderers still produce an image for the gallery, not because
// anyone should prefer a PNG of a terminal mark over the terminal itself.
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

	// The largest whole scale that still fits the cell. Whole scales only:
	// a bitmap font resampled to a fractional size loses the strokes that
	// distinguish one randomart character from the next.
	scale := int(math.Min(cw/GlyphW, chh/GlyphH))
	if scale < 1 {
		scale = 1
	}
	gx := x0 + (cw-float64(GlyphW*scale))/2
	gy := y0 + (chh-float64(GlyphH*scale))/2
	DrawGlyph(r.img, int(gx), int(gy), ch, fg, scale)
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
