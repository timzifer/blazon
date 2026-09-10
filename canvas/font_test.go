package canvas

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// TestGlyphsAreWellFormed is why the font is written as pictures: a typo in a
// hex table is invisible, and a typo here is a row of the wrong length.
func TestGlyphsAreWellFormed(t *testing.T) {
	for r, g := range glyphs {
		rows := strings.Split(g, "/")
		if len(rows) != GlyphH {
			t.Errorf("%q has %d rows, want %d", r, len(rows), GlyphH)
			continue
		}
		for i, row := range rows {
			if len(row) != GlyphW {
				t.Errorf("%q row %d is %d wide, want %d: %q", r, i, len(row), GlyphW, row)
			}
			for j := 0; j < len(row); j++ {
				if row[j] != '#' && row[j] != '.' {
					t.Errorf("%q row %d contains %q; only '#' and '.' are allowed", r, i, row[j])
				}
			}
		}
	}
	if len(strings.Split(fallbackGlyph, "/")) != GlyphH {
		t.Error("the fallback glyph is malformed")
	}
}

// TestPrintableASCIIIsCovered matters because version strings carry prerelease
// identifiers and the randomart ramp uses punctuation: a missing glyph would
// silently become a box.
func TestPrintableASCIIIsCovered(t *testing.T) {
	for r := rune(0x20); r <= 0x7e; r++ {
		if _, ok := glyphs[r]; !ok {
			t.Errorf("no glyph for %q (%#x)", r, r)
		}
	}
}

func TestGlyphsAreDistinct(t *testing.T) {
	seen := make(map[string]rune, len(glyphs))
	for r, g := range glyphs {
		if r == ' ' {
			continue
		}
		if prev, dup := seen[g]; dup {
			t.Errorf("%q and %q have identical bitmaps", prev, r)
			continue
		}
		seen[g] = r
	}
	if g := glyphs[' ']; strings.ContainsRune(g, '#') {
		t.Error("the space glyph is not blank")
	}
}

func TestUnknownRuneDrawsTheFallback(t *testing.T) {
	rows := glyphRows('中')
	if strings.Join(rows, "/") != fallbackGlyph {
		t.Error("an uncovered rune did not fall back to the box")
	}
}

func TestDrawStringPlacesGlyphs(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 20))
	DrawString(img, 2, 3, "1.4", color.RGBA{0, 0, 0, 255}, 1)

	var lit int
	for y := 0; y < 20; y++ {
		for x := 0; x < 60; x++ {
			if img.RGBAAt(x, y).A > 0 {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Fatal("DrawString drew nothing")
	}
	// Nothing may be drawn above or left of the origin.
	for y := 0; y < 3; y++ {
		for x := 0; x < 60; x++ {
			if img.RGBAAt(x, y).A > 0 {
				t.Fatalf("pixel at (%d,%d) is above the text origin", x, y)
			}
		}
	}
	for y := 0; y < 20; y++ {
		for x := 0; x < 2; x++ {
			if img.RGBAAt(x, y).A > 0 {
				t.Fatalf("pixel at (%d,%d) is left of the text origin", x, y)
			}
		}
	}
}

func TestDrawStringClipsToBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	// Must not panic or write outside the image.
	DrawString(img, -20, -20, "blazon", color.Black, 3)
	DrawString(img, 100, 100, "blazon", color.Black, 3)
	DrawString(img, 4, 4, "blazon", color.Black, 3)
}

func TestScaledGlyphIsProportional(t *testing.T) {
	count := func(scale int) int {
		img := image.NewRGBA(image.Rect(0, 0, 40, 40))
		DrawGlyph(img, 1, 1, 'H', color.Black, scale)
		n := 0
		for y := 0; y < 40; y++ {
			for x := 0; x < 40; x++ {
				if img.RGBAAt(x, y).A > 0 {
					n++
				}
			}
		}
		return n
	}
	one, three := count(1), count(3)
	if three != one*9 {
		t.Errorf("scaling by three multiplied the ink by %d, want 9", three/one)
	}
}

func TestStringWidth(t *testing.T) {
	if got, want := StringWidth("", 1), 0; got != want {
		t.Errorf("StringWidth(\"\") = %d, want %d", got, want)
	}
	if got, want := StringWidth("A", 1), GlyphW; got != want {
		t.Errorf("StringWidth(\"A\") = %d, want %d", got, want)
	}
	if got, want := StringWidth("AB", 1), GlyphW*2+1; got != want {
		t.Errorf("StringWidth(\"AB\") = %d, want %d", got, want)
	}
	if got, want := StringWidth("AB", 2), (GlyphW*2+1)*2; got != want {
		t.Errorf("StringWidth(\"AB\", 2) = %d, want %d", got, want)
	}
}
