package blazon

import (
	"math"

	"github.com/timzifer/blazon/internal/sha256"
)

// BitReader is a deterministic, unlimited bit stream derived from a 32-byte
// hash. Bits are consumed most-significant-first from successive bytes; when
// the current block is exhausted it is extended by hashing the root together
// with a counter, so a renderer never has to budget its bits.
//
// A BitReader is not safe for concurrent use.
type BitReader struct {
	root    [32]byte
	block   [32]byte
	counter uint64
	byteIdx int  // next unread byte in block
	acc     byte // partially consumed byte
	accBits uint // bits still available in acc
}

// NewBitReader starts a stream at the given root hash.
func NewBitReader(root [32]byte) *BitReader {
	br := &BitReader{root: root, block: root}
	return br
}

// Derive returns an independent stream from the same root, separated by tag.
// Use it when a renderer needs a fresh draw that must stay deterministic, such
// as resampling a degenerate parameter set.
func (b *BitReader) Derive(tag string) *BitReader {
	h := sha256.New()
	h.Write([]byte("blazon/v1/derive\x00"))
	h.Write(b.root[:])
	h.Write([]byte{0})
	h.Write([]byte(tag))
	var root [32]byte
	copy(root[:], h.Sum(nil))
	return NewBitReader(root)
}

// nextByte returns the next byte of the stream, extending it when needed.
func (b *BitReader) nextByte() byte {
	if b.byteIdx >= len(b.block) {
		b.counter++
		h := sha256.New()
		h.Write([]byte("blazon/v1/extend\x00"))
		h.Write(b.root[:])
		var ctr [8]byte
		putUint64(ctr[:], b.counter)
		h.Write(ctr[:])
		copy(b.block[:], h.Sum(nil))
		b.byteIdx = 0
	}
	c := b.block[b.byteIdx]
	b.byteIdx++
	return c
}

// Bit returns the next single bit.
func (b *BitReader) Bit() uint64 {
	if b.accBits == 0 {
		b.acc = b.nextByte()
		b.accBits = 8
	}
	b.accBits--
	return uint64(b.acc>>b.accBits) & 1
}

// Uint returns the next n bits as an unsigned integer, most-significant bit
// first. n must be in [0, 64].
func (b *BitReader) Uint(n uint) uint64 {
	if n > 64 {
		panic("blazon: BitReader.Uint: n > 64")
	}
	var v uint64
	for i := uint(0); i < n; i++ {
		v = v<<1 | b.Bit()
	}
	return v
}

// Bool returns the next bit as a boolean.
func (b *BitReader) Bool() bool { return b.Bit() == 1 }

// Float01 returns a value in [0,1) with 53 bits of precision.
func (b *BitReader) Float01() float64 {
	return float64(b.Uint(53)) / (1 << 53)
}

// Range returns a value in [lo,hi).
func (b *BitReader) Range(lo, hi float64) float64 {
	return lo + b.Float01()*(hi-lo)
}

// LogRange returns a value in [lo,hi) distributed uniformly on a logarithmic
// scale. Both bounds must be positive. Shape parameters whose visual effect is
// multiplicative — the superformula exponents, for instance — must be drawn
// this way, otherwise nearly every sample lands in the large-value regime and
// the shapes collapse onto one another.
func (b *BitReader) LogRange(lo, hi float64) float64 {
	if lo <= 0 || hi <= 0 {
		panic("blazon: BitReader.LogRange: bounds must be positive")
	}
	return math.Exp(b.Range(math.Log(lo), math.Log(hi)))
}

// IntRange returns an integer in [lo,hi] inclusive. It uses rejection sampling
// so the result is unbiased regardless of range width.
func (b *BitReader) IntRange(lo, hi int) int {
	if hi < lo {
		panic("blazon: BitReader.IntRange: hi < lo")
	}
	span := uint64(hi-lo) + 1
	if span == 1 {
		return lo
	}
	// Smallest bit width covering span, then reject out-of-range draws.
	var width uint
	for 1<<width < span {
		width++
	}
	for {
		if v := b.Uint(width); v < span {
			return lo + int(v)
		}
	}
}

// Choice returns an index in [0,n).
func (b *BitReader) Choice(n int) int {
	if n <= 0 {
		panic("blazon: BitReader.Choice: n <= 0")
	}
	return b.IntRange(0, n-1)
}

// Sign returns -1 or +1.
func (b *BitReader) Sign() float64 {
	if b.Bool() {
		return 1
	}
	return -1
}
