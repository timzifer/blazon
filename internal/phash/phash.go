// Package phash computes perceptual hashes of rendered marks.
//
// It exists so that "these two versions look different" can be a test
// assertion rather than a claim. Two hashes are compared by Hamming distance:
// small means the images look alike to a human, large means they do not.
package phash

import (
	"image"
	"image/color"
	"math"
	"math/bits"
)

// Hash is a 64-bit perceptual hash.
type Hash uint64

// Hamming reports the number of differing bits, which is the perceptual
// distance between two marks. The scale runs from 0 (identical) to 64.
func Hamming(a, b Hash) int { return bits.OnesCount64(uint64(a ^ b)) }

// Combined is the measure actually used to judge renderers: the DCT hash and
// the difference hash together, giving 128 bits.
//
// Sixty-four bits of DCT hash alone are not enough. Two different filled
// silhouettes reduce to nearly the same low-frequency content, and pairs that
// are obviously distinct to the eye can land on the same value. The difference
// hash fails in an unrelated way — it reads local gradients, not global
// structure — so the pair together separates marks that either one alone
// confuses.
type Combined struct {
	P, D Hash
}

// Of computes both hashes of an image.
func Of(img image.Image) Combined { return Combined{P: P(img), D: D(img)} }

// Distance is the Hamming distance over both hashes, from 0 to MaxDistance.
func Distance(a, b Combined) int {
	return Hamming(a.P, b.P) + Hamming(a.D, b.D)
}

// MaxDistance is the largest value Distance can return.
const MaxDistance = 128

const (
	// dctSize is the working resolution. Hashing a downsampled image is what
	// makes the measure perceptual rather than pixel-exact: sub-pixel
	// differences between platforms wash out, real shape differences do not.
	dctSize = 32
	// hashSide is the edge of the retained low-frequency block.
	hashSide = 8
)

// Gray downsamples an image to n×n luminance values in [0,1]. Alpha is
// composited over white, so a mark drawn on a transparent ground still has
// the contrast it would have on a page.
func Gray(img image.Image, n int) []float64 {
	b := img.Bounds()
	out := make([]float64, n*n)
	if b.Empty() {
		return out
	}
	for oy := 0; oy < n; oy++ {
		y0 := b.Min.Y + oy*b.Dy()/n
		y1 := b.Min.Y + (oy+1)*b.Dy()/n
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for ox := 0; ox < n; ox++ {
			x0 := b.Min.X + ox*b.Dx()/n
			x1 := b.Min.X + (ox+1)*b.Dx()/n
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum float64
			var count int
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					sum += luminance(img.At(x, y))
					count++
				}
			}
			out[oy*n+ox] = sum / float64(count)
		}
	}
	return out
}

func luminance(c color.Color) float64 {
	r, g, b, a := c.RGBA()
	af := float64(a) / 0xffff
	// Composite over white: the un-covered part of the pixel contributes full
	// luminance.
	rf := float64(r)/0xffff + (1 - af)
	gf := float64(g)/0xffff + (1 - af)
	bf := float64(b)/0xffff + (1 - af)
	return 0.2126*rf + 0.7152*gf + 0.0722*bf
}

// P computes the DCT-based perceptual hash. The image is reduced to 32×32,
// transformed, and the low-frequency 8×8 block (minus the DC term, which only
// carries overall brightness) is thresholded at its median. Comparing
// low-frequency content is what makes the hash tolerant of anti-aliasing and
// of one-ulp differences in transcendental functions between architectures,
// while still separating genuinely different shapes.
func P(img image.Image) Hash {
	px := Gray(img, dctSize)
	coef := dct2D(px, dctSize)

	vals := make([]float64, 0, hashSide*hashSide)
	for y := 0; y < hashSide; y++ {
		for x := 0; x < hashSide; x++ {
			vals = append(vals, coef[y*dctSize+x])
		}
	}
	med := median(append([]float64(nil), vals[1:]...))

	var h Hash
	for i, v := range vals {
		if v > med {
			h |= 1 << uint(i)
		}
	}
	return h
}

// D computes the difference hash: a 9×8 grid compared horizontally. It is a
// cheap second opinion with very different failure modes from P, used as a
// cross-check rather than as the primary measure.
func D(img image.Image) Hash {
	const w, h = 9, 8
	b := img.Bounds()
	px := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*b.Dx()/w
			sy := b.Min.Y + y*b.Dy()/h
			px[y*w+x] = luminance(img.At(sx, sy))
		}
	}
	var out Hash
	i := 0
	for y := 0; y < h; y++ {
		for x := 0; x+1 < w; x++ {
			if px[y*w+x] > px[y*w+x+1] {
				out |= 1 << uint(i)
			}
			i++
		}
	}
	return out
}

// dct2D computes a separable DCT-II over an n×n matrix.
func dct2D(px []float64, n int) []float64 {
	cos := cosTable(n)
	tmp := make([]float64, n*n)
	for y := 0; y < n; y++ {
		row := px[y*n : (y+1)*n]
		for u := 0; u < n; u++ {
			var s float64
			for x := 0; x < n; x++ {
				s += row[x] * cos[u*n+x]
			}
			tmp[y*n+u] = s * alpha(u, n)
		}
	}
	out := make([]float64, n*n)
	for u := 0; u < n; u++ {
		for v := 0; v < n; v++ {
			var s float64
			for y := 0; y < n; y++ {
				s += tmp[y*n+u] * cos[v*n+y]
			}
			out[v*n+u] = s * alpha(v, n)
		}
	}
	return out
}

func alpha(u, n int) float64 {
	if u == 0 {
		return math.Sqrt(1 / float64(n))
	}
	return math.Sqrt(2 / float64(n))
}

// cos32 holds the DCT basis for the working resolution. Only one size is ever
// used, so it is built once at init rather than cached in a map that would
// need locking under the race detector.
var cos32 = buildCosTable(dctSize)

func buildCosTable(n int) []float64 {
	t := make([]float64, n*n)
	for u := 0; u < n; u++ {
		for x := 0; x < n; x++ {
			t[u*n+x] = math.Cos(float64(2*x+1) * float64(u) * math.Pi / float64(2*n))
		}
	}
	return t
}

func cosTable(n int) []float64 {
	if n == dctSize {
		return cos32
	}
	return buildCosTable(n)
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	// Insertion sort: the slice is always 63 elements here.
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}
