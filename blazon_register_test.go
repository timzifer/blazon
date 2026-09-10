//go:build !tinygo

package blazon

import "testing"

// TestRegisterRejectsDuplicates is kept away from TinyGo, whose recover()
// does not catch a panic raised inside another function and whose t.Skip does
// not unwind — either one turns this test from a check into a crash. The
// behaviour under test is compiler-independent; only this way of observing it
// is not.
func TestRegisterRejectsDuplicates(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("registering a duplicate name did not panic")
		}
	}()
	Register(truchetRenderer{})
}
