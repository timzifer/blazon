package pngenc

// A deflate compressor, written here so the library keeps no dependency on
// compress/flate. The output is a fixed-Huffman stream: it compresses the
// large flat areas and repeated scanlines a mark produces well enough, and it
// costs a fraction of the code a dynamic-Huffman encoder would.
//
// Determinism matters more here than the last percent of ratio. The match
// search is fully specified below — window, chain limit, lazy rule — so the
// same pixels always produce the same file, on every architecture and every
// Go release.

const (
	windowSize = 1 << 15 // the deflate window, 32 KiB
	minMatch   = 3
	maxMatch   = 258
	hashBits   = 15
	hashSize   = 1 << hashBits
	maxChain   = 128 // positions examined per match; caps the worst case
	noPos      = -1
)

// Tables from RFC 1951, section 3.2.5.
var (
	lengthBase = [...]int{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31,
		35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258}
	lengthExtra = [...]uint{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2,
		3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0}
	distBase = [...]int{1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193,
		257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577}
	distExtra = [...]uint{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6,
		7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13}
)

// bitWriter emits deflate's little-endian bit order: bits fill each byte from
// the least significant end, while Huffman codes are written most significant
// bit first.
type bitWriter struct {
	out   []byte
	bits  uint32
	nbits uint
}

func (w *bitWriter) writeBits(v uint32, n uint) {
	w.bits |= v << w.nbits
	w.nbits += n
	for w.nbits >= 8 {
		w.out = append(w.out, byte(w.bits))
		w.bits >>= 8
		w.nbits -= 8
	}
}

// writeCode emits a Huffman code, whose bits travel in the opposite order to
// the value bits above.
func (w *bitWriter) writeCode(code uint32, n uint) {
	var rev uint32
	for i := uint(0); i < n; i++ {
		rev = rev<<1 | (code>>i)&1
	}
	w.writeBits(rev, n)
}

func (w *bitWriter) flush() {
	for w.nbits > 0 {
		w.out = append(w.out, byte(w.bits))
		w.bits >>= 8
		if w.nbits < 8 {
			w.nbits = 0
		} else {
			w.nbits -= 8
		}
	}
}

// writeSymbol emits a literal/length symbol from the fixed alphabet.
func (w *bitWriter) writeSymbol(sym int) {
	switch {
	case sym < 144:
		w.writeCode(0x30+uint32(sym), 8)
	case sym < 256:
		w.writeCode(0x190+uint32(sym)-144, 9)
	case sym < 280:
		w.writeCode(uint32(sym)-256, 7)
	default:
		w.writeCode(0xC0+uint32(sym)-280, 8)
	}
}

func (w *bitWriter) writeMatch(length, dist int) {
	lc := len(lengthBase) - 1
	for lc > 0 && lengthBase[lc] > length {
		lc--
	}
	w.writeSymbol(257 + lc)
	if n := lengthExtra[lc]; n > 0 {
		w.writeBits(uint32(length-lengthBase[lc]), n)
	}

	dc := len(distBase) - 1
	for dc > 0 && distBase[dc] > dist {
		dc--
	}
	// Distance codes are five fixed bits, most significant first.
	w.writeCode(uint32(dc), 5)
	if n := distExtra[dc]; n > 0 {
		w.writeBits(uint32(dist-distBase[dc]), n)
	}
}

// deflate compresses src into a single fixed-Huffman block.
func deflate(src []byte) []byte {
	w := &bitWriter{out: make([]byte, 0, len(src)/2+64)}
	w.writeBits(1, 1) // final block
	w.writeBits(1, 2) // fixed Huffman

	head := make([]int32, hashSize)
	for i := range head {
		head[i] = noPos
	}
	prev := make([]int32, len(src))

	pos := 0
	for pos < len(src) {
		length, dist := 0, 0
		if pos+minMatch <= len(src) {
			length, dist = findMatch(src, prev, head, pos, 0)
		}
		if length >= minMatch {
			// Lazy matching: a longer match starting one byte later is worth
			// the extra literal, and this is where most of the ratio beyond a
			// plain greedy parse comes from.
			if pos+1+minMatch <= len(src) {
				if l2, d2 := findMatch(src, prev, head, pos+1, length); l2 > length {
					w.writeSymbol(int(src[pos]))
					insert(src, prev, head, pos)
					pos++
					length, dist = l2, d2
				}
			}
			w.writeMatch(length, dist)
			for i := 0; i < length; i++ {
				insert(src, prev, head, pos+i)
			}
			pos += length
			continue
		}
		w.writeSymbol(int(src[pos]))
		insert(src, prev, head, pos)
		pos++
	}

	w.writeSymbol(256) // end of block
	w.flush()
	return w.out
}

func hash4(src []byte, pos int) uint32 {
	// Three bytes are the shortest match deflate can encode, so they are what
	// the chain is keyed on.
	h := uint32(src[pos])<<16 | uint32(src[pos+1])<<8 | uint32(src[pos+2])
	return (h * 0x9E3779B1) >> (32 - hashBits)
}

func insert(src []byte, prev []int32, head []int32, pos int) {
	if pos+minMatch > len(src) {
		return
	}
	h := hash4(src, pos)
	prev[pos] = head[h]
	head[h] = int32(pos)
}

// findMatch returns the longest match for src[pos:], or a length below
// minMatch when there is none longer than best.
func findMatch(src []byte, prev []int32, head []int32, pos, best int) (int, int) {
	if pos+minMatch > len(src) {
		return 0, 0
	}
	limit := pos - windowSize
	if limit < 0 {
		limit = 0
	}
	maxLen := len(src) - pos
	if maxLen > maxMatch {
		maxLen = maxMatch
	}
	bestLen, bestDist := best, 0
	cand := head[hash4(src, pos)]
	for chain := 0; chain < maxChain && cand >= 0 && int(cand) >= limit; chain++ {
		i := int(cand)
		cand = prev[i]
		if bestLen >= maxLen {
			break
		}
		// The byte past the current best is the cheapest way to reject a
		// candidate that cannot improve on it.
		if bestLen > 0 && src[i+bestLen] != src[pos+bestLen] {
			continue
		}
		n := 0
		for n < maxLen && src[i+n] == src[pos+n] {
			n++
		}
		if n > bestLen {
			bestLen, bestDist = n, pos-i
		}
	}
	if bestLen < minMatch || bestDist == 0 {
		return 0, 0
	}
	return bestLen, bestDist
}

// zlib wraps a deflate stream in the container PNG's IDAT expects.
func zlib(src []byte) []byte {
	out := make([]byte, 0, len(src)/2+64)
	out = append(out, 0x78, 0x01) // 32 KiB window, deflate
	out = append(out, deflate(src)...)
	a := adler32(src)
	return append(out, byte(a>>24), byte(a>>16), byte(a>>8), byte(a))
}

func adler32(data []byte) uint32 {
	const mod = 65521
	s1, s2 := uint32(1), uint32(0)
	for _, b := range data {
		s1 += uint32(b)
		if s1 >= mod {
			s1 -= mod
		}
		s2 += s1
		if s2 >= mod {
			s2 -= mod
		}
	}
	return s2<<16 | s1
}
