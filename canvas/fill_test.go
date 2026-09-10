package canvas

import (
	"image/color"
	"math"
	"testing"
)

// coverage sums the alpha of a rendered shape. Because the rasteriser computes
// true area coverage, the sum has to equal the geometric area of the shape —
// which turns "is the anti-aliasing correct" into an arithmetic question
// rather than a matter of taste.
func coverage(draw func(*Raster)) float64 {
	r := NewRasterSS(64, 64, 1)
	draw(r)
	img := r.Image()
	var sum float64
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum += float64(img.RGBAAt(x, y).A) / 255
		}
	}
	return sum
}

var black = color.RGBA{0, 0, 0, 255}

func TestFillAreaIsExact(t *testing.T) {
	cases := []struct {
		name string
		want float64
		draw func(*Raster)
	}{
		{
			// Deliberately off-grid on every side: a rasteriser that rounded
			// to whole pixels would be out by up to four.
			name: "fractional rectangle",
			want: 10.4 * 10.4,
			draw: func(r *Raster) {
				r.MoveTo(5.3, 5.3)
				r.LineTo(15.7, 5.3)
				r.LineTo(15.7, 15.7)
				r.LineTo(5.3, 15.7)
				r.Close()
				r.Fill(black)
			},
		},
		{
			// Narrower than one pixel: must still deposit its area rather
			// than disappearing or snapping to a full column.
			name: "sub-pixel sliver",
			want: 0.4 * 30,
			draw: func(r *Raster) {
				r.MoveTo(10, 10)
				r.LineTo(10.4, 10)
				r.LineTo(10.4, 40)
				r.LineTo(10, 40)
				r.Close()
				r.Fill(black)
			},
		},
		{
			name: "diagonal triangle",
			want: 30 * 30 / 2.0,
			draw: func(r *Raster) {
				r.MoveTo(10, 10)
				r.LineTo(40, 10)
				r.LineTo(10, 40)
				r.Close()
				r.Fill(black)
			},
		},
		{
			// Curves, and the shape the polar renderer actually draws.
			name: "annular sector",
			want: 0.5 * (20*20 - 10*10) / 2,
			draw: func(r *Raster) {
				AnnularSector(r, 32, 32, 10, 20, 0, 0.5)
				r.Fill(black)
			},
		},
		{
			name: "full circle",
			want: math.Pi * 12 * 12,
			draw: func(r *Raster) {
				MoveToCircle(r, 32, 32, 12)
				r.Fill(black)
			},
		},
	}

	for _, c := range cases {
		got := coverage(c.draw)
		if rel := math.Abs(got-c.want) / c.want; rel > 0.01 {
			t.Errorf("%s: covered %.3f, want %.3f (%.2f%% off)", c.name, got, c.want, rel*100)
		}
	}
}

// TestFillWindingCancels is the arithmetic form of the hole-punching rule: a
// reversed inner contour must remove exactly its own area.
func TestFillWindingCancels(t *testing.T) {
	got := coverage(func(r *Raster) {
		r.MoveTo(10, 10)
		r.LineTo(40, 10)
		r.LineTo(40, 40)
		r.LineTo(10, 40)
		r.Close()
		// Reversed.
		r.MoveTo(20, 20)
		r.LineTo(20, 30)
		r.LineTo(30, 30)
		r.LineTo(30, 20)
		r.Close()
		r.Fill(black)
	})
	want := 30*30 - 10*10.0
	if rel := math.Abs(got-want) / want; rel > 0.01 {
		t.Errorf("ring covered %.3f, want %.3f", got, want)
	}
}

// TestFillOverlapDoesNotDouble checks the clamp: two contours wound the same
// way union rather than accumulating past full coverage.
func TestFillOverlapDoesNotDouble(t *testing.T) {
	got := coverage(func(r *Raster) {
		r.MoveTo(10, 10)
		r.LineTo(30, 10)
		r.LineTo(30, 30)
		r.LineTo(10, 30)
		r.Close()
		r.MoveTo(20, 20)
		r.LineTo(40, 20)
		r.LineTo(40, 40)
		r.LineTo(20, 40)
		r.Close()
		r.Fill(black)
	})
	want := 2*20*20 - 10*10.0 // two squares less their overlap
	if rel := math.Abs(got-want) / want; rel > 0.01 {
		t.Errorf("overlapping squares covered %.3f, want %.3f", got, want)
	}
}

// TestFillClipsToTheCanvas checks that geometry running off the edge is
// clipped rather than wrapping, panicking, or folding onto the boundary.
func TestFillClipsToTheCanvas(t *testing.T) {
	got := coverage(func(r *Raster) {
		r.MoveTo(-100, -100)
		r.LineTo(32, -100)
		r.LineTo(32, 32)
		r.LineTo(-100, 32)
		r.Close()
		r.Fill(black)
	})
	want := 32 * 32.0
	if rel := math.Abs(got-want) / want; rel > 0.02 {
		t.Errorf("clipped rectangle covered %.3f, want %.3f", got, want)
	}
}

func TestFillIgnoresTransparentInk(t *testing.T) {
	got := coverage(func(r *Raster) {
		r.MoveTo(10, 10)
		r.LineTo(40, 10)
		r.LineTo(40, 40)
		r.Close()
		r.Fill(color.Transparent)
	})
	if got != 0 {
		t.Errorf("a transparent fill painted %.3f of coverage", got)
	}
}

// MoveToCircle appends a circle, used by the area test above. It lives here
// rather than in the package because only tests need a whole circle in one
// call.
func MoveToCircle(c Canvas, cx, cy, r float64) {
	const k = 0.5522847498307936
	kr := k * r
	c.MoveTo(cx+r, cy)
	c.CubicTo(cx+r, cy+kr, cx+kr, cy+r, cx, cy+r)
	c.CubicTo(cx-kr, cy+r, cx-r, cy+kr, cx-r, cy)
	c.CubicTo(cx-r, cy-kr, cx-kr, cy-r, cx, cy-r)
	c.CubicTo(cx+kr, cy-r, cx+r, cy-kr, cx+r, cy)
	c.Close()
}
