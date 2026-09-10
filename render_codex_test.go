package blazon

import (
	"fmt"
	"testing"
)

func TestCodexWidth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0.0.0", 8},
		{"1.4.2", 8},
		{"255.0.0", 8},
		{"256.0.0", 12},
		{"0.4095.0", 12},
		{"0.0.4096", 16},
		{"18446744073709551615.0.0", 64},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if got := codexWidth(v); got != c.want {
			t.Errorf("codexWidth(%s) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestCodexWidthIsAWholeNibble(t *testing.T) {
	for _, s := range []string{"0.0.0", "1.4.2", "300.12.5", "70000.0.0", "1.0.0-rc.1"} {
		v, _ := ParseVersion(s)
		if w := codexWidth(v); w%4 != 0 {
			t.Errorf("codexWidth(%s) = %d, which is not a whole nibble", s, w)
		}
	}
}

// TestCodexRoundTrip is the promise an ordered renderer makes, checked by
// keeping it: the number has to come back out of the cells.
func TestCodexRoundTrip(t *testing.T) {
	versions := []string{
		"0.0.0", "0.0.1", "1.0.0", "1.4.2", "1.4.3", "12.4.7",
		"300.12.5", "65535.1.0", "1.0.0-rc.1", "18446744073709551615.0.0",
	}
	for _, s := range versions {
		v, err := ParseVersion(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		p := NewParams(NewSeed(v), PolicyFamilyAmplified)
		width := codexWidth(v)
		fields := codexFields(p, width)

		got := Version{
			Major: codexValue(fields[0].cells),
			Minor: codexValue(fields[1].cells),
			Patch: codexValue(fields[2].cells),
		}
		if got != v.Core() {
			t.Errorf("%s decoded to %s", s, got)
		}
	}
}

func TestCodexMostSignificantBitComesFirst(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	p := NewParams(NewSeed(v), PolicyFamilyAmplified)
	cells := codexFields(p, 8)[0].cells

	// The value 1 is the least significant bit, so only the last cell is lit.
	for i, on := range cells {
		want := i == len(cells)-1
		if on != want {
			t.Fatalf("cell %d of major=1 is %v, want %v (cells %v)", i, on, want, cells)
		}
	}
}

func TestCodexPrereleaseFieldOnlyForPrereleases(t *testing.T) {
	release, _ := ParseVersion("1.0.0")
	pre, _ := ParseVersion("1.0.0-rc.1")

	rf := codexFields(NewParams(NewSeed(release), PolicyFamilyAmplified), 8)
	if rf[3].present {
		t.Error("a released version has a prerelease field")
	}
	pf := codexFields(NewParams(NewSeed(pre), PolicyFamilyAmplified), 8)
	if !pf[3].present {
		t.Fatal("a prerelease has no prerelease field")
	}
	if !pf[3].noise {
		t.Error("the prerelease field is not marked as hashed")
	}
	// The core fields must be untouched by the prerelease.
	for i := 0; i < 3; i++ {
		if fmt.Sprint(rf[i].cells) != fmt.Sprint(pf[i].cells) {
			t.Errorf("field %d differs between 1.0.0 and 1.0.0-rc.1", i)
		}
	}
}

// TestCodexPrereleasesDiffer guards the one thing the noise field is for.
func TestCodexPrereleasesDiffer(t *testing.T) {
	seen := map[string]string{}
	for _, s := range []string{"1.0.0-alpha.1", "1.0.0-alpha.2", "1.0.0-beta.1", "1.0.0-rc.1", "1.0.0-rc.2"} {
		v, _ := ParseVersion(s)
		f := codexFields(NewParams(NewSeed(v), PolicyFamilyAmplified), 8)
		key := fmt.Sprint(f[3].cells)
		if prev, dup := seen[key]; dup {
			t.Errorf("%s and %s produced the same prerelease field", prev, s)
			continue
		}
		seen[key] = s
	}
}

func TestCodexRenderersAreOrdered(t *testing.T) {
	for _, name := range []string{"dial", "orbit"} {
		r, ok := Lookup(name)
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		if !r.Caps().Ordered {
			t.Errorf("%s does not declare itself ordered", name)
		}

		o := Options{Renderer: name, Size: 128, Palette: PaletteMono, Background: BackgroundLight}
		a, err := SVG("1.4.2", o)
		if err != nil {
			t.Fatal(err)
		}
		b, err := SVG("1.4.3", o)
		if err != nil {
			t.Fatal(err)
		}
		if string(a) == string(b) {
			t.Fatalf("%s: 1.4.2 and 1.4.3 render identically", name)
		}
		// One bit apart in the patch field, so nearly the whole document is
		// shared. A hashing renderer would share almost nothing.
		if shared := commonPrefix(string(a), string(b)); shared < len(a)/3 {
			t.Errorf("%s: only %d of %d bytes shared between 1.4.2 and 1.4.3",
				name, shared, len(a))
		}
	}
}

func TestFadeIsPremultiplied(t *testing.T) {
	opaque := Palette{}.Ink(0) // opaque black
	got := fade(opaque, 128)
	if got.A != 128 {
		t.Errorf("alpha = %d, want 128", got.A)
	}
	// Premultiplied: no channel may exceed alpha.
	if got.R > got.A || got.G > got.A || got.B > got.A {
		t.Errorf("%+v is not premultiplied", got)
	}
}
