package blazon

import (
	"errors"
	"strconv"
	"strings"
)

// Version is a parsed semantic version. Build metadata is retained because it
// feeds the prerelease entropy stream, even though SemVer ignores it for
// ordering.
type Version struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	Prerelease string // without the leading '-'
	Build      string // without the leading '+'
}

var (
	// ErrEmptyVersion is returned when the input contains no version at all.
	ErrEmptyVersion = errors.New("blazon: empty version string")
	// ErrSyntax is returned when the input is not a valid semantic version.
	ErrSyntax = errors.New("blazon: invalid semantic version")
)

// syntaxError reports a malformed version and wraps ErrSyntax, so that
// errors.Is keeps working while the package stays clear of fmt: reflection is
// the single heaviest thing this library could put in a caller's binary, and
// nothing here formats anything but strings.
type syntaxError struct {
	orig   string
	detail string
}

func (e *syntaxError) Error() string {
	return ErrSyntax.Error() + " " + strconv.Quote(e.orig) + ": " + e.detail
}

func (e *syntaxError) Unwrap() error { return ErrSyntax }

// ParseVersion parses a semantic version string. A single leading "v" is
// tolerated because Go module tags carry one. Missing minor and patch
// components default to zero, so "1" and "1.2" are accepted as "1.0.0" and
// "1.2.0" respectively.
func ParseVersion(s string) (Version, error) {
	orig := s
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, ErrEmptyVersion
	}
	if s[0] == 'v' || s[0] == 'V' {
		s = s[1:]
	}

	var v Version

	if i := strings.IndexByte(s, '+'); i >= 0 {
		v.Build = s[i+1:]
		s = s[:i]
		if err := checkDotIdents(v.Build, true); err != nil {
			return Version{}, &syntaxError{orig: orig, detail: "build metadata: " + err.Error()}
		}
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		v.Prerelease = s[i+1:]
		s = s[:i]
		if err := checkDotIdents(v.Prerelease, false); err != nil {
			return Version{}, &syntaxError{orig: orig, detail: "prerelease: " + err.Error()}
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return Version{}, &syntaxError{orig: orig, detail: "too many numeric components"}
	}
	dst := [...]*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := parseNumeric(p)
		if err != nil {
			return Version{}, &syntaxError{orig: orig, detail: err.Error()}
		}
		*dst[i] = n
	}
	return v, nil
}

// parseNumeric parses a SemVer numeric identifier: digits only, no leading
// zero unless the value is exactly "0".
func parseNumeric(p string) (uint64, error) {
	if p == "" {
		return 0, errors.New("empty numeric component")
	}
	for i := 0; i < len(p); i++ {
		if p[i] < '0' || p[i] > '9' {
			return 0, errors.New("non-digit " + strconv.QuoteRune(rune(p[i])) + " in numeric component")
		}
	}
	if len(p) > 1 && p[0] == '0' {
		return 0, errors.New("leading zero in numeric component " + strconv.Quote(p))
	}
	n, err := strconv.ParseUint(p, 10, 64)
	if err != nil {
		return 0, errors.New("numeric component " + strconv.Quote(p) + " out of range")
	}
	return n, nil
}

// checkDotIdents validates a dot-separated identifier list. Prerelease
// identifiers may not carry leading zeroes when purely numeric; build
// metadata identifiers may.
func checkDotIdents(s string, isBuild bool) error {
	if s == "" {
		return errors.New("empty")
	}
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return errors.New("empty identifier")
		}
		numeric := true
		for i := 0; i < len(id); i++ {
			c := id[i]
			switch {
			case c >= '0' && c <= '9':
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '-':
				numeric = false
			default:
				return errors.New("invalid character " + strconv.QuoteRune(rune(c)) +
					" in identifier " + strconv.Quote(id))
			}
		}
		if !isBuild && numeric && len(id) > 1 && id[0] == '0' {
			return errors.New("leading zero in numeric identifier " + strconv.Quote(id))
		}
	}
	return nil
}

// Core reports the version without prerelease or build metadata.
func (v Version) Core() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

// IsPrerelease reports whether the version carries a prerelease identifier.
func (v Version) IsPrerelease() bool { return v.Prerelease != "" }

// String renders the canonical SemVer form.
func (v Version) String() string {
	var b strings.Builder
	b.WriteString(strconv.FormatUint(v.Major, 10))
	b.WriteByte('.')
	b.WriteString(strconv.FormatUint(v.Minor, 10))
	b.WriteByte('.')
	b.WriteString(strconv.FormatUint(v.Patch, 10))
	if v.Prerelease != "" {
		b.WriteByte('-')
		b.WriteString(v.Prerelease)
	}
	if v.Build != "" {
		b.WriteByte('+')
		b.WriteString(v.Build)
	}
	return b.String()
}
