package app

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Ensure background goroutines from processor loop tests are not left running.
	goleak.VerifyTestMain(m)
}
