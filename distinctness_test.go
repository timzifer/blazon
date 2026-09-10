package blazon_test

import (
	"testing"

	"github.com/timzifer/blazon"
	"github.com/timzifer/blazon/blazontest"
)

// thresholds are frozen regression bounds, one entry per renderer. Each is
// calibrated from the renderer's own distance histogram — run
//
//	go run ./cmd/blazon dist -r <name>
//
// to see it — and then set with a margin below the observed minimum. They are
// not aspirations: a renderer change that pushes any relation below its bound
// is a regression and turns the build red.
//
// The margin is about thirty percent, and it is not padding. Measured minima
// are architecture-dependent: arm64 contracts float expressions that amd64
// evaluates separately, which moves an anti-aliased edge by a fraction of a
// pixel and, on a borderline pair, moves its measured distance by several
// bits. A bound set just under one machine's minimum is not a bound — the CI
// matrix demonstrated exactly that, failing a single polar pair on macOS that
// cleared the same threshold everywhere else. Means are stable to well under a
// bit; minima are not, so the bounds sit clear of them.
//
// The numbers differ per renderer because the shapes do. A dense tiling and a
// single smooth contour occupy the perceptual-hash space very differently, and
// a single global threshold would either be vacuous for one or unreachable for
// the other.
var thresholds = map[string]blazontest.Thresholds{
	// cistercian is an ordered renderer: it encodes the number rather than
	// hashing it, so adjacent versions are meant to look adjacent. The suite
	// skips the distance thresholds for it and checks that no two versions
	// share a mark instead, which is why this entry is empty.
	"cistercian": {},
	// dial and orbit are ordered for the same reason: they write the version
	// in binary, one field per component, with the field boundary standing in
	// for the dot.
	"dial":  {},
	"orbit": {},
	"bishop": {
		MinPatch:        14,
		MinMinor:        23,
		MinMajor:        20,
		Floor:           14,
		PrereleaseRatio: 0.35,
	},
	"flowfield": {
		MinPatch:        14,
		MinMinor:        16,
		MinMajor:        16,
		Floor:           10,
		PrereleaseRatio: 0.35,
	},
	// polar remains the weakest of the six. A radial figure has a small
	// low-frequency vocabulary, and every attempt to widen it traded one
	// relation against another — see the renderer's own notes.
	"polar": {
		MinPatch:        11,
		MinMinor:        11,
		MinMajor:        15,
		Floor:           5,
		PrereleaseRatio: 0.35,
	},
	"superformula": {
		MinPatch:        5,
		MinMinor:        11,
		MinMajor:        18,
		Floor:           2,
		PrereleaseRatio: 0.35,
	},
	"truchet": {
		MinPatch:        16,
		MinMinor:        23,
		MinMajor:        25,
		Floor:           13,
		PrereleaseRatio: 0.35,
	},
}

func TestDistinctness(t *testing.T) {
	for _, name := range blazon.Renderers() {
		th, ok := thresholds[name]
		if !ok {
			t.Errorf("renderer %q has no calibrated thresholds; add an entry to the table", name)
			continue
		}
		r, _ := blazon.Lookup(name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			blazontest.Suite{Renderer: r, Thresholds: th}.Run(t)
		})
	}
}

// TestPolicyChangesFamilyDistance is the policy contract expressed as a
// measurement rather than as documentation: the plain family policy must keep
// patch neighbours closer than the amplified one does, and the independent
// policy must push them furthest apart.
func TestPolicyChangesFamilyDistance(t *testing.T) {
	r, ok := blazon.Lookup("truchet")
	if !ok {
		t.Skip("truchet is not registered")
	}
	mean := func(p blazon.Policy) float64 {
		rep, err := blazontest.Suite{
			Renderer: r,
			Options:  blazon.Options{Policy: p},
		}.Report(nil)
		if err != nil {
			t.Fatal(err)
		}
		return rep.Patch.Mean
	}
	family := mean(blazon.PolicyFamily)
	amplified := mean(blazon.PolicyFamilyAmplified)
	independent := mean(blazon.PolicyIndependent)

	if !(family < amplified) {
		t.Errorf("patch distance under the family policy (%.1f) is not below the amplified one (%.1f)",
			family, amplified)
	}
	if !(amplified <= independent) {
		t.Errorf("patch distance under the amplified policy (%.1f) exceeds the independent one (%.1f)",
			amplified, independent)
	}
}
