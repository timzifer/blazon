package phash

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// checker draws an n-square checkerboard, which gives the DCT a strong,
// predictable low-frequency signal.
func checker(size, n int, invert bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cell := size / n
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			on := ((x/cell)+(y/cell))%2 == 0
			if invert {
				on = !on
			}
			c := color.RGBA{255, 255, 255, 255}
			if on {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestHashIsStableForIdenticalImages(t *testing.T) {
	a := checker(128, 8, false)
	b := checker(128, 8, false)
	if d := Hamming(P(a), P(b)); d != 0 {
		t.Errorf("identical images differ by %d bits", d)
	}
	if d := Hamming(D(a), D(b)); d != 0 {
		t.Errorf("identical images differ by %d dHash bits", d)
	}
}

// TestHashSurvivesResampling is the property the whole measurement rests on:
// the same picture at a different resolution must hash to nearly the same
// value, or platform differences in anti-aliasing would masquerade as design
// differences.
func TestHashSurvivesResampling(t *testing.T) {
	a := P(checker(128, 8, false))
	b := P(checker(256, 8, false))
	if d := Hamming(a, b); d > 2 {
		t.Errorf("the same pattern at two resolutions differs by %d bits, want <= 2", d)
	}
}

func TestHashSeparatesDifferentImages(t *testing.T) {
	coarse := P(checker(128, 2, false))
	fine := P(checker(128, 16, false))
	if d := Hamming(coarse, fine); d < 8 {
		t.Errorf("a 2-square and a 16-square board differ by only %d bits", d)
	}
}

func TestHammingBounds(t *testing.T) {
	if got := Hamming(0, 0); got != 0 {
		t.Errorf("Hamming(0,0) = %d", got)
	}
	if got := Hamming(0, ^Hash(0)); got != 64 {
		t.Errorf("Hamming(0,^0) = %d, want 64", got)
	}
}

// TestGrayCompositesOverWhite matters because marks are often drawn on a
// transparent ground: an uncovered pixel must read as page white, not as
// black.
func TestGrayCompositesOverWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	// Left untouched: fully transparent.
	g := Gray(img, 2)
	for i, v := range g {
		if math.Abs(v-1) > 1e-9 {
			t.Errorf("transparent cell %d = %v, want 1 (white)", i, v)
		}
	}

	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	g = Gray(img, 2)
	for i, v := range g {
		if math.Abs(v) > 1e-9 {
			t.Errorf("opaque black cell %d = %v, want 0", i, v)
		}
	}
}

func TestGrayHandlesEmptyImage(t *testing.T) {
	g := Gray(image.NewRGBA(image.Rect(0, 0, 0, 0)), 4)
	if len(g) != 16 {
		t.Fatalf("got %d values, want 16", len(g))
	}
}

// TestDCTMatchesDirectFormula checks the separable transform against the
// textbook double sum on a small matrix.
func TestDCTMatchesDirectFormula(t *testing.T) {
	const n = 8
	px := make([]float64, n*n)
	for i := range px {
		px[i] = float64((i*37)%11) / 11
	}
	got := dct2D(px, n)

	for v := 0; v < n; v++ {
		for u := 0; u < n; u++ {
			var want float64
			for y := 0; y < n; y++ {
				for x := 0; x < n; x++ {
					want += px[y*n+x] *
						math.Cos(float64(2*x+1)*float64(u)*math.Pi/float64(2*n)) *
						math.Cos(float64(2*y+1)*float64(v)*math.Pi/float64(2*n))
				}
			}
			want *= alpha(u, n) * alpha(v, n)
			if diff := math.Abs(got[v*n+u] - want); diff > 1e-9 {
				t.Fatalf("coefficient (%d,%d) = %v, want %v", u, v, got[v*n+u], want)
			}
		}
	}
}

func TestMedian(t *testing.T) {
	if got := median([]float64{3, 1, 2}); got != 2 {
		t.Errorf("median of odd count = %v, want 2", got)
	}
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median of even count = %v, want 2.5", got)
	}
	if got := median(nil); got != 0 {
		t.Errorf("median of nothing = %v, want 0", got)
	}
}
