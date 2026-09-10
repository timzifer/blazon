package blazon

import (
	"bytes"
	"image/png"
	"slices"
	"strings"
	"testing"
)

func TestRenderersRegistered(t *testing.T) {
	names := Renderers()
	if !slices.IsSorted(names) {
		t.Errorf("Renderers() is not sorted: %v", names)
	}
	if !slices.Contains(names, "truchet") {
		t.Errorf("truchet is not registered: %v", names)
	}
	for _, n := range names {
		r, ok := Lookup(n)
		if !ok {
			t.Errorf("Lookup(%q) failed although it is listed", n)
			continue
		}
		if r.Name() != n {
			t.Errorf("renderer registered as %q reports name %q", n, r.Name())
		}
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("registering a duplicate name did not panic")
		}
	}()
	Register(truchetRenderer{})
}

func TestOptionDefaults(t *testing.T) {
	var o Options
	if got := o.size(); got != defaultSize {
		t.Errorf("default size = %d, want %d", got, defaultSize)
	}
	if got := o.padding(); got != defaultPadding {
		t.Errorf("default padding = %v, want %v", got, defaultPadding)
	}
	if got := (Options{Padding: -1}).padding(); got != 0 {
		t.Errorf("negative padding = %v, want 0", got)
	}
	if got := o.rendererName(); got != DefaultRenderer {
		t.Errorf("default renderer = %q, want %q", got, DefaultRenderer)
	}
}

func TestUnknownRendererIsAnError(t *testing.T) {
	_, err := SVG("1.0.0", Options{Renderer: "no-such-renderer"})
	if err == nil {
		t.Fatal("unknown renderer did not produce an error")
	}
	if !strings.Contains(err.Error(), "no-such-renderer") {
		t.Errorf("error does not name the renderer: %v", err)
	}
}

func TestInvalidVersionIsAnError(t *testing.T) {
	if _, err := SVG("not.a.version", Options{Renderer: "truchet"}); err == nil {
		t.Fatal("invalid version did not produce an error")
	}
}

func TestSVGAndPNGForEveryRenderer(t *testing.T) {
	for _, name := range Renderers() {
		t.Run(name, func(t *testing.T) {
			o := Options{Renderer: name, Size: 128}
			svg, err := SVG("1.4.2", o)
			if err != nil {
				t.Fatalf("SVG: %v", err)
			}
			if !bytes.HasPrefix(svg, []byte("<svg")) || !bytes.Contains(svg, []byte("</svg>")) {
				t.Error("SVG output is not a complete document")
			}
			raw, err := PNG("1.4.2", o)
			if err != nil {
				t.Fatalf("PNG: %v", err)
			}
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("PNG does not decode: %v", err)
			}
			if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
				t.Errorf("PNG bounds = %v, want 128x128", b)
			}
		})
	}
}

// TestRendersAreDeterministic is the baseline the whole library rests on: the
// same version must produce the same bytes every time, in the same process and
// with no shared state leaking between calls.
func TestRendersAreDeterministic(t *testing.T) {
	for _, name := range Renderers() {
		t.Run(name, func(t *testing.T) {
			o := Options{Renderer: name, Size: 96}
			a, err := SVG("3.7.11", o)
			if err != nil {
				t.Fatal(err)
			}
			// Render something else in between, to catch shared state.
			if _, err := SVG("9.9.9", o); err != nil {
				t.Fatal(err)
			}
			b, err := SVG("3.7.11", o)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, b) {
				t.Error("the same version rendered differently on a second call")
			}
		})
	}
}

func TestBackgroundIsPaintedOrNot(t *testing.T) {
	opaque, err := SVG("1.0.0", Options{Renderer: "truchet", Size: 64, Background: BackgroundDark})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(opaque, []byte("<rect width=")) {
		t.Error("dark background produced no ground rect")
	}
	transparent, err := SVG("1.0.0", Options{Renderer: "truchet", Size: 64})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(transparent, []byte("<rect width=")) {
		t.Error("transparent background produced a ground rect")
	}
}

func TestBoxHelpers(t *testing.T) {
	b := Box{X: 10, Y: 20, W: 100, H: 40}
	if x, y := b.Center(); x != 60 || y != 40 {
		t.Errorf("Center = %v,%v, want 60,40", x, y)
	}
	if got := b.Min(); got != 40 {
		t.Errorf("Min = %v, want 40", got)
	}
}

func TestPaddingShrinksTheDrawingBox(t *testing.T) {
	var got Box
	probe := probeRenderer{fn: func(ctx *Context) { got = ctx.Box }}

	// Rendered through SVGWith rather than Register, so the probe does not
	// appear in the registry and skew tests that sweep every renderer.
	if _, err := SVGWith(probe, "1.0.0", Options{Size: 200, Padding: 0.1}); err != nil {
		t.Fatal(err)
	}
	if got.X != 20 || got.W != 160 {
		t.Errorf("box = %+v, want x=20 w=160", got)
	}
	if _, err := SVGWith(probe, "1.0.0", Options{Size: 200, Padding: -1}); err != nil {
		t.Fatal(err)
	}
	if got.X != 0 || got.W != 200 {
		t.Errorf("unpadded box = %+v, want x=0 w=200", got)
	}
}

// probeRenderer captures the context it is handed.
type probeRenderer struct {
	fn func(*Context)
}

func (probeRenderer) Name() string { return "test-probe" }
func (probeRenderer) Caps() Caps   { return Caps{} }
func (p probeRenderer) Render(ctx *Context) error {
	p.fn(ctx)
	return nil
}
