// Package sha256 implements SHA-256 (FIPS 180-4).
//
// It exists for size, not for secrecy. Nothing here is a secret — the input
// is a version string and the output is a picture — but crypto/sha256 now
// arrives with the FIPS-140 module's self-tests, entropy sources and runtime
// checks behind it, which is about a sixth of a megabyte in a binary whose
// only job is to draw a mark. The algorithm itself is a fixed standard, so
// owning it costs nothing in drift: the digests are the same digests, and a
// test checks that against crypto/sha256 for every input the library uses.
package sha256

// Size is the length of a SHA-256 digest in bytes.
const Size = 32

// BlockSize is the algorithm's block size in bytes.
const BlockSize = 64

var initial = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

var k = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

// Digest is an in-progress SHA-256 computation. Its method set is the part of
// hash.Hash the library uses, so callers read the same as they did against
// crypto/sha256.
type Digest struct {
	h   [8]uint32
	buf [BlockSize]byte
	n   int    // bytes buffered
	len uint64 // bytes consumed in total
}

// New returns a Digest ready to hash.
func New() *Digest {
	d := new(Digest)
	d.Reset()
	return d
}

// Reset returns the Digest to its initial state.
func (d *Digest) Reset() {
	d.h = initial
	d.n = 0
	d.len = 0
}

// Size returns the digest length in bytes.
func (d *Digest) Size() int { return Size }

// BlockSize returns the algorithm's block size in bytes.
func (d *Digest) BlockSize() int { return BlockSize }

// Write adds more data to the running digest. It never returns an error.
func (d *Digest) Write(p []byte) (int, error) {
	written := len(p)
	d.len += uint64(written)
	if d.n > 0 {
		n := copy(d.buf[d.n:], p)
		d.n += n
		p = p[n:]
		if d.n < BlockSize {
			// Still short of a block, so everything is buffered and there is
			// nothing to compress yet.
			return written, nil
		}
		d.block(d.buf[:])
		d.n = 0
	}
	for len(p) >= BlockSize {
		d.block(p[:BlockSize])
		p = p[BlockSize:]
	}
	d.n = copy(d.buf[:], p)
	return written, nil
}

// Sum appends the digest of everything written so far to b, leaving the
// Digest itself usable for further writes.
func (d *Digest) Sum(b []byte) []byte {
	// The padding is applied to a copy so that Sum stays non-destructive,
	// which is what hash.Hash promises and what the callers here rely on.
	c := *d

	// A one bit, then zeroes, then the length in bits — enough zeroes that
	// the length lands in the final eight bytes of a block.
	var pad [BlockSize * 2]byte
	pad[0] = 0x80
	padLen := BlockSize - int((c.len+9)%BlockSize)
	if padLen == BlockSize {
		padLen = 0
	}
	tail := pad[:1+padLen+8]
	putUint64(tail[1+padLen:], c.len*8)
	c.Write(tail)

	var out [Size]byte
	for i, v := range c.h {
		putUint32(out[i*4:], v)
	}
	return append(b, out[:]...)
}

// Sum256 returns the digest of data.
func Sum256(data []byte) [Size]byte {
	d := New()
	d.Write(data)
	var out [Size]byte
	copy(out[:], d.Sum(nil))
	return out
}

func (d *Digest) block(p []byte) {
	var w [64]uint32
	for i := 0; i < 16; i++ {
		w[i] = uint32be(p[i*4:])
	}
	for i := 16; i < 64; i++ {
		s0 := rotr(w[i-15], 7) ^ rotr(w[i-15], 18) ^ (w[i-15] >> 3)
		s1 := rotr(w[i-2], 17) ^ rotr(w[i-2], 19) ^ (w[i-2] >> 10)
		w[i] = w[i-16] + s0 + w[i-7] + s1
	}

	a, b, c, dd := d.h[0], d.h[1], d.h[2], d.h[3]
	e, f, g, h := d.h[4], d.h[5], d.h[6], d.h[7]
	for i := 0; i < 64; i++ {
		s1 := rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)
		ch := (e & f) ^ (^e & g)
		t1 := h + s1 + ch + k[i] + w[i]
		s0 := rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)
		maj := (a & b) ^ (a & c) ^ (b & c)
		t2 := s0 + maj

		h, g, f, e = g, f, e, dd+t1
		dd, c, b, a = c, b, a, t1+t2
	}
	d.h[0] += a
	d.h[1] += b
	d.h[2] += c
	d.h[3] += dd
	d.h[4] += e
	d.h[5] += f
	d.h[6] += g
	d.h[7] += h
}

func rotr(v uint32, n uint) uint32 { return v>>n | v<<(32-n) }

// The three big-endian conversions the algorithm needs. encoding/binary would
// do them, but it imports reflect, and reflect is the largest thing this
// library could add to a caller's binary.

func uint32be(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func putUint32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
}

func putUint64(b []byte, v uint64) {
	putUint32(b, uint32(v>>32))
	putUint32(b[4:], uint32(v))
}
