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
// Amplified parameters: the whole-figure rotation, the ring the pattern starts
// at, and — through the palette — hue.
type polarRenderer struct{}

func (polarRenderer) Name() string { return "polar" }

// Caps reports no byte-exactness: the wedges are built from Cos and Sin.
func (polarRenderer) Caps() Caps { return Caps{} }

func (polarRenderer) Render(ctx *Context) error {
	p := ctx.Params
	cx, cy := ctx.Box.Center()
	outer := ctx.Box.Min() / 2

	arch := p.Archetype()
	// The fold is the symmetry order. Even folds read as calm and heraldic,
	// odd ones as dynamic; both are far more legible than an unmirrored field.
	fold := arch.IntRange(3, 8)
	mirrored := arch.Bool()

	variant := p.Variant()
	rings := variant.IntRange(3, 6)
	// Sectors per fold, so the total sector count is always a multiple of the
	// symmetry order and the pattern closes on itself exactly.
	perFold := variant.IntRange(2, 4)
	hole := variant.Range(0.12, 0.3)
	gap := variant.Range(0, 0.18)

	amp := p.Amplified()
	rot := amp.Range(0, 2*math.Pi)
	inkBias := amp.Choice(2)

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
			// Three states: empty, first ink, second ink. Leaving cells empty
			// is what gives the figure air; a fully populated ring is a solid
			// annulus and carries no information.
			v := detail.Choice(3)
			for f := 0; f < fold; f++ {
				cells[r][f*perFold+i] = v
				if mirrored {
					cells[r][f*perFold+(perFold-1-i)] = v
				}
			}
		}
	}

	ringW := (1 - hole) / float64(rings)

	for ink := 1; ink <= 2; ink++ {
		any := false
		for r := 0; r < rings; r++ {
			r0 := outer * (hole + float64(r)*ringW)
			r1 := outer * (hole + float64(r+1)*ringW)
			// A gap between rings keeps the figure from fusing into a disc.
			r1 -= outer * ringW * gap
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

	MarkPrerelease(ctx)
	return nil
}
