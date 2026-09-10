package blazon_test

import (
	"testing"

	"github.com/timzifer/blazon"
	"github.com/timzifer/blazon/blazontest"
)

// TestMetricIsResolutionIndependent guards the instrument rather than the
// renderers. Two renderers are enough to cover the two ways a perceptual hash
// goes wrong: polar is all thin high-contrast edges, superformula is large
// solid areas.
func TestMetricIsResolutionIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("renders the corpus twice per renderer")
	}
	for _, name := range []string{"polar", "superformula"} {
		r, ok := blazon.Lookup(name)
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			blazontest.Suite{Renderer: r}.CheckMetricStability(t, nil, 128, 192)
		})
	}
}
