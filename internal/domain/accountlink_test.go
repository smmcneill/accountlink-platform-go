package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewAccountLink_Valid(t *testing.T) {
	id := uuid.New()
	actual, err := NewAccountLink(id, "user-123", "Chase", LinkStatusActive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if actual.ID != id {
		t.Fatalf("expected ID %v, got %v", id, actual.ID)
	}
	if actual.UserID != "user-123" {
		t.Fatalf("expected userID user-123, got %q", actual.UserID)
	}
	if actual.ExternalInstitution != "Chase" {
		t.Fatalf("expected institution Chase, got %q", actual.ExternalInstitution)
	}
	if actual.Status != LinkStatusActive {
		t.Fatalf("expected status %q, got %q", LinkStatusActive, actual.Status)
	}
}

func TestNewAccountLink_RequiresID(t *testing.T) {
	_, err := NewAccountLink(uuid.Nil, "user-123", "Chase", LinkStatusActive)
	if err == nil {
		t.Fatalf("expected error for nil id")
	}
}

func TestNewAccountLink_RequiresUserID(t *testing.T) {
	_, err := NewAccountLink(uuid.New(), "", "Chase", LinkStatusActive)
	if err == nil {
		t.Fatalf("expected error for blank userID")
	}
}

func TestNewAccountLink_RequiresExternalInstitution(t *testing.T) {
	_, err := NewAccountLink(uuid.New(), "user-123", "", LinkStatusActive)
	if err == nil {
		t.Fatalf("expected error for blank externalInstitution")
	}
}

func TestNewAccountLink_RequiresStatus(t *testing.T) {
	_, err := NewAccountLink(uuid.New(), "user-123", "Chase", "")
	if err == nil {
		t.Fatalf("expected error for blank status")
	}
}

func TestNewAccountLink_RejectsUnknownStatus(t *testing.T) {
	_, err := NewAccountLink(uuid.New(), "user-123", "Chase", LinkStatus("UNKNOWN"))
	if err == nil {
		t.Fatalf("expected error for unknown status")
	}
}

func TestAccountLink_Validate_RejectsUnknownStatus(t *testing.T) {
	link := AccountLink{
		ID:                  uuid.New(),
		UserID:              "user-123",
		ExternalInstitution: "Chase",
		Status:              LinkStatus("UNKNOWN"),
	}

	if err := link.Validate(); err == nil {
		t.Fatalf("expected error for unknown status")
	}
}
