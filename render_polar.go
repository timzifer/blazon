package blazon

import (
	"math"

	"github.com/timzifer/blazon/canvas"
)

func init() {
	Register(polarRenderer{})
}

// polarRenderer draws a bitmap in polar coordinates: rings by sectors instead
// of rows by columns. It is the same idea as a block identicon and almost the
// same code, but the result reads as a mandala or a rose window rather than as
// a grid of squares — the cheapest possible escape from block optics.
//
// Its cells are mirrored across a fold whose order comes from the archetype
// stream, which is what turns noise into something that looks designed. A
// major version therefore has a symmetry, visible at a glance.
//
// The radial profile carries the identity, not just the cells. Ring widths and
// per-ring density both vary, and a centre device sits in the hole. That is a
// deliberate response to measurement: an evenly spaced ring of evenly filled
// wedges reduces, at the resolution a perceptual hash works at, to "a grey
// annulus" whatever the cells say, and the marks scored far closer together
// than they looked. Information a hash can see has to live in the low
// frequencies, which for a radial figure means the profile from the middle
// outwards.
//
// Amplified parameters: the whole-figure rotation, the density of each ring,
// the ring proportions, the size of the centre device, and — through the
// palette — hue.
//
// Four other ideas were measured and rejected, all for the same reason: on a
// radial figure the low-frequency vocabulary is small, and anything large
// enough for a coarse comparison to see is large enough to dominate it. A
// heavy outer rim on the archetype stream made every version of a major share
// one silhouette, which improved family cohesion and cost more distinctness
// than it bought; on the patch stream it drowned out the symmetry that
// separates majors. Putting the figure's overall size on the variant stream
// separated minors and pushed patches together by exactly as much. Turning
// each ring independently broke the fold symmetry a major is recognised by.
// Letting rings reach empty or solid gained on one relation and lost on
// another. What is left is the configuration whose *weakest* relation is
// strongest — which is the one that matters, since the guarantee is a floor.
type polarRenderer struct{}

func (polarRenderer) Name() string { return "polar" }

// Caps reports no byte-exactness: the wedges are built from Cos and Sin.
func (polarRenderer) Caps() Caps { return Caps{} }

// Centre devices. The hole in the middle of a radial figure is the single most
// visible place on it, and leaving it empty for every version wastes the
// strongest position on the mark.
const (
	centreEmpty = iota
	centreDisc
	centreRing
	centreRosette
	centreStar
)

// densitySteps is the granularity of a ring's fill probability.
const densitySteps = 8

