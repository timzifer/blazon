// Package pngenc encodes images as PNG without image/png.
//
// The library owns its rasteriser and its font so that a mark can only change
// when this repository decides it changes; the encoder is the last step of
// that same pipeline. Owning it keeps the file format on the same footing —
// and keeps the whole library inside the standard library's smallest corner,
// which is what makes a binary that draws marks cost kilobytes rather than
// megabytes.
package pngenc

import (
	"image"
	"image/color"
)

// Encode returns the image as a PNG file.
//
// Fully opaque images are written as 8-bit truecolour and the rest as
// truecolour with alpha, which is the one size decision worth making here:
// most marks are opaque, and the alpha channel would otherwise be a quarter
// of the data for nothing.
func Encode(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		// A zero-sized image is still a valid PNG; the alternative is an
		// error return that no caller could do anything with.
		w, h = 0, 0
	}

	bpp := 4
	colorType := byte(6) // truecolour with alpha
	if opaque(img) {
		bpp = 3
		colorType = 2 // truecolour
	}

	raw := scanlines(img, b, w, h, bpp)

	out := make([]byte, 0, len(raw)/2+256)
	out = append(out, 0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n')

	ihdr := make([]byte, 0, 13)
	ihdr = appendU32(ihdr, uint32(w))
	ihdr = appendU32(ihdr, uint32(h))
	ihdr = append(ihdr, 8, colorType, 0, 0, 0) // depth, colour, deflate, adaptive filtering, no interlace
	out = appendChunk(out, "IHDR", ihdr)
	out = appendChunk(out, "IDAT", zlib(raw))
	out = appendChunk(out, "IEND", nil)
	return out
}

// scanlines lays the pixels out as PNG wants them: one filter byte in front
// of every row, and the filter chosen per row.
func scanlines(img image.Image, b image.Rectangle, w, h, bpp int) []byte {
	stride := w * bpp
	raw := make([]byte, 0, (stride+1)*h)
	cur := make([]byte, stride)
	prev := make([]byte, stride)
	filtered := make([]byte, stride)

	rgba, _ := img.(*image.RGBA)
	for y := 0; y < h; y++ {
		if rgba != nil {
			readRGBA(rgba, b, y, bpp, cur)
		} else {
			readGeneric(img, b, y, bpp, cur)
		}
		ft := filterRow(cur, prev, bpp, filtered)
		raw = append(raw, ft)
		raw = append(raw, filtered...)
		prev, cur = cur, prev
	}
	return raw
}

// readRGBA is the fast path. image.RGBA holds premultiplied alpha, which PNG
// does not, so the colours are divided back out.
func readRGBA(img *image.RGBA, b image.Rectangle, y, bpp int, dst []byte) {
	row := img.Pix[img.PixOffset(b.Min.X, b.Min.Y+y):]
	for x := 0; x < len(dst)/bpp; x++ {
		r, g, bl, a := row[x*4], row[x*4+1], row[x*4+2], row[x*4+3]
		if a != 0 && a != 0xff {
			// The division is carried out at 16 bits and truncated back
			// down, which is what color.NRGBAModel does; doing it at 8 bits
			// would round differently and lose a round trip through any
			// other decoder.
			r = byte(uint32(r) * 0xffff / uint32(a) >> 8)
			g = byte(uint32(g) * 0xffff / uint32(a) >> 8)
			bl = byte(uint32(bl) * 0xffff / uint32(a) >> 8)
		}
		dst[x*bpp], dst[x*bpp+1], dst[x*bpp+2] = r, g, bl
		if bpp == 4 {
			dst[x*bpp+3] = a
		}
	}
}

func readGeneric(img image.Image, b image.Rectangle, y, bpp int, dst []byte) {
	for x := 0; x < len(dst)/bpp; x++ {
		c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
		dst[x*bpp], dst[x*bpp+1], dst[x*bpp+2] = c.R, c.G, c.B
		if bpp == 4 {
			dst[x*bpp+3] = c.A
		}
	}
}

func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

// filterRow picks the row filter and writes the filtered bytes into dst,
// returning the filter type. The choice is the minimum sum of absolute
// signed differences, the heuristic PNG's own specification suggests: it is
// cheap, and on marks — flat areas and long runs of identical rows — it picks
// Up almost everywhere, which is exactly what compresses.
func filterRow(cur, prev []byte, bpp int, dst []byte) byte {
	best := byte(0)
	bestScore := -1
	var cand []byte = make([]byte, len(cur))
	for ft := byte(0); ft <= 4; ft++ {
		applyFilter(ft, cur, prev, bpp, cand)
		score := 0
		for _, v := range cand {
			if int8(v) < 0 {
				score += 256 - int(v)
			} else {
				score += int(v)
			}
		}
		if bestScore < 0 || score < bestScore {
			bestScore, best = score, ft
			copy(dst, cand)
		}
	}
	return best
}

func applyFilter(ft byte, cur, prev []byte, bpp int, dst []byte) {
	for i := range cur {
		var a, b, c byte
		if i >= bpp {
			a = cur[i-bpp]
			c = prev[i-bpp]
		}
		b = prev[i]
		switch ft {
		case 0:
			dst[i] = cur[i]
		case 1:
			dst[i] = cur[i] - a
		case 2:
			dst[i] = cur[i] - b
		case 3:
			dst[i] = cur[i] - byte((int(a)+int(b))/2)
		case 4:
			dst[i] = cur[i] - paeth(a, b, c)
		}
	}
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	switch {
	case pa <= pb && pa <= pc:
		return a
	case pb <= pc:
		return b
	default:
		return c
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func appendU32(dst []byte, v uint32) []byte {
	return append(dst, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func appendChunk(dst []byte, kind string, data []byte) []byte {
	dst = appendU32(dst, uint32(len(data)))
	start := len(dst)
	dst = append(dst, kind...)
	dst = append(dst, data...)
	return appendU32(dst, crc32(dst[start:]))
}
