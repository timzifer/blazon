package canvas

import (
	"image"
	"image/color"
	"math"
)

// filler is an anti-aliased scanline rasteriser.
//
// It is written out rather than pulled in for the same reason the noise field
// is: this library's whole claim is that a version always produces the same
// mark, and a rasteriser it does not own is a rasteriser that can improve.
// A dependency that sharpened its anti-aliasing by a fraction of a pixel would
// silently change every mark ever generated, and the gallery check would
// report it as a regression in this repository's own code.
//
// The method is signed-area accumulation. Each edge deposits, per pixel, the
// signed area it sweeps in that pixel's column; a running sum along the row
// then gives coverage. It is exact for straight edges — no supersampling, no
// sorting of intersections — and the coverage it produces is the true area of
// the pixel covered by the polygon.
type filler struct {
	w, h int
	// area is one wider than the image: an edge landing on the right-hand
	// boundary of the last pixel deposits its remainder one column further.
	area []float64
}

func newFiller(w, h int) *filler {
	return &filler{w: w, h: h, area: make([]float64, (w+1)*h)}
}

func (f *filler) reset() {
	for i := range f.area {
		f.area[i] = 0
	}
}

// addPolygon accumulates a closed polygon. The caller need not repeat the
// first point.
func (f *filler) addPolygon(pts []Point) {
	if len(pts) < 2 {
		return
	}
	for i := range pts {
		f.addEdge(pts[i], pts[(i+1)%len(pts)])
	}
}

// addEdge accumulates one straight edge.
//
// The edge is walked scanline by scanline, and within a scanline column by
// column. Direction is folded into a sign, so an edge going up cancels one
// going down — which is what makes the non-zero rule fall out of the
// accumulation instead of needing a separate winding pass.
func (f *filler) addEdge(a, b Point) {
	if a.Y == b.Y {
		// Horizontal edges sweep no area.
		return
	}
	dir := 1.0
	if a.Y > b.Y {
		a, b = b, a
		dir = -1
	}

	// Clip to the vertical extent of the image, interpolating x so the
	// geometry stays true rather than being folded onto the boundary.
	dxdy := (b.X - a.X) / (b.Y - a.Y)
	if a.Y < 0 {
		a = Point{X: a.X + dxdy*(0-a.Y), Y: 0}
	}
	if b.Y > float64(f.h) {
		b = Point{X: a.X + dxdy*(float64(f.h)-a.Y), Y: float64(f.h)}
	}
	if a.Y >= b.Y {
		return
	}

	x := a.X
	y0i := int(math.Floor(a.Y))
	y1i := int(math.Ceil(b.Y))
	if y0i < 0 {
		y0i = 0
	}
	if y1i > f.h {
		y1i = f.h
	}

	for y := y0i; y < y1i; y++ {
		yTop := math.Max(float64(y), a.Y)
		yBot := math.Min(float64(y+1), b.Y)
		dy := yBot - yTop
		if dy <= 0 {
			continue
		}
		xNext := x + dy*dxdy
		f.addSpan(y, x, xNext, dy*dir)
		x = xNext
	}
}

// addSpan deposits the area swept between two x positions within one scanline.
func (f *filler) addSpan(y int, x0, x1, d float64) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	// Clamping sideways is safe in a way clamping vertically is not: the
	// accumulation only cares about where an edge crosses a column boundary,
	// and everything left of the image contributes to the very first column
	// regardless.
	limit := float64(f.w)
	if x1 <= 0 {
		x0, x1 = 0, 0
	} else if x0 >= limit {
		x0, x1 = limit, limit
	} else {
		if x0 < 0 {
			x0 = 0
		}
		if x1 > limit {
			x1 = limit
		}
	}

	row := f.area[y*(f.w+1) : (y+1)*(f.w+1)]
	x0i := int(x0)
	if x0i >= f.w {
		x0i = f.w - 1
	}
	x1i := int(math.Ceil(x1))
	if x1i > f.w {
		x1i = f.w
	}

	if x1i <= x0i+1 {
		// The span stays inside one column: split the area between that
		// column and the next by where the span's midpoint falls.
		xmf := 0.5*(x0+x1) - float64(x0i)
		row[x0i] += d * (1 - xmf)
		row[x0i+1] += d * xmf
		return
	}

	// The span crosses column boundaries. Each column takes the share of the
	// area the edge sweeps while inside it.
	s := 1 / (x1 - x0)
	x0f := x0 - float64(x0i)
	oneMinusX0f := 1 - x0f
	a0 := 0.5 * s * oneMinusX0f * oneMinusX0f
	x1f := x1 - float64(x1i) + 1
	am := 0.5 * s * x1f * x1f

	row[x0i] += d * a0
	if x1i == x0i+2 {
		row[x0i+1] += d * (1 - a0 - am)
	} else {
		a1 := s * (1.5 - x0f)
		row[x0i+1] += d * (a1 - a0)
		for xi := x0i + 2; xi < x1i-1; xi++ {
			row[xi] += d * s
		}
		a2 := a1 + float64(x1i-x0i-3)*s
		row[x1i-1] += d * (1 - a2 - am)
	}
	row[x1i] += d * am
}

// blit composites the accumulated coverage onto dst in the given colour.
//
// Coverage is the absolute value of the running sum, clamped to one. Two
// contours wound the same way therefore union, and a contour wound against its
// enclosing one cancels to zero and leaves a hole — the non-zero fill rule,
// which is the only rule this canvas offers.
func (f *filler) blit(dst *image.RGBA, col color.Color) {
	cr, cg, cb, ca := col.RGBA()
	if ca == 0 {
		return
	}

	b := dst.Bounds()
	for y := 0; y < f.h; y++ {
		if y >= b.Dy() {
			break
		}
		row := f.area[y*(f.w+1) : (y+1)*(f.w+1)]
		acc := 0.0
		for x := 0; x < f.w; x++ {
			acc += row[x]
			cov := math.Abs(acc)
			if cov <= 0.0001 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			if x >= b.Dx() {
				break
			}
			alpha := uint32(cov*float64(ca) + 0.5)
			if alpha == 0 {
				continue
			}
			blendPixel(dst, x, y, cr, cg, cb, ca, alpha)
		}
	}
}

// blendPixel composites one source pixel with coverage-scaled alpha over the
// destination, in premultiplied space.
func blendPixel(dst *image.RGBA, x, y int, sr, sg, sb, sa, alpha uint32) {
	i := dst.PixOffset(x+dst.Rect.Min.X, y+dst.Rect.Min.Y)
	p := dst.Pix[i : i+4 : i+4]

	// Scale the premultiplied source by the coverage fraction alpha/sa.
	if sa == 0 {
		return
	}
	r := sr * alpha / sa
	g := sg * alpha / sa
	bl := sb * alpha / sa

	inv := 0xffff - alpha
	p[0] = uint8((r + uint32(p[0])*0x101*inv/0xffff) >> 8)
	p[1] = uint8((g + uint32(p[1])*0x101*inv/0xffff) >> 8)
	p[2] = uint8((bl + uint32(p[2])*0x101*inv/0xffff) >> 8)
	p[3] = uint8((alpha + uint32(p[3])*0x101*inv/0xffff) >> 8)
}
