package blazon

import (
	"math"

	"github.com/timzifer/blazon/canvas"
)

func init() {
	Register(superformulaRenderer{})
}

// superformulaRenderer draws Gielis' superformula:
//
//	r(θ) = ( |cos(mθ/4)/a|^n2 + |sin(mθ/4)/b|^n3 )^(-1/n1)
//
// Six parameters span circles, stars, leaves, starfish and crystals with a
// single closed smooth contour, which is why it is the default: no other
// renderer here gets that much visual range for that little code, and the
// contrast between two parameter sets is immediately obvious.
//
// The mapping is chosen so that the shape family and m — the symmetry count,
// the one property a viewer reads instantly — come from the archetype stream.
// A major version therefore has a gestalt, and its minors and patches are
// variations on it.
//
// Amplified parameters: rotation, an exponent jitter, the twist between nested
// rings, and — through the palette — hue.
type superformulaRenderer struct{}

func (superformulaRenderer) Name() string { return "superformula" }

// Caps reports no byte-exactness: the contour is built from Cos, Sin and Pow,
// which may differ by an ulp between architectures.
func (superformulaRenderer) Caps() Caps { return Caps{} }

// superSamples is the contour sample count before simplification. It is high
// because the formula can produce very sharp cusps that a coarse sample would
// round away; the simplification pass afterwards removes the samples that turn
// out to lie on straight runs.
const superSamples = 1440

func (superformulaRenderer) Render(ctx *Context) error {
	p := ctx.Params
	cx, cy := ctx.Box.Center()
	outer := ctx.Box.Min() / 2

	arch := p.Archetype()
	fam := superFamilies[arch.Choice(len(superFamilies))]
	m := arch.IntRange(fam.minM, fam.maxM)

	shape := p.Variant()

	// The amplified draws are the ones that must separate neighbouring
	// patches. Rotation alone is not enough: a shape with m-fold symmetry
	// barely changes under rotation, and two patches could land on marks a
	// perceptual hash cannot tell apart. So the patch also decides how many
	// rings there are, how far in the inner ones sit, and where the formula's
	// quarter-period offset falls — all changes to the outline's low
	// frequencies, which is exactly what the hash measures.
	amp := p.Amplified()
	// At least two rings, always. A single filled silhouette is a poor
	// carrier of identity: two different convex outlines reduce to almost the
	// same low-frequency content, and the distinctness measurement shows them
	// as the same mark. Nesting a second contour of a different tone puts
	// structure inside the shape, where the measurement — and the eye — can
	// actually see it.
	rings := amp.IntRange(2, 3)
	rot := amp.Range(0, 2*math.Pi)
	jitter := amp.Range(0.7, 1.4)
	twist := amp.Range(-0.6, 0.6)
	inner := amp.Range(0.45, 0.74)
	offset := amp.Range(0, math.Pi/2)

	for i := 0; i < rings; i++ {
		par := fam.draw(shape, m, jitter)
		par.symmetryOffset = offset + twist*float64(i)
		// Each ring is smaller and turned a little further, so a nested set
		// reads as one seal rather than as concentric copies.
		scale := math.Pow(inner, float64(i))
		pts := superContour(par, rot+twist*float64(i), cx, cy, outer*scale, superSamples)
		pts = canvas.Simplify(pts, outer*0.0015)
		canvas.ClosedCurve(ctx.Canvas, pts)
		ctx.Canvas.Fill(ctx.Palette.Ink(i))
	}

	MarkPrerelease(ctx)
	return nil
}

// superFamily is a curated region of the parameter space.
//
// Sampling the six parameters freely does not work. Most of the space produces
// either a near-circle or a sliver, and the interesting shapes — stars,
// flowers, gears, crystals — sit in narrow, disconnected pockets. Naming those
// pockets and drawing a family from the archetype stream is what gives a major
// version a recognisable gestalt instead of yet another rounded blob.
type superFamily struct {
	name       string
	minM, maxM int
	n1Lo, n1Hi float64
	n2Lo, n2Hi float64
	n3Lo, n3Hi float64
	// linked ties n3 to n2, which produces clean rotational symmetry rather
	// than a lopsided outline.
	linked bool
}

