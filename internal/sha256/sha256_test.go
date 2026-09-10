package sha256

import (
	"bytes"
	std "crypto/sha256"
	"math/rand"
	"testing"
)

// TestAgreesWithTheStandardLibrary is the only test this package really
// needs: SHA-256 has one right answer, and the reference implementation is
// two lines away. Lengths around the block boundary are enumerated because
// padding is where a hand-written implementation goes wrong.
func TestAgreesWithTheStandardLibrary(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	data := make([]byte, 300)
	rng.Read(data)
	for n := 0; n <= len(data); n++ {
		want := std.Sum256(data[:n])
		if got := Sum256(data[:n]); got != want {
			t.Fatalf("Sum256 of %d bytes = %x, want %x", n, got, want)
		}
	}
}

// TestWritesInAnyChunkingAgree covers the buffering, which the callers
// exercise by hashing several pieces into one digest.
func TestWritesInAnyChunkingAgree(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	data := make([]byte, 1000)
	rng.Read(data)
	want := std.Sum256(data)

	for _, chunk := range []int{1, 7, 63, 64, 65, 128, 999} {
		d := New()
		for i := 0; i < len(data); i += chunk {
			end := i + chunk
			if end > len(data) {
				end = len(data)
			}
			if _, err := d.Write(data[i:end]); err != nil {
				t.Fatal(err)
			}
		}
		if got := d.Sum(nil); !bytes.Equal(got, want[:]) {
			t.Errorf("chunked by %d = %x, want %x", chunk, got, want)
		}
	}
}

// TestSumDoesNotEndTheDigest guards the property the noise permutation
// depends on: it hashes a counter, takes a digest, and keeps writing.
func TestSumDoesNotEndTheDigest(t *testing.T) {
	d := New()
	d.Write([]byte("one"))
	first := d.Sum(nil)
	d.Write([]byte("two"))
	second := d.Sum(nil)

	if want := std.Sum256([]byte("one")); !bytes.Equal(first, want[:]) {
		t.Errorf("first sum = %x, want %x", first, want)
	}
	if want := std.Sum256([]byte("onetwo")); !bytes.Equal(second, want[:]) {
		t.Errorf("second sum = %x, want %x", second, want)
	}
}

// TestSumAppends checks the hash.Hash convention, since callers pass a
// non-empty prefix.
func TestSumAppends(t *testing.T) {
	d := New()
	d.Write([]byte("data"))
	got := d.Sum([]byte("prefix"))
	want := std.Sum256([]byte("data"))
	if string(got[:6]) != "prefix" || !bytes.Equal(got[6:], want[:]) {
		t.Errorf("Sum with prefix = %x", got)
	}
}

// TestResetRestartsTheDigest keeps Reset honest for callers that reuse one.
func TestResetRestartsTheDigest(t *testing.T) {
	d := New()
	d.Write([]byte("throwaway"))
	d.Reset()
	d.Write([]byte("real"))
	want := std.Sum256([]byte("real"))
	if got := d.Sum(nil); !bytes.Equal(got, want[:]) {
		t.Errorf("after Reset = %x, want %x", got, want)
	}
}
