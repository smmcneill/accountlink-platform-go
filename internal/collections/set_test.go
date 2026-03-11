package collections

import "testing"

func TestSetTracksPresence(t *testing.T) {
	s := NewSet[string]()
	s.Add("a")

	if !s.Has("a") {
		t.Fatalf("expected value to be present")
	}

	if s.Has("b") {
		t.Fatalf("expected missing value")
	}
}