var superFamilies = []superFamily{
	// Sharp points meeting at a narrow waist.
	{name: "star", minM: 5, maxM: 12, n1Lo: 0.16, n1Hi: 0.45, n2Lo: 0.3, n2Hi: 1.1, linked: true},
	// Rounded petals.
	{name: "flower", minM: 4, maxM: 10, n1Lo: 0.5, n1Hi: 1.6, n2Lo: 0.6, n2Hi: 2.2, linked: true},
	// A polygon with softened corners.
	{name: "gear", minM: 6, maxM: 14, n1Lo: 2, n1Hi: 9, n2Lo: 6, n2Hi: 28, linked: true},
	// Straight facets and hard corners.
	{name: "crystal", minM: 3, maxM: 8, n1Lo: 0.7, n1Hi: 2.2, n2Lo: 0.25, n2Hi: 0.9, n3Lo: 2, n3Hi: 9},
	// Lopsided, organic outlines.
	{name: "leaf", minM: 2, maxM: 6, n1Lo: 0.3, n1Hi: 1.1, n2Lo: 1, n2Hi: 3.5, n3Lo: 0.2, n3Hi: 0.7},
	// Wide, calm forms with a slow undulation.
	{name: "shield", minM: 3, maxM: 7, n1Lo: 1.5, n1Hi: 4, n2Lo: 1, n2Hi: 3, linked: true},
}

// draw samples a parameter set from the family, resampling once if the first
// draw would be degenerate.
func (f superFamily) draw(br *BitReader, m int, jitter float64) superParams {
	one := func(r *BitReader) superParams {
		n2 := r.LogRange(f.n2Lo, f.n2Hi)
		n3 := n2
		if !f.linked {
			n3 = r.LogRange(f.n3Lo, f.n3Hi)
		}
		return superParams{
			m:  float64(m),
			n1: r.LogRange(f.n1Lo, f.n1Hi) * jitter,
			n2: n2,
			n3: n3,
			a:  r.Range(0.85, 1.2),
			b:  r.Range(0.85, 1.2),
			// Overwritten by the caller from the amplified stream: a
			// quarter-period offset turns a star into a flower without
			// changing its symmetry count, which makes it a strong patch
			// signal that leaves the family intact.
			symmetryOffset: r.Range(0, math.Pi/2),
		}
	}
	par := one(br)
	if superDegenerate(par) {
		par = one(br.Derive("superformula/resample"))
	}
	return par
}

// superParams is one parameter set for the formula.
type superParams struct {
	m              float64
	n1, n2, n3     float64
	a, b           float64
	symmetryOffset float64
}

// radius evaluates the formula. The result is unnormalised; callers scale by
// the maximum over a full turn.
func (p superParams) radius(theta float64) float64 {
	t := p.m*theta/4 + p.symmetryOffset
	c := math.Pow(math.Abs(math.Cos(t)/p.a), p.n2)
	s := math.Pow(math.Abs(math.Sin(t)/p.b), p.n3)
	sum := c + s
	if sum <= 0 || math.IsInf(sum, 0) || math.IsNaN(sum) {
		return 0
	}
	r := math.Pow(sum, -1/p.n1)
	if math.IsInf(r, 0) || math.IsNaN(r) {
		return 0
	}
	return r
}

// superDegenerate reports parameter sets that would render as a sliver or a
// speck. The curated families keep the shapes interesting; this is the safety
// net for the corners of those families where the contour still collapses.
func superDegenerate(p superParams) bool {
	const n = 360
	maxR := 0.0
	var area float64
	var prevX, prevY, firstX, firstY float64

	for i := 0; i < n; i++ {
		th := 2 * math.Pi * float64(i) / n
		r := p.radius(th)
		if r > maxR {
			maxR = r
		}
		x, y := r*math.Cos(th), r*math.Sin(th)
		if i == 0 {
			firstX, firstY = x, y
		} else {
			area += prevX*y - x*prevY
		}
		prevX, prevY = x, y
	}
	area += prevX*firstY - firstX*prevY
	area = math.Abs(area) / 2

	if maxR <= 0 || math.IsInf(maxR, 0) || math.IsNaN(maxR) {
		return true
	}
	// Compared against the disc that circumscribes the shape: a contour
	// filling less than a twelfth of it is a spider, not a seal.
	return area/(math.Pi*maxR*maxR) < 0.08
}

// superContour samples the formula into a closed polyline scaled to radius.
func superContour(p superParams, rot, cx, cy, radius float64, n int) []canvas.Point {
	raw := make([]float64, n)
	maxR := 0.0
	for i := 0; i < n; i++ {
		th := 2 * math.Pi * float64(i) / float64(n)
		r := p.radius(th)
		raw[i] = r
		if r > maxR {
			maxR = r
		}
	}
	if maxR <= 0 {
		maxR = 1
	}
	scale := radius / maxR

	pts := make([]canvas.Point, n)
	for i, r := range raw {
		th := 2*math.Pi*float64(i)/float64(n) + rot
		pts[i] = canvas.Point{
			X: cx + r*scale*math.Cos(th),
			Y: cy + r*scale*math.Sin(th),
		}
	}
	return pts
}
