package blazon

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want Version
	}{
		{"1.4.2", Version{Major: 1, Minor: 4, Patch: 2}},
		{"v1.4.2", Version{Major: 1, Minor: 4, Patch: 2}},
		{"0.0.0", Version{}},
		{"1", Version{Major: 1}},
		{"1.2", Version{Major: 1, Minor: 2}},
		{"  1.4.2  ", Version{Major: 1, Minor: 4, Patch: 2}},
		{"1.0.0-rc.1", Version{Major: 1, Prerelease: "rc.1"}},
		{"1.0.0-alpha-2", Version{Major: 1, Prerelease: "alpha-2"}},
		{"1.0.0+build.5", Version{Major: 1, Build: "build.5"}},
		{"1.0.0+0010", Version{Major: 1, Build: "0010"}},
		{"1.0.0-rc.1+exp.sha.abc", Version{Major: 1, Prerelease: "rc.1", Build: "exp.sha.abc"}},
		{"18446744073709551615.0.0", Version{Major: 1<<64 - 1}},
	}
	for _, c := range cases {
		got, err := ParseVersion(c.in)
		if err != nil {
			t.Errorf("ParseVersion(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseVersion(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseVersionErrors(t *testing.T) {
	bad := []string{
		"", "   ",
		"1.2.3.4",
		"01.2.3",
		"1.02.3",
		"1.2.x",
		"1..3",
		"1.2.3-",
		"1.2.3+",
		"1.2.3-01",
		"1.2.3-rc..1",
		"1.2.3-rc$1",
		"1.2.3+bad!",
		"99999999999999999999.0.0",
	}
	for _, in := range bad {
		if v, err := ParseVersion(in); err == nil {
			t.Errorf("ParseVersion(%q) = %+v, want error", in, v)
		}
	}
}

func TestVersionString(t *testing.T) {
	cases := []string{"1.4.2", "0.0.0", "1.0.0-rc.1", "1.0.0+build.5", "2.3.4-beta.1+sha.abc"}
	for _, in := range cases {
		v, err := ParseVersion(in)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", in, err)
		}
		if got := v.String(); got != in {
			t.Errorf("round trip %q: got %q", in, got)
		}
	}
	// Short forms canonicalise.
	v, _ := ParseVersion("v1.2")
	if got := v.String(); got != "1.2.0" {
		t.Errorf("canonical form of v1.2 = %q, want 1.2.0", got)
	}
}

func TestVersionHelpers(t *testing.T) {
	v, _ := ParseVersion("1.2.3-rc.1+b")
	if !v.IsPrerelease() {
		t.Error("IsPrerelease = false, want true")
	}
	if got, want := v.Core(), (Version{Major: 1, Minor: 2, Patch: 3}); got != want {
		t.Errorf("Core = %+v, want %+v", got, want)
	}
}
