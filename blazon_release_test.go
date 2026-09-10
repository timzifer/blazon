package blazon

import "testing"

// TestReleaseIsAVersion guards the constant the release workflow compares its
// tag against: a malformed value there would only be noticed at tag time.
func TestReleaseIsAVersion(t *testing.T) {
	v, err := ParseVersion(Release)
	if err != nil {
		t.Fatalf("Release = %q does not parse: %v", Release, err)
	}
	if v.String() != Release {
		t.Errorf("Release = %q is not canonical; want %q", Release, v.String())
	}
}

func TestLogoRendererExists(t *testing.T) {
	if _, ok := Lookup(LogoRenderer); !ok {
		t.Errorf("LogoRenderer = %q is not registered", LogoRenderer)
	}
	if _, err := SVG(Release, Options{Renderer: LogoRenderer, Size: 64}); err != nil {
		t.Errorf("rendering the project's own logo failed: %v", err)
	}
}
