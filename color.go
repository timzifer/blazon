package blazon

import (
	"image/color"
	"math"
)

// PaletteMode selects how colour is derived.
type PaletteMode int

const (
	// PaletteColor derives hues from the version's entropy.
	PaletteColor PaletteMode = iota
	// PaletteMono renders in a single neutral ink. Useful for print, for
	// embedding in coloured UI, and for perceptual-hash comparisons that
	// should judge form rather than hue.
	PaletteMono
)

// Background selects the ground the mark is drawn on.
type Background int

const (
	// BackgroundTransparent leaves the ground unpainted. Ink lightness is
	// held mid-band so the mark reads on light and dark surfaces alike.
	BackgroundTransparent Background = iota
	BackgroundLight
	BackgroundDark
)

// Palette is the resolved set of colours for one mark.
type Palette struct {
	Ground color.RGBA // ignored when the background is transparent
	Inks   []color.RGBA
}

// Ink returns the i-th ink, cycling. Renderers index inks rather than picking
// colours themselves, which keeps colour policy in one place.
func (p Palette) Ink(i int) color.RGBA {
	if len(p.Inks) == 0 {
		return color.RGBA{0, 0, 0, 255}
	}
	return p.Inks[((i%len(p.Inks))+len(p.Inks))%len(p.Inks)]
}

// Ink band. Lightness is deliberately confined to a mid range and chroma kept
// moderate: a mark must hold contrast against both a white page and a dark
// terminal, so neither very light nor very dark inks are available.
const (
	inkL      = 0.62
	inkLAlt   = 0.48
	inkC      = 0.13
	groundLLt = 0.97
	groundLDk = 0.18
)

// DerivePalette resolves the palette for a render. The base hue comes from the
// archetype stream, so a major version keeps its colour identity; the hue
// rotation is an amplified parameter, which is what makes neighbouring patch
// versions separate at a glance even when their geometry barely moves.
func DerivePalette(p *Params, mode PaletteMode, bg Background) Palette {
	pal := Palette{Ground: groundColor(bg)}

	if mode == PaletteMono {
		l := inkL
		switch bg {
		case BackgroundLight:
			l = 0.30
		case BackgroundDark:
			l = 0.88
		}
		pal.Inks = []color.RGBA{
			oklchToRGBA(l, 0, 0),
			oklchToRGBA(clamp01(l+0.18), 0, 0),
			oklchToRGBA(clamp01(l-0.18), 0, 0),
		}
		return pal
	}

	baseHue := p.PaletteBase().Range(0, 360)
	rot := p.PaletteShift().Range(0, 360)
	hue := math.Mod(baseHue+rot, 360)

	// Scheme offsets keep the ink set harmonic rather than arbitrary.
	var offsets []float64
	switch p.PaletteBase().Choice(3) {
	case 0: // analogous
		offsets = []float64{0, 28, -28}
	case 1: // complementary split
		offsets = []float64{0, 168, 192}
	default: // triad
		offsets = []float64{0, 120, 240}
	}

	lightPrimary := inkL
	switch bg {
	case BackgroundLight:
		lightPrimary = inkLAlt
	case BackgroundDark:
		lightPrimary = 0.74
	}

	pal.Inks = make([]color.RGBA, 0, len(offsets))
	for i, off := range offsets {
		l := lightPrimary
		if i > 0 {
			l = clamp01(l - 0.10*float64(i))
		}
		pal.Inks = append(pal.Inks, oklchToRGBA(l, inkC, math.Mod(hue+off+360, 360)))
	}
	return pal
}

func groundColor(bg Background) color.RGBA {
	switch bg {
	case BackgroundLight:
		return oklchToRGBA(groundLLt, 0.004, 250)
	case BackgroundDark:
		return oklchToRGBA(groundLDk, 0.008, 250)
	}
	return color.RGBA{}
}

// oklchToRGBA converts OKLCH to sRGB. Out-of-gamut colours have their chroma
// reduced until they fit, which preserves hue and lightness; clamping the
// channels instead would shift both.
func oklchToRGBA(l, c, hDeg float64) color.RGBA {
	lo, hi := 0.0, c
	r, g, b, ok := oklchLinear(l, c, hDeg)
	if !ok {
		for i := 0; i < 24; i++ {
			mid := (lo + hi) / 2
			if _, _, _, in := oklchLinear(l, mid, hDeg); in {
				lo = mid
			} else {
				hi = mid
			}
		}
		r, g, b, _ = oklchLinear(l, lo, hDeg)
	}
	return color.RGBA{
		R: encodeSRGB(r),
		G: encodeSRGB(g),
		B: encodeSRGB(b),
		A: 255,
	}
}

// oklchLinear converts to linear sRGB and reports whether the result is in
// gamut.
func oklchLinear(l, c, hDeg float64) (r, g, b float64, inGamut bool) {
	h := hDeg * math.Pi / 180
	a := c * math.Cos(h)
	bb := c * math.Sin(h)

	l_ := l + 0.3963377774*a + 0.2158037573*bb
	m_ := l - 0.1055613458*a - 0.0638541728*bb
	s_ := l - 0.0894841775*a - 1.2914855480*bb

	lc, mc, sc := l_*l_*l_, m_*m_*m_, s_*s_*s_

	r = +4.0767416621*lc - 3.3077115913*mc + 0.2309699292*sc
	g = -1.2684380046*lc + 2.6097574011*mc - 0.3413193965*sc
	b = -0.0041960863*lc - 0.7034186147*mc + 1.7076147010*sc

	const eps = 1e-6
	inGamut = r >= -eps && r <= 1+eps &&
		g >= -eps && g <= 1+eps &&
		b >= -eps && b <= 1+eps
	return r, g, b, inGamut
}

func encodeSRGB(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(math.Round(v * 255))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
