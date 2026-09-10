package blazon

import (
	"math"

	"github.com/timzifer/blazon/canvas"
	"github.com/timzifer/blazon/internal/noise"
)

func init() {
	Register(flowfieldRenderer{})
}

// flowfieldRenderer draws evenly spaced streamlines through an orientation
// field seeded with fingerprint singularities. It is the answer to "make it
// look like a real fingerprint", and it gets there by building the same
// structure a real one has rather than by imitating the look.
//
// The field is the Sherlock–Monro model: ridge orientation at a point is half
// the sum of the angles to the cores minus the angles to the deltas. Placing
// one core and one delta produces a loop; two of each produce a whorl; none
// produces an arch. Those are the three classes a fingerprint examiner reads
// first, and here the class comes from the archetype stream — so the pattern
// type is a directly readable property of the major version.
//
// Ridges are traced with the Jobard–Lefer method: integrate a streamline until
// it comes too close to one already drawn, then seed the next one a fixed
// distance away. That is what keeps the ridge spacing even, and short lines
// that get cut off produce ridge endings and bifurcations on their own —
// minutiae nobody had to place.
//
// Amplified parameters: ridge spacing, field rotation, the offset applied to
// every singularity, and — through the palette — hue.
type flowfieldRenderer struct{}

func (flowfieldRenderer) Name() string { return "flowfield" }

// Caps reports no byte-exactness: the field is built from Atan2 and the
// integration from Cos and Sin.
func (flowfieldRenderer) Caps() Caps { return Caps{} }

// Pattern classes, in the order an examiner would name them.
const (
	patternArch = iota
	patternTentedArch
	patternLoop
	patternDoubleLoop
	patternWhorl
)

var patternNames = [...]string{"arch", "tented arch", "loop", "double loop", "whorl"}

// singularities returns the cores and deltas for a pattern class. Positions
// are in the unit square; the caller jitters them.
func singularities(class int) (cores, deltas [][2]float64) {
	switch class {
	case patternArch:
		return nil, nil
	case patternTentedArch:
		return [][2]float64{{0.5, 0.42}}, [][2]float64{{0.5, 0.78}}
	case patternLoop:
		return [][2]float64{{0.44, 0.40}}, [][2]float64{{0.66, 0.72}}
	case patternDoubleLoop:
		return [][2]float64{{0.40, 0.38}, {0.60, 0.62}},
			[][2]float64{{0.30, 0.72}, {0.70, 0.30}}
	default: // whorl
		return [][2]float64{{0.44, 0.46}, {0.56, 0.54}},
			[][2]float64{{0.24, 0.74}, {0.76, 0.30}}
	}
}

func (flowfieldRenderer) Render(ctx *Context) error {
	p := ctx.Params
	box := ctx.Box
	side := box.Min()
	ox := box.X + (box.W-side)/2
	oy := box.Y + (box.H-side)/2

	arch := p.Archetype()
	class := arch.Choice(len(patternNames))
	cores, deltas := singularities(class)

	variant := p.Variant()
	noiseScale := variant.Range(1.4, 3.2)
	noiseWeight := variant.Range(0.15, 0.5)
	baseAngle := variant.Range(0, math.Pi)
	field := noise.New(p.Seed.H1)

	amp := p.Amplified()
	// Ridge spacing is the loudest thing a patch can change: it alters the
	// entire texture, and a perceptual hash sees it immediately.
	sep := amp.Range(0.026, 0.045)
	rotation := amp.Range(0, math.Pi)
	jitterX := amp.Range(-0.08, 0.08)
	jitterY := amp.Range(-0.08, 0.08)

	cores = offsetPoints(cores, jitterX, jitterY)
	deltas = offsetPoints(deltas, jitterX, jitterY)

	f := &flowField{
		cores:       cores,
		deltas:      deltas,
		noise:       field,
		noiseScale:  noiseScale,
		noiseWeight: noiseWeight,
		base:        baseAngle + rotation,
	}

	lines := traceStreamlines(f, sep)

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		ctx.Canvas.MoveTo(ox+line[0].X*side, oy+line[0].Y*side)
		for _, pt := range line[1:] {
			ctx.Canvas.LineTo(ox+pt.X*side, oy+pt.Y*side)
		}
	}
	// One stroke for the whole field: the ridges are one texture, not a
	// collection of independent marks.
	ctx.Canvas.Stroke(ctx.Palette.Ink(0), side*sep*0.4)

	MarkPrerelease(ctx)
	return nil
}

func offsetPoints(pts [][2]float64, dx, dy float64) [][2]float64 {
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		out[i] = [2]float64{clampUnit(p[0] + dx), clampUnit(p[1] + dy)}
	}
	return out
}

