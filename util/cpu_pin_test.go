package util

import "testing"

func TestCPUPin(t *testing.T) {
	if err := PinTo(0); err != nil {
		t.Fatal(err)
	}
}
