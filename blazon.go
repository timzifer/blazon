// Package blazon turns a version number into a visual seal.
//
// The pipeline has three stages. Entropy is derived from the version in
// separated streams (Seed, Params) so that family resemblance between
// neighbouring versions is a policy choice rather than an accident. A Renderer
// maps those streams onto its own parameters — this is the stage that decides
// whether the result is useful. A canvas.Canvas turns drawing primitives into
// SVG, pixels or terminal cells — this stage decides only how it looks.
//
// Distinguishability is not a promise here but a test: see the blazontest
// package, which measures perceptual distance across a version corpus and
// fails when neighbouring versions render too much alike.
package blazon

import (
	"fmt"
	"image"
	"sort"
	"sync"

	"github.com/timzifer/blazon/canvas"
	"github.com/timzifer/blazon/internal/pngenc"
)

// DefaultRenderer is used when Options.Renderer is empty.
const DefaultRenderer = "superformula"

// Release is this library's own version.
//
// It is here because blazon signs its own releases with its own marks: the
// logo in the README is the flowfield mark for this exact version, and the
// release workflow refuses a tag that does not match. Bump it in the same
// commit as the tag.
const Release = "0.1.0"

// LogoRenderer draws the mark blazon uses for itself.
const LogoRenderer = "flowfield"

// Caps describes what a renderer can and cannot do. The registry and the test
// suite consult it instead of special-casing renderers by name.
type Caps struct {
	// TextOnly means the renderer draws exclusively through Canvas.Cell.
	// Vector back ends still render it, via a monospace font, but the
	// terminal is its native target.
	TextOnly bool

	// GridCols and GridRows are the character grid a TextOnly renderer needs.
	GridCols, GridRows int

	// ByteExact means the renderer's geometry is derived from integer
	// arithmetic only, so its output can be compared against golden files
	// byte for byte. Renderers using transcendental functions cannot promise
	// this: math.Sin and math.Pow may differ by an ulp between architectures,
	// so those are compared by perceptual hash with a tolerance instead.
	ByteExact bool

	// Ordered means the mark encodes the version legibly rather than
	// hashing it. Such renderers are exempt from the distinctness thresholds,
	// because being systematic is the point; they are checked for injectivity
	// instead.
	Ordered bool
}

// Box is the drawing area available to a renderer, in canvas units, after
// padding has been applied.
type Box struct{ X, Y, W, H float64 }

// Center returns the middle of the box.
func (b Box) Center() (x, y float64) { return b.X + b.W/2, b.Y + b.H/2 }

// Min returns the shorter of the box's two sides.
func (b Box) Min() float64 {
	if b.W < b.H {
		return b.W
	}
	return b.H
}

// Context is everything a renderer is given. Palette and Box are resolved
// centrally so that colour policy and padding stay in one place instead of
// being reimplemented by every renderer.
type Context struct {
	Params  *Params
	Canvas  canvas.Canvas
	Palette Palette
	Box     Box
	Options Options
}

// Renderer maps entropy onto geometry — stage two of the pipeline.
type Renderer interface {
	Name() string
	Caps() Caps
	Render(ctx *Context) error
}

// Options configures a render. The zero value is valid and yields a 512-pixel
// mark with the default renderer, the default policy and a transparent ground.
type Options struct {
	// Renderer names a registered renderer; empty means DefaultRenderer.
	Renderer string
	// Size is the edge length in pixels or user units; zero means 512.
	Size int
	// Padding is the fraction of Size kept clear at each edge. Zero means the
	// default of 0.06; use a negative value for no padding at all.
	Padding float64
	// Policy controls family resemblance between neighbouring versions.
	Policy Policy
	// Palette selects colour or monochrome ink.
	Palette PaletteMode
	// Background selects the ground.
	Background Background
	// Cols is the terminal width in character cells for Text output; zero
	// means 32. It is ignored by renderers that declare their own grid.
	Cols int
	// NoColor suppresses ANSI colour in Text output, falling back to a
	// density ramp.
	NoColor bool
}

const (
	defaultSize    = 512
	defaultPadding = 0.06
	defaultCols    = 32
)

func (o Options) cols() int {
	if o.Cols <= 0 {
		return defaultCols
	}
	return o.Cols
}

func (o Options) size() int {
	if o.Size <= 0 {
		return defaultSize
	}
	return o.Size
}

func (o Options) padding() float64 {
	if o.Padding == 0 {
		return defaultPadding
	}
	if o.Padding < 0 {
		return 0
	}
	return o.Padding
}

func (o Options) rendererName() string {
	if o.Renderer == "" {
		return DefaultRenderer
	}
	return o.Renderer
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Renderer{}
)

