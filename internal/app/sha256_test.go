package app

import (
	"testing"
)

func TestSHA256Hex_Stable(t *testing.T) {
	input := "user-123|Chase"

	a := sha256Hex(input)
	b := sha256Hex(input)

	if a != b {
		t.Fatalf("expected deterministic output, got %q and %q", a, b)
	}
}

func TestSHA256Hex_ChangesWithInput(t *testing.T) {
	a := sha256Hex("user-123|Chase")
	b := sha256Hex("user-999|Chase")

	if a == b {
		t.Fatalf("expected different hashes for different inputs")
	}
}

func TestSHA256Hex_HasExpectedLength(t *testing.T) {
	got := sha256Hex("abc")
	if len(got) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(got))
	}
}
