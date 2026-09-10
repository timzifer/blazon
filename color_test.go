package blazon

import (
	"image/color"
	"math"
	"testing"
)

// relLuminance is the WCAG relative luminance, used to assert that inks stay
// legible against both grounds.
func relLuminance(c color.RGBA) float64 {
	f := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
}

func contrastRatio(a, b color.RGBA) float64 {
	la, lb := relLuminance(a), relLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestOKLCHKnownValues(t *testing.T) {
	// OKLab lightness 1 with zero chroma is white; 0 is black.
	if got := oklchToRGBA(1, 0, 0); got != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("oklchToRGBA(1,0,0) = %+v, want white", got)
	}
	if got := oklchToRGBA(0, 0, 0); got != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("oklchToRGBA(0,0,0) = %+v, want black", got)
	}
	// Mid grey: equal channels, nothing clipped.
	g := oklchToRGBA(0.6, 0, 0)
	if g.R != g.G || g.G != g.B {
		t.Errorf("achromatic OKLCH produced a tinted RGB: %+v", g)
	}
}

// TestOKLCHGamutMapping checks that an unreachable chroma is reduced rather
// than clipped: hue and lightness must survive.
func TestOKLCHGamutMapping(t *testing.T) {
	for _, h := range []float64{0, 60, 120, 180, 240, 300} {
		wild := oklchToRGBA(0.62, 0.9, h) // far outside sRGB
		sane := oklchToRGBA(0.62, 0.13, h)
		if wild.A != 255 {
			t.Fatalf("hue %v: alpha lost", h)
		}
		// A clipped conversion would saturate to a channel extreme in a way
		// that destroys the relative channel ordering; gamut mapping keeps it.
		order := func(c color.RGBA) [3]bool {
			return [3]bool{c.R > c.G, c.G > c.B, c.R > c.B}
		}
		if order(wild) != order(sane) {
			t.Errorf("hue %v: channel ordering changed under gamut mapping: %+v vs %+v", h, wild, sane)
		}
	}
}

func TestPaletteContrastOnBothGrounds(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	corpus := []string{"0.1.0", "1.0.0", "1.4.2", "2.7.13", "17.3.0", "99.0.1"}

	for _, ver := range corpus {
		p := NewParams(mustSeed(t, ver), PolicyFamilyAmplified)
		pal := DerivePalette(p, PaletteColor, BackgroundTransparent)
		if len(pal.Inks) == 0 {
			t.Fatalf("%s: empty palette", ver)
		}
		for i, ink := range pal.Inks {
			// A transparent-ground mark must survive on either surface, so
			// require a modest ratio against both rather than a strong ratio
			// against one.
			if cw := contrastRatio(ink, white); cw < 1.9 {
				t.Errorf("%s ink %d: contrast on white = %.2f, want >= 1.9", ver, i, cw)
			}
			if cb := contrastRatio(ink, black); cb < 1.9 {
				t.Errorf("%s ink %d: contrast on black = %.2f, want >= 1.9", ver, i, cb)
			}
		}
	}
}

func TestPaletteInkCycles(t *testing.T) {
	pal := Palette{Inks: []color.RGBA{{1, 0, 0, 255}, {2, 0, 0, 255}}}
	if pal.Ink(0) != pal.Ink(2) || pal.Ink(1) != pal.Ink(3) {
		t.Error("Ink did not cycle")
	}
	if pal.Ink(-1) != pal.Ink(1) {
		t.Error("Ink did not handle a negative index")
	}
	if (Palette{}).Ink(0).A != 255 {
		t.Error("empty palette did not fall back to opaque black")
	}
}

func TestPaletteMonoIsAchromatic(t *testing.T) {
	p := NewParams(mustSeed(t, "1.4.2"), PolicyFamilyAmplified)
	pal := DerivePalette(p, PaletteMono, BackgroundLight)
	for i, ink := range pal.Inks {
		if ink.R != ink.G || ink.G != ink.B {
			t.Errorf("mono ink %d is tinted: %+v", i, ink)
		}
	}
}

// TestPaletteHueMovesOnPatch is the colour half of the amplified policy: two
// adjacent patches must not share a hue.
func TestPaletteHueMovesOnPatch(t *testing.T) {
	a := DerivePalette(NewParams(mustSeed(t, "1.4.2"), PolicyFamilyAmplified), PaletteColor, BackgroundTransparent)
	b := DerivePalette(NewParams(mustSeed(t, "1.4.3"), PolicyFamilyAmplified), PaletteColor, BackgroundTransparent)
	if a.Ink(0) == b.Ink(0) {
		t.Error("1.4.2 and 1.4.3 produced the same primary ink")
	}
}

func TestGroundColors(t *testing.T) {
	if groundColor(BackgroundTransparent).A != 0 {
		t.Error("transparent ground is not transparent")
	}
	lt, dk := groundColor(BackgroundLight), groundColor(BackgroundDark)
	if relLuminance(lt) <= relLuminance(dk) {
		t.Error("light ground is not lighter than dark ground")
	}
}
