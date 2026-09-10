package noise

import (
	"math"
	"testing"
)

func seed(b byte) [32]byte {
	var s [32]byte
	for i := range s {
		s[i] = b ^ byte(i*7)
	}
	return s
}

func TestDeterministic(t *testing.T) {
	a, b := New(seed(0x11)), New(seed(0x11))
	for i := 0; i < 500; i++ {
		x := float64(i) * 0.137
		y := float64(i) * 0.271
		if a.At(x, y) != b.At(x, y) {
			t.Fatalf("two fields from one seed differ at (%v,%v)", x, y)
		}
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	a, b := New(seed(0x11)), New(seed(0x22))
	same := 0
	const n = 500
	for i := 0; i < n; i++ {
		x := float64(i) * 0.137
		y := float64(i) * 0.271
		if a.At(x, y) == b.At(x, y) {
			same++
		}
	}
	// Both fields are zero at lattice points, so a few coincidences are
	// expected; near-total agreement would mean the seed is ignored.
	if same > n/10 {
		t.Errorf("two seeds agreed at %d of %d samples", same, n)
	}
}

func TestRangeAndVariation(t *testing.T) {
	s := New(seed(0x33))
	min, max := math.Inf(1), math.Inf(-1)
	var sum float64
	const n = 4000
	for i := 0; i < n; i++ {
		x := float64(i%80) * 0.31
		y := float64(i/80) * 0.29
		v := s.At(x, y)
		if math.IsNaN(v) {
			t.Fatalf("NaN at (%v,%v)", x, y)
		}
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	if min < -1.2 || max > 1.2 {
		t.Errorf("values span [%v,%v], want roughly [-1,1]", min, max)
	}
	if max-min < 0.8 {
		t.Errorf("field is nearly flat: span %v", max-min)
	}
	if mean := sum / n; math.Abs(mean) > 0.15 {
		t.Errorf("mean %v, want near zero", mean)
	}
}

// TestContinuity is what the streamline tracer depends on: neighbouring
// samples must be close, or the integrated ridges would jump.
func TestContinuity(t *testing.T) {
	s := New(seed(0x44))
	const eps = 1e-3
	for i := 0; i < 200; i++ {
		x := float64(i) * 0.37
		y := float64(i) * 0.11
		a := s.At(x, y)
		b := s.At(x+eps, y)
		if math.Abs(a-b) > 0.05 {
			t.Errorf("jump of %v over %v at (%v,%v)", math.Abs(a-b), eps, x, y)
		}
	}
}

func TestFractalIsBounded(t *testing.T) {
	s := New(seed(0x55))
	for i := 0; i < 500; i++ {
		x := float64(i) * 0.19
		y := float64(i) * 0.23
		v := s.Fractal(x, y, 3)
		if v < -1.3 || v > 1.3 {
			t.Fatalf("Fractal = %v at (%v,%v)", v, x, y)
		}
	}
	if got := s.Fractal(1, 1, 0); got != 0 {
		t.Errorf("Fractal with no octaves = %v, want 0", got)
	}
}

// TestFractalOneOctaveIsPlainNoise pins the normalisation: with a single
// octave the weighted sum has to reduce exactly to the base field, or every
// mark's noise weight would mean something different per octave count.
func TestFractalOneOctaveIsPlainNoise(t *testing.T) {
	s := New(seed(0x66))
	for i := 0; i < 200; i++ {
		x := float64(i) * 0.19
		y := float64(i) * 0.07
		if got, want := s.Fractal(x, y, 1), s.At(x, y); got != want {
			t.Fatalf("Fractal(%v,%v,1) = %v, At = %v", x, y, got, want)
		}
	}
}

func TestFractalOctavesChangeTheField(t *testing.T) {
	s := New(seed(0x77))
	differ := 0
	const n = 200
	for i := 0; i < n; i++ {
		x := float64(i) * 0.19
		y := float64(i) * 0.07
		if s.Fractal(x, y, 1) != s.Fractal(x, y, 3) {
			differ++
		}
	}
	if differ < n/2 {
		t.Errorf("adding octaves changed only %d of %d samples", differ, n)
	}
}

func TestFastFloor(t *testing.T) {
	cases := map[float64]int{0: 0, 0.9: 0, 1: 1, -0.1: -1, -1: -1, -1.5: -2}
	for in, want := range cases {
		if got := fastFloor(in); got != want {
			t.Errorf("fastFloor(%v) = %d, want %d", in, got, want)
		}
	}
}
