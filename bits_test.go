package blazon

import (
	"math"
	"testing"
)

func testRoot(b byte) [32]byte {
	var r [32]byte
	for i := range r {
		r[i] = b ^ byte(i)
	}
	return r
}

func TestBitReaderMSBFirst(t *testing.T) {
	var root [32]byte
	root[0] = 0b10110001
	br := NewBitReader(root)
	// The reader consumes a hashed root, not the root itself, so assert the
	// structural property instead: Uint(8) must equal eight successive Bit
	// calls assembled most-significant-first.
	a := NewBitReader(root)
	var want uint64
	for i := 0; i < 8; i++ {
		want = want<<1 | a.Bit()
	}
	if got := br.Uint(8); got != want {
		t.Errorf("Uint(8) = %d, want %d", got, want)
	}
}

func TestBitReaderDeterministic(t *testing.T) {
	root := testRoot(0x5a)
	a, b := NewBitReader(root), NewBitReader(root)
	for i := 0; i < 1000; i++ {
		if x, y := a.Uint(17), b.Uint(17); x != y {
			t.Fatalf("draw %d: %d != %d", i, x, y)
		}
	}
}

func TestBitReaderExtendsPastBlock(t *testing.T) {
	br := NewBitReader(testRoot(0x11))
	// Far past the 256 bits of the root block.
	seen := make(map[uint64]int)
	for i := 0; i < 4096; i++ {
		seen[br.Uint(32)]++
	}
	if len(seen) < 4000 {
		t.Errorf("extended stream is degenerate: %d distinct of 4096", len(seen))
	}
}

func TestBitReaderDeriveIsIndependent(t *testing.T) {
	root := testRoot(0x22)
	base := NewBitReader(root)
	d1 := base.Derive("a")
	d2 := base.Derive("b")
	d1again := NewBitReader(root).Derive("a")

	if d1.Uint(64) == d2.Uint(64) {
		t.Error("Derive(\"a\") and Derive(\"b\") produced the same first draw")
	}
	// Same tag, same root, fresh reader: must reproduce.
	x := NewBitReader(root).Derive("a").Uint(64)
	_ = d1again
	if y := NewBitReader(root).Derive("a").Uint(64); x != y {
		t.Error("Derive is not deterministic")
	}
}

func TestFloat01Range(t *testing.T) {
	br := NewBitReader(testRoot(0x33))
	var sum float64
	const n = 20000
	for i := 0; i < n; i++ {
		v := br.Float01()
		if v < 0 || v >= 1 {
			t.Fatalf("Float01 = %v, out of [0,1)", v)
		}
		sum += v
	}
	if mean := sum / n; math.Abs(mean-0.5) > 0.02 {
		t.Errorf("Float01 mean = %v, want ~0.5", mean)
	}
}

func TestLogRange(t *testing.T) {
	br := NewBitReader(testRoot(0x44))
	lo, hi := 0.1, 40.0
	var below, above int
	const n = 10000
	geoMid := math.Sqrt(lo * hi)
	for i := 0; i < n; i++ {
		v := br.LogRange(lo, hi)
		if v < lo || v >= hi {
			t.Fatalf("LogRange = %v, out of [%v,%v)", v, lo, hi)
		}
		if v < geoMid {
			below++
		} else {
			above++
		}
	}
	// Log-uniform means the geometric midpoint splits the samples evenly.
	// A linear draw would put ~95% above it.
	if r := float64(below) / n; r < 0.45 || r > 0.55 {
		t.Errorf("fraction below geometric midpoint = %v, want ~0.5", r)
	}
}

func TestIntRangeUnbiased(t *testing.T) {
	br := NewBitReader(testRoot(0x55))
	const lo, hi = 3, 16 // span 14: not a power of two, so rejection matters
	counts := make(map[int]int)
	const n = 70000
	for i := 0; i < n; i++ {
		v := br.IntRange(lo, hi)
		if v < lo || v > hi {
			t.Fatalf("IntRange = %d, out of [%d,%d]", v, lo, hi)
		}
		counts[v]++
	}
	span := hi - lo + 1
	want := float64(n) / float64(span)
	for v, c := range counts {
		if dev := math.Abs(float64(c)-want) / want; dev > 0.06 {
			t.Errorf("value %d drawn %d times, want ~%.0f (deviation %.3f)", v, c, want, dev)
		}
	}
	if len(counts) != span {
		t.Errorf("saw %d distinct values, want %d", len(counts), span)
	}
}

func TestIntRangeSingleton(t *testing.T) {
	br := NewBitReader(testRoot(0x66))
	if got := br.IntRange(7, 7); got != 7 {
		t.Errorf("IntRange(7,7) = %d", got)
	}
}