func clampUnit(v float64) float64 {
	if v < 0.05 {
		return 0.05
	}
	if v > 0.95 {
		return 0.95
	}
	return v
}

// flowField evaluates ridge orientation on the unit square.
type flowField struct {
	cores, deltas [][2]float64
	noise         *noise.Simplex
	noiseScale    float64
	noiseWeight   float64
	base          float64
}

// angle returns the ridge direction at a point.
//
// The half-sum is what makes the singularities behave like fingerprint cores
// and deltas rather than like vortices: ridge orientation is defined modulo
// 180 degrees, so a full turn around a core must rotate the ridges by half a
// turn, not a whole one.
func (f *flowField) angle(x, y float64) float64 {
	a := f.base
	for _, c := range f.cores {
		a += 0.5 * math.Atan2(y-c[1], x-c[0])
	}
	for _, d := range f.deltas {
		a -= 0.5 * math.Atan2(y-d[1], x-d[0])
	}
	a += f.noiseWeight * math.Pi * f.noise.Fractal(x*f.noiseScale, y*f.noiseScale, 3)
	return a
}

// inside reports whether a point lies on the print. The domain is an ellipse
// rather than the full square because a fingerprint is left by a fingertip:
// ridges that run off a straight edge read as wallpaper, ridges that stop on a
// curved boundary read as a print.
func (f *flowField) inside(x, y float64) bool {
	const cx, cy = 0.5, 0.52
	const rx, ry = 0.47, 0.5
	dx := (x - cx) / rx
	dy := (y - cy) / ry
	return dx*dx+dy*dy <= 1
}

// step returns the unit direction at a point.
func (f *flowField) step(x, y float64) (dx, dy float64) {
	a := f.angle(x, y)
	return math.Cos(a), math.Sin(a)
}

// traceStreamlines integrates evenly spaced ridges over the print's domain.
//
// This is the Jobard–Lefer algorithm proper: every finished ridge offers new
// seeds one separation to either side of itself, so the next ridge grows
// alongside the last one. Seeding from a lattice instead produces short
// fragments that start wherever the lattice happens to fall; seeding from the
// existing ridges is what makes them long, parallel, and evenly spaced, the
// way real ridges are.
func traceStreamlines(f *flowField, sep float64) [][]canvas.Point {
	g := newSepGrid(sep)
	// Integration step. A third of the separation keeps the curvature of a
	// ridge around a core faithful without making the trace slow.
	h := sep / 3
	// Terminate a little before the nominal spacing, so ridges approach each
	// other and stop — which is what produces ridge endings.
	dTest := sep * 0.82
	// Anything shorter than this is a speck rather than a ridge. The bound is
	// low on purpose: the short ridges that survive it are the ridge endings
	// and bifurcations that make the texture read as a print rather than as a
	// contour map.
	minLen := sep * 2.5

	var out [][]canvas.Point
	queue := []canvas.Point{{X: 0.5, Y: 0.5}}

	// Propagating seeds from finished ridges fills the print outward from the
	// centre, but a ridge that ends short can leave a bald patch behind it
	// that no neighbour ever reaches. After the queue runs dry, sweep for
	// uncovered ground and start again from there.
	for pass := 0; pass < 4 && len(out) < maxRidges; pass++ {
		for len(queue) > 0 && len(out) < maxRidges {
			seed := queue[0]
			queue = queue[1:]

			if !f.inside(seed.X, seed.Y) || g.near(seed.X, seed.Y, sep*0.95) {
				continue
			}
			line := integrate(f, g, seed.X, seed.Y, h, dTest)
			if polylineLength(line) < minLen {
				continue
			}
			for _, pt := range line {
				g.add(pt.X, pt.Y)
			}
			out = append(out, line)
			queue = append(queue, offsetSeeds(line, sep)...)
		}
		queue = uncoveredSeeds(f, g, sep)
		if len(queue) == 0 {
			break
		}
	}
	return out
}

// uncoveredSeeds scans a lattice for ground no ridge has claimed.
func uncoveredSeeds(f *flowField, g *sepGrid, sep float64) []canvas.Point {
	var out []canvas.Point
	n := int(math.Ceil(1 / sep))
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x := (float64(i) + 0.5) / float64(n)
			y := (float64(j) + 0.5) / float64(n)
			if !f.inside(x, y) || g.near(x, y, sep*0.95) {
				continue
			}
			out = append(out, canvas.Point{X: x, Y: y})
		}
	}
	return out
}

// maxRidges bounds the work for one mark. At the densest permitted spacing the
// print holds a few hundred ridges; the cap only guards against a pathological
// field that would otherwise keep spawning seeds.
const maxRidges = 900