func (polarRenderer) Render(ctx *Context) error {
	p := ctx.Params
	cx, cy := ctx.Box.Center()
	outer := ctx.Box.Min() / 2

	arch := p.Archetype()
	// The fold is the symmetry order. Even folds read as calm and heraldic,
	// odd ones as dynamic; both are far more legible than an unmirrored field.
	fold := arch.IntRange(3, 9)
	mirrored := arch.Bool()
	centre := arch.Choice(5)

	variant := p.Variant()
	rings := variant.IntRange(3, 7)
	// Sectors per fold, so the total sector count is always a multiple of the
	// symmetry order and the pattern closes on itself exactly.
	perFold := variant.IntRange(2, 5)
	hole := variant.Range(0.16, 0.36)
	gap := variant.Range(0.02, 0.16)

	amp := p.Amplified()
	rot := amp.Range(0, 2*math.Pi)
	inkBias := amp.Choice(2)
	centreScale := amp.Range(0.5, 1.0)

	// Ring widths are drawn rather than uniform, and drawn from both streams.
	// A figure of equal rings is the same annulus for every version; the
	// variant part gives a release family its radial rhythm, and the patch
	// part re-proportions it. Without the second half, every version sharing a
	// major and minor differs only in which cells are lit — high-frequency
	// detail that a coarse comparison averages away, and the versions in a
	// long 0.0.x series scored far closer together than they looked.
	widths := make([]float64, rings)
	var widthTotal float64
	for i := range widths {
		widths[i] = variant.Range(0.55, 1.9) * amp.Range(0.65, 1.5)
		widthTotal += widths[i]
	}

	// How full each ring is. This is the loudest thing a patch can change: it
	// rewrites the figure's radial brightness profile, which is precisely what
	// a low-frequency comparison looks at.
	density := make([]int, rings)
	for i := range density {
		// Never quite empty and never quite solid: a ring of either kind
		// carries no angular information, and letting rings reach those states
		// measurably cost more on one relation than it bought on another.
		density[i] = amp.IntRange(2, densitySteps-1)
	}

	sectors := fold * perFold
	// Under mirroring only half of each fold is drawn from entropy; the rest
	// is its reflection.
	drawn := perFold
	if mirrored {
		drawn = (perFold + 1) / 2
	}

	detail := p.Detail()
	cells := make([][]int, rings)
	for r := range cells {
		cells[r] = make([]int, sectors)
		for i := 0; i < drawn; i++ {
			v := 0
			if detail.IntRange(0, densitySteps-1) < density[r] {
				// Two inks, so a ring can be solid, mottled, or empty.
				v = 1 + int(detail.Bit())
			}
			for f := 0; f < fold; f++ {
				cells[r][f*perFold+i] = v
				if mirrored {
					cells[r][f*perFold+(perFold-1-i)] = v
				}
			}
		}
	}

	// Ring boundaries, from the hole outwards, following the drawn widths.
	bounds := make([]float64, rings+1)
	bounds[0] = hole
	acc := 0.0
	for i, w := range widths {
		acc += w
		bounds[i+1] = hole + (1-hole)*acc/widthTotal
	}

	for ink := 1; ink <= 2; ink++ {
		any := false
		for r := 0; r < rings; r++ {
			r0 := outer * bounds[r]
			r1 := outer * bounds[r+1]
			// A gap between rings keeps the figure from fusing into a disc.
			r1 -= (r1 - r0) * gap
			for sIdx := 0; sIdx < sectors; sIdx++ {
				if cells[r][sIdx] != ink {
					continue
				}
				any = true
				a0 := rot + 2*math.Pi*float64(sIdx)/float64(sectors)
				a1 := rot + 2*math.Pi*float64(sIdx+1)/float64(sectors)
				a1 -= 2 * math.Pi / float64(sectors) * gap
				canvas.AnnularSector(ctx.Canvas, cx, cy, r0, r1, a0, a1)
			}
		}
		if any {
			ctx.Canvas.Fill(ctx.Palette.Ink((ink - 1 + inkBias) % 2))
		}
	}

	drawCentreDevice(ctx, centre, cx, cy, outer*hole*centreScale, fold, rot)

	MarkPrerelease(ctx)
	return nil
}

// drawCentreDevice fills the hole. Radius is the space available; each device
// keeps clear of the innermost ring.
func drawCentreDevice(ctx *Context, kind int, cx, cy, radius float64, fold int, rot float64) {
	if kind == centreEmpty || radius <= 0 {
		return
	}
	c := ctx.Canvas

	switch kind {
	case centreDisc:
		circle(c, cx, cy, radius*0.82)

	case centreRing:
		circle(c, cx, cy, radius*0.86)
		// Reversed, so the non-zero rule leaves a hole rather than filling it.
		reverseCircle(c, cx, cy, radius*0.5)

	case centreRosette:
		// One bud per fold, so the centre repeats the figure's symmetry.
		r := radius * 0.34
		ring := radius - r
		for i := 0; i < fold; i++ {
			a := rot + 2*math.Pi*float64(i)/float64(fold)
			circle(c, cx+ring*math.Cos(a), cy+ring*math.Sin(a), r)
		}

	case centreStar:
		// A simple 2·fold-pointed star, alternating between two radii.
		n := fold * 2
		for i := 0; i < n; i++ {
			a := rot + math.Pi*float64(i)/float64(fold)
			rr := radius * 0.9
			if i%2 == 1 {
				rr = radius * 0.38
			}
			x, y := cx+rr*math.Cos(a), cy+rr*math.Sin(a)
			if i == 0 {
				c.MoveTo(x, y)
				continue
			}
			c.LineTo(x, y)
		}
		c.Close()
	}

	ctx.Canvas.Fill(ctx.Palette.Ink(2))
}

// reverseCircle appends a circle wound the opposite way to circle, so that it
// subtracts under the non-zero fill rule.
func reverseCircle(c interface {
	MoveTo(x, y float64)
	CubicTo(x1, y1, x2, y2, x, y float64)
	Close()
}, cx, cy, r float64) {
	k := kappa * r
	c.MoveTo(cx+r, cy)
	c.CubicTo(cx+r, cy-k, cx+k, cy-r, cx, cy-r)
	c.CubicTo(cx-k, cy-r, cx-r, cy-k, cx-r, cy)
	c.CubicTo(cx-r, cy+k, cx-k, cy+r, cx, cy+r)
	c.CubicTo(cx+k, cy+r, cx+r, cy+k, cx+r, cy)
	c.Close()
}
