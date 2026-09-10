// Package noise provides deterministic 2D simplex noise.
//
// It is written out rather than imported because the whole library depends on
// reproducibility: the permutation table has to be derived from the version's
// own entropy, and the algorithm has to be fixed for the lifetime of the
// output format. A dependency that improved its noise would silently change
// every mark ever generated.
package noise

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// Simplex is a 2D simplex noise field with its own permutation table.
type Simplex struct {
	perm [512]uint8
}

// New builds a field from a 32-byte seed. The permutation is a Fisher-Yates
// shuffle driven by a hash expansion of the seed, so two different seeds give
// unrelated fields and the same seed always gives the same one.
//
// The shuffle needs more bytes than the seed holds, and mixing the seed's own
// bytes together to stretch them is a trap: XOR-ing two entries of a seed can
// cancel exactly the part that distinguishes it, leaving two different seeds
// with the same permutation. Hashing the seed with a block counter has no such
// structure.
func New(seed [32]byte) *Simplex {
	var s Simplex
	var p [256]uint8
	for i := range p {
		p[i] = uint8(i)
	}

	stream := expand(seed, 256)
	for i, k := 255, 0; i > 0; i, k = i-1, k+1 {
		j := int(stream[k]) % (i + 1)
		p[i], p[j] = p[j], p[i]
	}
	for i := 0; i < 512; i++ {
		s.perm[i] = p[i&255]
	}
	return &s
}

// expand derives n bytes from a seed by hashing it with a block counter.
func expand(seed [32]byte, n int) []byte {
	out := make([]byte, 0, n+sha256.Size)
	var ctr [8]byte
	for block := uint64(0); len(out) < n; block++ {
		h := sha256.New()
		h.Write([]byte("blazon/v1/noise"))
		h.Write([]byte{0})
		h.Write(seed[:])
		binary.BigEndian.PutUint64(ctr[:], block)
		h.Write(ctr[:])
		out = h.Sum(out)
	}
	return out[:n]
}

// Skewing factors for the 2D case.
var (
	f2 = 0.5 * (math.Sqrt(3) - 1)
	g2 = (3 - math.Sqrt(3)) / 6
)

var grad2 = [12][2]float64{
	{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
	{1, 0}, {-1, 0}, {1, 0}, {-1, 0},
	{0, 1}, {0, -1}, {0, 1}, {0, -1},
}

// At returns noise in roughly [-1,1].
func (s *Simplex) At(x, y float64) float64 {
	// Skew the input space to find the simplex cell.
	sk := (x + y) * f2
	i := fastFloor(x + sk)
	j := fastFloor(y + sk)

	t := float64(i+j) * g2
	x0 := x - (float64(i) - t)
	y0 := y - (float64(j) - t)

	// Which of the two triangles of the cell are we in?
	var i1, j1 int
	if x0 > y0 {
		i1 = 1
	} else {
		j1 = 1
	}

	x1 := x0 - float64(i1) + g2
	y1 := y0 - float64(j1) + g2
	x2 := x0 - 1 + 2*g2
	y2 := y0 - 1 + 2*g2

	ii := i & 255
	jj := j & 255

	n0 := s.corner(x0, y0, int(s.perm[ii+int(s.perm[jj])])%12)
	n1 := s.corner(x1, y1, int(s.perm[ii+i1+int(s.perm[jj+j1])])%12)
	n2 := s.corner(x2, y2, int(s.perm[ii+1+int(s.perm[jj+1])])%12)

	// The scale factor brings the sum into roughly [-1,1].
	return 70 * (n0 + n1 + n2)
}

func (s *Simplex) corner(x, y float64, gi int) float64 {
	t := 0.5 - x*x - y*y
	if t < 0 {
		return 0
	}
	t *= t
	g := grad2[gi]
	return t * t * (g[0]*x + g[1]*y)
}

// Fractal sums octaves of noise, each at twice the frequency and half the
// amplitude of the last. One octave alone is too smooth to read as a natural
// ridge field.
func (s *Simplex) Fractal(x, y float64, octaves int) float64 {
	var sum, amp, norm float64
	amp = 1
	freq := 1.0
	for i := 0; i < octaves; i++ {
		sum += amp * s.At(x*freq, y*freq)
		norm += amp
		amp /= 2
		freq *= 2
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}

func fastFloor(v float64) int {
	i := int(v)
	if v < float64(i) {
		return i - 1
	}
	return i
}
