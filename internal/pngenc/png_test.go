package pngenc

import (
	"bytes"
	"image"
	"image/color"
	stdpng "image/png"
	"math/rand"
	"testing"
)

// TestRoundTripsThroughTheStandardDecoder is the whole contract of this
// package: whatever the encoder emits, the reference implementation must read
// back as the pixels that went in. It is checked over several kinds of image
// because the parts most likely to be wrong — the filter heuristic and the
// match search — behave differently on flat, noisy and transparent input.
func TestRoundTripsThroughTheStandardDecoder(t *testing.T) {
	for _, tc := range []struct {
		name string
		img  *image.RGBA
	}{
		{"flat", flat(64, 64, color.RGBA{0x20, 0x30, 0x40, 0xff})},
		{"noise", noise(64, 64, 1, 0xff)},
		{"translucent", noise(64, 64, 2, 0)},
		{"one pixel", flat(1, 1, color.RGBA{0xff, 0, 0, 0xff})},
		{"tall and thin", noise(1, 300, 3, 0xff)},
		{"wide and short", noise(300, 1, 4, 0xff)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stdpng.Decode(bytes.NewReader(Encode(tc.img)))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Bounds() != tc.img.Bounds() {
				t.Fatalf("bounds = %v, want %v", got.Bounds(), tc.img.Bounds())
			}
			for y := 0; y < tc.img.Bounds().Dy(); y++ {
				for x := 0; x < tc.img.Bounds().Dx(); x++ {
					want := color.NRGBAModel.Convert(tc.img.At(x, y))
					if have := color.NRGBAModel.Convert(got.At(x, y)); have != want {
						t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, have, want)
					}
				}
			}
		})
	}
}

// TestOpaqueImagesDropTheAlphaChannel guards the one format choice the
// encoder makes on the caller's behalf.
func TestOpaqueImagesDropTheAlphaChannel(t *testing.T) {
	if got := colorType(Encode(noise(32, 32, 5, 0xff))); got != 2 {
		t.Errorf("opaque image encoded as colour type %d, want 2", got)
	}
	if got := colorType(Encode(noise(32, 32, 5, 0x80))); got != 6 {
		t.Errorf("translucent image encoded as colour type %d, want 6", got)
	}
}

// TestEncodingIsDeterministic matters more here than anywhere else in the
// package: the gallery is committed and diffed, so an encoder that varied
// would show up as a spurious image change on every regeneration.
func TestEncodingIsDeterministic(t *testing.T) {
	img := noise(48, 48, 6, 0xff)
	if a, b := Encode(img), Encode(img); !bytes.Equal(a, b) {
		t.Error("two encodings of the same image differ")
	}
}

// TestCompressesBetterThanStoring keeps the compressor honest. It is not a
// race against image/png — the fixed-Huffman stream will lose that — but a
// file no smaller than its pixels would mean the match search is not working
// at all.
func TestCompressesBetterThanStoring(t *testing.T) {
	img := flat(128, 128, color.RGBA{0x11, 0x22, 0x33, 0xff})
	if got, raw := len(Encode(img)), 128*128*3; got > raw/10 {
		t.Errorf("flat 128x128 image encodes to %d bytes, want well under %d", got, raw/10)
	}

	img = noise(128, 128, 7, 0xff)
	std := new(bytes.Buffer)
	if err := stdpng.Encode(std, img); err != nil {
		t.Fatal(err)
	}
	// Noise is incompressible, so this is really a check that the fixed
	// Huffman block does not inflate the data the way a stored block would.
	if got, want := len(Encode(img)), std.Len()*12/10; got > want {
		t.Errorf("noise encodes to %d bytes, more than 1.2x image/png's %d", got, std.Len())
	}
}

// TestEncodesAnyImage covers the generic path, which the library itself does
// not take but an exported encoder invites.
func TestEncodesAnyImage(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 16, 16))
	for i := range src.Pix {
		src.Pix[i] = byte(i)
	}
	got, err := stdpng.Decode(bytes.NewReader(Encode(src)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			want := color.NRGBAModel.Convert(src.At(x, y))
			if have := color.NRGBAModel.Convert(got.At(x, y)); have != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, have, want)
			}
		}
	}
}

// TestNonZeroOriginIsEncodedFromItsBounds catches the classic sub-image bug,
// where the pixels are read from the origin instead of the bounds.
func TestNonZeroOriginIsEncodedFromItsBounds(t *testing.T) {
	full := noise(32, 32, 8, 0xff)
	sub := full.SubImage(image.Rect(8, 8, 24, 24)).(*image.RGBA)
	got, err := stdpng.Decode(bytes.NewReader(Encode(sub)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b := got.Bounds(); b.Dx() != 16 || b.Dy() != 16 {
		t.Fatalf("bounds = %v, want 16x16", b)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			want := color.NRGBAModel.Convert(full.At(8+x, 8+y))
			if have := color.NRGBAModel.Convert(got.At(x, y)); have != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, have, want)
			}
		}
	}
}

func colorType(data []byte) byte {
	// Signature, length and type of the first chunk, then width and height.
	return data[8+8+8+1]
}

func flat(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// noise fills an image with random premultiplied pixels. An alpha of 0xff
// makes it opaque; anything else randomises alpha from that floor upwards.
func noise(w, h int, seed int64, alpha byte) *image.RGBA {
	rng := rand.New(rand.NewSource(seed))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := alpha
			if a != 0xff {
				a = byte(int(alpha) + rng.Intn(256-int(alpha)))
			}
			img.SetRGBA(x, y, color.RGBA{
				byte(rng.Intn(int(a) + 1)),
				byte(rng.Intn(int(a) + 1)),
				byte(rng.Intn(int(a) + 1)),
				a,
			})
		}
	}
	return img
}
