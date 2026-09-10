package blazon

import (
	"testing"
)

func mustSeed(t *testing.T, s string) *Seed {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return NewSeed(v)
}

// TestSeedFieldSeparation is the reason fields are length-prefixed: without
// it, 1.23.4 and 12.3.4 would hash identically.
func TestSeedFieldSeparation(t *testing.T) {
	a := mustSeed(t, "1.23.4")
	b := mustSeed(t, "12.3.4")
	if a.H2 == b.H2 {
		t.Error("1.23.4 and 12.3.4 share a detail hash")
	}
	if a.H0 == b.H0 {
		t.Error("major 1 and major 12 share an archetype hash")
	}
}

func TestSeedCascade(t *testing.T) {
	base := mustSeed(t, "1.4.2")
	patch := mustSeed(t, "1.4.3")
	minor := mustSeed(t, "1.5.2")
	major := mustSeed(t, "2.4.2")

	if base.H0 != patch.H0 || base.H1 != patch.H1 {
		t.Error("patch bump changed the archetype or variant hash")
	}
	if base.H2 == patch.H2 {
		t.Error("patch bump left the detail hash unchanged")
	}
	if base.H0 != minor.H0 {
		t.Error("minor bump changed the archetype hash")
	}
	if base.H1 == minor.H1 {
		t.Error("minor bump left the variant hash unchanged")
	}
	if base.H0 == major.H0 {
		t.Error("major bump left the archetype hash unchanged")
	}
}

func TestSeedPrereleaseOnlyMovesH3(t *testing.T) {
	rel := mustSeed(t, "1.0.0")
	pre := mustSeed(t, "1.0.0-rc.1")
	if rel.H0 != pre.H0 || rel.H1 != pre.H1 || rel.H2 != pre.H2 {
		t.Error("prerelease changed a core stream; 1.0.0-rc.1 must still look like a 1.0.0")
	}
	if rel.H3 == pre.H3 {
		t.Error("prerelease left the prerelease stream unchanged")
	}
	build := mustSeed(t, "1.0.0+abc")
	if rel.H3 == build.H3 {
		t.Error("build metadata left the prerelease stream unchanged")
	}
}

func TestParamStreamsAreIndependent(t *testing.T) {
	p := NewParams(mustSeed(t, "1.4.2"), PolicyFamilyAmplified)

	// Drawing from one stream must not shift another.
	want := NewParams(mustSeed(t, "1.4.2"), PolicyFamilyAmplified).Variant().Uint(64)
	for i := 0; i < 5; i++ {
		p.Archetype().Uint(64)
		p.Detail().Uint(64)
	}
	if got := p.Variant().Uint(64); got != want {
		t.Error("draws from one stream disturbed another")
	}
}

func TestParamStreamsDifferUnderIndependent(t *testing.T) {
	// All streams share H2 here; the stream name must still separate them.
	p := NewParams(mustSeed(t, "1.4.2"), PolicyIndependent)
	a := p.Archetype().Uint(64)
	v := p.Variant().Uint(64)
	d := p.Detail().Uint(64)
	if a == v || v == d || a == d {
		t.Error("streams collided under PolicyIndependent")
	}
}

func TestPolicyRouting(t *testing.T) {
	draw := func(ver string, pol Policy, pick func(*Params) *BitReader) uint64 {
		return pick(NewParams(mustSeed(t, ver), pol)).Uint(64)
	}
	amp := func(p *Params) *BitReader { return p.Amplified() }
	arch := func(p *Params) *BitReader { return p.Archetype() }

	// Amplified follows the patch by default...
	if draw("1.4.2", PolicyFamilyAmplified, amp) == draw("1.4.3", PolicyFamilyAmplified, amp) {
		t.Error("PolicyFamilyAmplified: patch bump did not move the amplified stream")
	}
	// ...and the minor under the plain family policy.
	if draw("1.4.2", PolicyFamily, amp) != draw("1.4.3", PolicyFamily, amp) {
		t.Error("PolicyFamily: patch bump moved the amplified stream")
	}
	if draw("1.4.2", PolicyFamily, amp) == draw("1.5.2", PolicyFamily, amp) {
		t.Error("PolicyFamily: minor bump did not move the amplified stream")
	}
	// Archetype is stable across patches unless the policy says otherwise.
	if draw("1.4.2", PolicyFamilyAmplified, arch) != draw("1.4.3", PolicyFamilyAmplified, arch) {
		t.Error("archetype moved on a patch bump under a family policy")
	}
	if draw("1.4.2", PolicyIndependent, arch) == draw("1.4.3", PolicyIndependent, arch) {
		t.Error("PolicyIndependent: archetype did not move on a patch bump")
	}
}

func TestParamsReset(t *testing.T) {
	p := NewParams(mustSeed(t, "3.1.4"), PolicyFamilyAmplified)
	first := p.Variant().Uint(64)
	p.Reset()
	if second := p.Variant().Uint(64); first != second {
		t.Errorf("Reset did not restore the stream: %d != %d", first, second)
	}
}

func TestPolicyString(t *testing.T) {
	for _, c := range []struct {
		p    Policy
		want string
	}{
		{PolicyFamilyAmplified, "family-amplified"},
		{PolicyFamily, "family"},
		{PolicyIndependent, "independent"},
		{Policy(99), "unknown"},
	} {
		if got := c.p.String(); got != c.want {
			t.Errorf("Policy(%d).String() = %q, want %q", c.p, got, c.want)
		}
	}
}
