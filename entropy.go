package blazon

import (
	"encoding/binary"

	"github.com/timzifer/blazon/internal/sha256"
)

// Policy decides how much of a version's identity is inherited from its
// neighbours. It is an API-level choice, not a design detail: it determines
// whether adjacent versions read as a family with a mutation, or as unrelated
// marks.
type Policy int

const (
	// PolicyFamilyAmplified is the default. Major fixes the archetype, minor
	// the variant, patch the detail — but a renderer additionally routes two
	// or three of its highest-contrast parameters (rotation, hue, one shape
	// parameter) through the patch stream. Neighbouring patches stay
	// recognisably related while remaining easy to tell apart at a glance.
	PolicyFamilyAmplified Policy = iota

	// PolicyFamily is the plain cascade. Patch changes only fine detail;
	// high-contrast parameters follow the minor stream.
	PolicyFamily

	// PolicyIndependent derives every stream from the full version. Adjacent
	// versions look unrelated: maximum distinguishability, no family
	// information.
	PolicyIndependent
)

func (p Policy) String() string {
	switch p {
	case PolicyFamilyAmplified:
		return "family-amplified"
	case PolicyFamily:
		return "family"
	case PolicyIndependent:
		return "independent"
	}
	return "unknown"
}

// Seed holds the four separated entropy streams for a version. Splitting the
// hash by version component is what makes family resemblance controllable;
// hashing the whole string at once would not.
type Seed struct {
	V Version

	H0 [32]byte // archetype: major
	H1 [32]byte // variant:   major, minor
	H2 [32]byte // detail:    major, minor, patch
	H3 [32]byte // prerelease: full version including build metadata
}

// NewSeed derives the entropy streams for v.
func NewSeed(v Version) *Seed {
	s := &Seed{V: v}
	s.H0 = hashFields("archetype", num(v.Major))
	s.H1 = hashFields("variant", num(v.Major), num(v.Minor))
	s.H2 = hashFields("detail", num(v.Major), num(v.Minor), num(v.Patch))
	s.H3 = hashFields("prerelease",
		num(v.Major), num(v.Minor), num(v.Patch),
		[]byte(v.Prerelease), []byte(v.Build))
	return s
}

func num(u uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], u)
	return b[:]
}

// hashFields hashes a domain tag followed by length-prefixed fields. The
// length prefix is what keeps "1.23.4" and "12.3.4" apart; naive concatenation
// would collide them.
func hashFields(domain string, fields ...[]byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("blazon/v1/"))
	h.Write([]byte(domain))
	h.Write([]byte{0})
	var n [8]byte
	for _, f := range fields {
		binary.BigEndian.PutUint64(n[:], uint64(len(f)))
		h.Write(n[:])
		h.Write(f)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Params is what a renderer draws its parameters from. Each named stream is a
// separate, lazily created BitReader: draws from one never shift the bits seen
// by another, so a renderer can add a parameter to one stream without
// disturbing the rest of the image.
type Params struct {
	Seed   *Seed
	Policy Policy

	streams map[string]*BitReader
}

// NewParams binds a seed to a policy.
func NewParams(s *Seed, p Policy) *Params {
	return &Params{Seed: s, Policy: p, streams: make(map[string]*BitReader, 5)}
}

// Version is the version being rendered. Renderers that encode the number
// itself rather than its hash — cistercian — read it directly.
func (p *Params) Version() Version { return p.Seed.V }

// Archetype carries the coarsest identity: major only. Parameters that define
// the basic gestalt belong here.
func (p *Params) Archetype() *BitReader { return p.stream("archetype", p.src("archetype")) }

// Variant carries major and minor: the shape family's variation.
func (p *Params) Variant() *BitReader { return p.stream("variant", p.src("variant")) }

// Detail carries the full core version, so it changes on every patch bump.
func (p *Params) Detail() *BitReader { return p.stream("detail", p.src("detail")) }

// Amplified is the stream a renderer uses for its two or three
// highest-contrast parameters. Under the default policy it follows the patch;
// under PolicyFamily it follows the minor, so patch bumps stay subtle. Drawing
// from it rather than branching on Policy keeps renderers policy-agnostic.
func (p *Params) Amplified() *BitReader { return p.stream("amplified", p.src("amplified")) }

// Pre carries prerelease and build metadata. Renderers use it for a marker
// rather than for the main shape, so 1.0.0-rc1 stays visibly a 1.0.0.
func (p *Params) Pre() *BitReader { return p.stream("pre", p.src("pre")) }

// PaletteBase and PaletteShift are the streams the palette draws from. They
// are separate from the renderer streams on purpose: if colour shared the
// archetype stream, switching to monochrome would shift every subsequent draw
// and change the shape of the mark. Form must not depend on whether the mark
// is being rendered in colour.
func (p *Params) PaletteBase() *BitReader { return p.stream("palette/base", p.src("archetype")) }

// PaletteShift follows the amplified stream, so hue moves with the patch under
// the default policy.
func (p *Params) PaletteShift() *BitReader { return p.stream("palette/shift", p.src("amplified")) }

// src maps a stream name to its source hash under the active policy.
func (p *Params) src(name string) [32]byte {
	s := p.Seed
	switch name {
	case "archetype":
		if p.Policy == PolicyIndependent {
			return s.H2
		}
		return s.H0
	case "variant":
		if p.Policy == PolicyIndependent {
			return s.H2
		}
		return s.H1
	case "detail":
		return s.H2
	case "amplified":
		if p.Policy == PolicyFamily {
			return s.H1
		}
		return s.H2
	case "pre":
		return s.H3
	}
	panic("blazon: unknown parameter stream " + name)
}

// stream returns the reader for name, creating it on first use. The stream
// name is mixed into the root so that two streams sharing a source hash — as
// they do under PolicyIndependent — still produce different bits.
func (p *Params) stream(name string, src [32]byte) *BitReader {
	if br, ok := p.streams[name]; ok {
		return br
	}
	br := NewBitReader(hashFields("stream/"+name, src[:]))
	p.streams[name] = br
	return br
}

// Reset discards all drawn bits, so the same Params can render again
// identically.
func (p *Params) Reset() { clear(p.streams) }