// Register adds a renderer under its own name. It panics on a duplicate name,
// because two renderers sharing a name would make golden files and gallery
// output ambiguous. Renderers in this package register themselves; Register is
// exported so that callers can add their own and run them through blazontest.
func Register(r Renderer) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := r.Name()
	if name == "" {
		panic("blazon: renderer with empty name")
	}
	if _, dup := registry[name]; dup {
		panic("blazon: duplicate renderer name " + name)
	}
	registry[name] = r
}

// Lookup returns the renderer registered under name.
func Lookup(name string) (Renderer, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	r, ok := registry[name]
	return r, ok
}

// Renderers lists the registered renderer names in sorted order.
func Renderers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// resolve looks up the renderer named by the options.
func resolve(o Options) (Renderer, error) {
	name := o.rendererName()
	r, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("blazon: unknown renderer %q (have %v)", name, Renderers())
	}
	return r, nil
}

// prepare resolves the version, palette and drawing box, and sets up a
// character grid when the renderer needs one.
func prepare(r Renderer, version string, o Options, c canvas.Canvas) (*Context, error) {
	v, err := ParseVersion(version)
	if err != nil {
		return nil, err
	}

	p := NewParams(NewSeed(v), o.Policy)
	pal := DerivePalette(p, o.Palette, o.Background)

	w, h := c.Size()
	pad := o.padding() * w
	box := Box{X: pad, Y: pad, W: w - 2*pad, H: h - 2*pad}

	if caps := r.Caps(); caps.TextOnly {
		c.SetGrid(caps.GridCols, caps.GridRows)
	}
	c.Background(pal.Ground)

	return &Context{Params: p, Canvas: c, Palette: pal, Box: box, Options: o}, nil
}

// SVG renders a version to an SVG document.
func SVG(version string, o Options) ([]byte, error) {
	r, err := resolve(o)
	if err != nil {
		return nil, err
	}
	return SVGWith(r, version, o)
}

// SVGWith renders with an explicit renderer, bypassing the registry. It is how
// blazontest exercises a renderer that a caller has written but not
// registered.
func SVGWith(r Renderer, version string, o Options) ([]byte, error) {
	s := float64(o.size())
	c := canvas.NewSVG(s, s)
	ctx, err := prepare(r, version, o, c)
	if err != nil {
		return nil, err
	}
	if err := r.Render(ctx); err != nil {
		return nil, err
	}
	return c.Bytes(), nil
}

// Image renders a version to an RGBA image.
func Image(version string, o Options) (*image.RGBA, error) {
	r, err := resolve(o)
	if err != nil {
		return nil, err
	}
	return ImageWith(r, version, o)
}

// ImageWith renders with an explicit renderer, bypassing the registry.
func ImageWith(r Renderer, version string, o Options) (*image.RGBA, error) {
	s := o.size()
	c := canvas.NewRaster(s, s)
	ctx, err := prepare(r, version, o, c)
	if err != nil {
		return nil, err
	}
	if err := r.Render(ctx); err != nil {
		return nil, err
	}
	return c.Image(), nil
}

// PNG renders a version to a PNG file.
func PNG(version string, o Options) ([]byte, error) {
	img, err := Image(version, o)
	if err != nil {
		return nil, err
	}
	return EncodePNG(img)
}

// EncodePNG encodes an image as PNG.
//
// The encoder is the library's own, for the same reason the rasteriser and
// the font are: it is the last stage that decides what bytes a version turns
// into, and importing image/png would hand that decision — and about a fifth
// of a megabyte of binary — to something outside this repository.
//
// The error is retained in the signature because callers already handle one
// and encoding is where a future format could fail; today it is always nil.
func EncodePNG(img image.Image) ([]byte, error) {
	return pngenc.Encode(img), nil
}

// Text renders a version as a block of terminal output.
//
// Half-block characters give each cell two square pixels, so vector renderers
// arrive in the terminal as pictures rather than as ASCII art. Renderers that
// declare their own character grid — the randomart field — write glyphs
// directly instead.
func Text(version string, o Options) (string, error) {
	r, err := resolve(o)
	if err != nil {
		return "", err
	}
	return TextWith(r, version, o)
}

// TextWith renders with an explicit renderer, bypassing the registry.
func TextWith(r Renderer, version string, o Options) (string, error) {
	cols := o.cols()
	// Two pixel rows per character cell, so half that many rows makes the
	// mark square.
	rows := cols / 2
	if caps := r.Caps(); caps.TextOnly {
		cols, rows = caps.GridCols, caps.GridRows
	}
	c := canvas.NewANSI(cols, rows, !o.NoColor)
	ctx, err := prepare(r, version, o, c)
	if err != nil {
		return "", err
	}
	if err := r.Render(ctx); err != nil {
		return "", err
	}
	return c.String(), nil
}
