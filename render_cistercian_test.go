package blazon

import (
	"fmt"
	"testing"
)

func TestCistercianGroups(t *testing.T) {
	cases := []struct {
		in   uint64
		want []uint64
	}{
		{0, []uint64{0}},
		{7, []uint64{7}},
		{9999, []uint64{9999}},
		{10000, []uint64{1, 0}},
		{10001, []uint64{1, 1}},
		{123456789, []uint64{1, 2345, 6789}},
	}
	for _, c := range cases {
		got := cistercianGroups(c.in)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("cistercianGroups(%d) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestCistercianGroupsRoundTrip is the injectivity argument in its clearest
// form: the grouping is a base-10000 representation, so recomposing the digits
// must return the original value.
func TestCistercianGroupsRoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 9, 10, 4321, 9999, 10000, 40000, 1 << 40, 1<<64 - 1} {
		var back uint64
		for _, g := range cistercianGroups(v) {
			back = back*cistercianBase + g
		}
		if back != v {
			t.Errorf("round trip of %d gave %d", v, back)
		}
	}
}

func TestCistercianStavesCoverEveryComponent(t *testing.T) {
	v, err := ParseVersion("10001.4.20000")
	if err != nil {
		t.Fatal(err)
	}
	values, component := cistercianStaves(v)
	if len(values) != len(component) {
		t.Fatalf("%d values but %d component tags", len(values), len(component))
	}
	// Two staves for the major, one for the minor, two for the patch.
	want := []int{0, 0, 1, 2, 2}
	if fmt.Sprint(component) != fmt.Sprint(want) {
		t.Errorf("component tags = %v, want %v", component, want)
	}
	if fmt.Sprint(values) != fmt.Sprint([]uint64{1, 1, 4, 2, 0}) {
		t.Errorf("stave values = %v", values)
	}
}

// TestCistercianIsOrdered checks the property that makes this renderer worth
// having: consecutive patch versions differ by a small, systematic amount of
// ink rather than by a whole new picture.
func TestCistercianIsOrdered(t *testing.T) {
	r, ok := Lookup("cistercian")
	if !ok {
		t.Fatal("cistercian is not registered")
	}
	if !r.Caps().Ordered {
		t.Error("cistercian does not declare itself ordered")
	}

	o := Options{Renderer: "cistercian", Size: 128, Palette: PaletteMono, Background: BackgroundLight}
	a, err := SVG("1.4.2", o)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SVG("1.4.3", o)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Fatal("1.4.2 and 1.4.3 render identically")
	}
	// The two marks share their first two staves, so most of the document is
	// common. A hashing renderer would share nothing.
	if shared := commonPrefix(string(a), string(b)); shared < len(a)/2 {
		t.Errorf("only %d of %d bytes are shared between 1.4.2 and 1.4.3; "+
			"an ordered renderer should change little", shared, len(a))
	}
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

func TestCistercianDigitStrokeCounts(t *testing.T) {
	// Each digit is drawn with a known number of strokes; 5, 7 and 8 take two
	// and 9 takes three.
	want := map[int]int{0: 0, 1: 1, 2: 1, 3: 1, 4: 1, 5: 2, 6: 1, 7: 2, 8: 2, 9: 3}
	for d, n := range want {
		c := &countingCanvas{}
		drawCistercianDigit(c, d, 0, 0, 10, 10)
		if c.moves != n {
			t.Errorf("digit %d drew %d strokes, want %d", d, c.moves, n)
		}
	}
	// Out-of-range digits draw nothing rather than panicking.
	c := &countingCanvas{}
	drawCistercianDigit(c, 12, 0, 0, 10, 10)
	drawCistercianDigit(c, -1, 0, 0, 10, 10)
	if c.moves != 0 {
		t.Errorf("an out-of-range digit drew %d strokes", c.moves)
	}
}
