package app

import (
	"testing"
)

func TestSHA256Hex(t *testing.T) {
	tests := map[string]struct {
		inputA   string
		inputB   string
		validate func(t *testing.T, outputA string, outputB string)
	}{
		"stable": {
			inputA: "user-123|Chase",
			inputB: "user-123|Chase",
			validate: func(t *testing.T, outputA string, outputB string) {
				t.Helper()
				if outputA != outputB {
					t.Fatalf("expected deterministic output, got %q and %q", outputA, outputB)
				}
			},
		},
		"changes with input": {
			inputA: "user-123|Chase",
			inputB: "user-999|Chase",
			validate: func(t *testing.T, outputA string, outputB string) {
				t.Helper()
				if outputA == outputB {
					t.Fatalf("expected different hashes for different inputs")
				}
			},
		},
		"has expected length": {
			inputA: "abc",
			inputB: "",
			validate: func(t *testing.T, outputA string, outputB string) {
				t.Helper()
				_ = outputB
				if len(outputA) != 64 {
					t.Fatalf("expected sha256 hex length 64, got %d", len(outputA))
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			outputA := sha256Hex(tc.inputA)
			outputB := sha256Hex(tc.inputB)
			tc.validate(t, outputA, outputB)
		})
	}
}
