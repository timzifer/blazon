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
	"bishop": {
		MinPatch:        17,
		MinMinor:        28,
		MinMajor:        24,
		Floor:           17,
		PrereleaseRatio: 0.35,
	},
	"flowfield": {
		MinPatch:        17,
		MinMinor:        20,
		MinMajor:        19,
		Floor:           12,
		PrereleaseRatio: 0.35,
	},
	// polar is the weakest of the six. Its marks are rings of thin wedges,
	// which reduce to similar low-frequency content however the cells fall,
	// so its floor sits noticeably lower than the rest.
	"polar": {
		MinPatch:        11,
		MinMinor:        13,
		MinMajor:        15,
		Floor:           5,
		PrereleaseRatio: 0.35,
	},
	"superformula": {
		MinPatch:        6,
		MinMinor:        13,
		MinMajor:        22,
		Floor:           2,
		PrereleaseRatio: 0.35,
	},
	"truchet": {
		MinPatch:        20,
		MinMinor:        28,
		MinMajor:        30,
		Floor:           16,
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