// offsetSeeds proposes new seeds perpendicular to a finished ridge, on both
// sides, spaced along it.
func offsetSeeds(line []canvas.Point, sep float64) []canvas.Point {
	var out []canvas.Point
	// One candidate pair per separation length along the ridge is enough to
	// cover the neighbouring band without flooding the queue.
	stride := 3
	for i := stride; i+stride < len(line); i += stride {
		a, b := line[i-1], line[i+1]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		nx, ny := -dy/l*sep, dx/l*sep
		out = append(out,
			canvas.Point{X: line[i].X + nx, Y: line[i].Y + ny},
			canvas.Point{X: line[i].X - nx, Y: line[i].Y - ny},
		)
	}
	return out
}

// integrate traces a streamline in both directions from a seed.
func integrate(f *flowField, g *sepGrid, sx, sy, h, dTest float64) []canvas.Point {
	forward := march(f, g, sx, sy, h, dTest)
	backward := march(f, g, sx, sy, -h, dTest)

	// Reverse the backward half and join, so the result is one continuous
	// ridge rather than two stubs meeting at the seed.
	line := make([]canvas.Point, 0, len(forward)+len(backward))
	for i := len(backward) - 1; i > 0; i-- {
		line = append(line, backward[i])
	}
	line = append(line, forward...)
	return line
}

// maxSteps bounds a single ridge. A streamline that spirals around a core
// would otherwise never satisfy the separation test against its own points.
const maxSteps = 600

func march(f *flowField, g *sepGrid, x, y, h, dTest float64) []canvas.Point {
	pts := []canvas.Point{{X: x, Y: y}}
	// Points of this ridge, checked separately so a ridge does not stop
	// against the step it just took.
	var own []canvas.Point

	for i := 0; i < maxSteps; i++ {
		// Fourth-order Runge-Kutta. Euler drifts visibly around a core, and
		// the drift shows up as ridges that cross each other.
		k1x, k1y := f.step(x, y)
		k2x, k2y := f.step(x+h*k1x/2, y+h*k1y/2)
		k3x, k3y := f.step(x+h*k2x/2, y+h*k2y/2)
		k4x, k4y := f.step(x+h*k3x, y+h*k3y)

		nx := x + h*(k1x+2*k2x+2*k3x+k4x)/6
		ny := y + h*(k1y+2*k2y+2*k3y+k4y)/6

		if !f.inside(nx, ny) {
			break
		}
		if g.near(nx, ny, dTest) {
			break
		}
		if tooCloseToOwn(own, nx, ny, dTest) {
			break
		}

		x, y = nx, ny
		pts = append(pts, canvas.Point{X: x, Y: y})
		// Skip the most recent points when self-testing, or every step would
		// trip the test against its own predecessor.
		if len(pts) > 8 {
			own = pts[:len(pts)-8]
		}
	}
	return pts
}

func tooCloseToOwn(pts []canvas.Point, x, y, d float64) bool {
	d2 := d * d
	for _, p := range pts {
		dx, dy := p.X-x, p.Y-y
		if dx*dx+dy*dy < d2 {
			return true
		}
	}
	return false
}

func polylineLength(pts []canvas.Point) float64 {
	var l float64
	for i := 1; i < len(pts); i++ {
		l += math.Hypot(pts[i].X-pts[i-1].X, pts[i].Y-pts[i-1].Y)
	}
	return l
}

// sepGrid answers "is any existing ridge point within d of here" in constant
// time. A linear scan over every point of every ridge would make the trace
// quadratic and the render unusably slow at realistic ridge densities.
type sepGrid struct {
	cell  float64
	n     int
	cells [][]canvas.Point
}

func newSepGrid(sep float64) *sepGrid {
	n := int(math.Ceil(1/sep)) + 1
	return &sepGrid{cell: 1 / float64(n), n: n, cells: make([][]canvas.Point, n*n)}
}

func (g *sepGrid) index(x, y float64) (int, int) {
	i := int(x / g.cell)
	j := int(y / g.cell)
	return clampInt(i, 0, g.n-1), clampInt(j, 0, g.n-1)
}

func (g *sepGrid) add(x, y float64) {
	i, j := g.index(x, y)
	g.cells[j*g.n+i] = append(g.cells[j*g.n+i], canvas.Point{X: x, Y: y})
}

func (g *sepGrid) near(x, y, d float64) bool {
	i, j := g.index(x, y)
	// The query radius never exceeds the cell size, so the eight neighbours
	// plus the cell itself cover it.
	d2 := d * d
	for dj := -1; dj <= 1; dj++ {
		for di := -1; di <= 1; di++ {
			ci, cj := i+di, j+dj
			if ci < 0 || cj < 0 || ci >= g.n || cj >= g.n {
				continue
			}
			for _, p := range g.cells[cj*g.n+ci] {
				dx, dy := p.X-x, p.Y-y
				if dx*dx+dy*dy < d2 {
					return true
				}
			}
		}
	}
	return false
}
