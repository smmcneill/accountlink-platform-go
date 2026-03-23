package domain

import (
	"errors"

	"github.com/google/uuid"
)

type (
	LinkStatus string

	AccountLink struct {
		ID                  uuid.UUID  `json:"id"`
		UserID              string     `json:"userId"`
		ExternalInstitution string     `json:"externalInstitution"`
		Status              LinkStatus `json:"status"`
	}
)

const (
	LinkStatusPending LinkStatus = "PENDING"
	LinkStatusActive  LinkStatus = "ACTIVE"
	LinkStatusFailed  LinkStatus = "FAILED"
)

func (s LinkStatus) IsValid() bool {
	switch s {
	case LinkStatusPending, LinkStatusActive, LinkStatusFailed:
		return true
	default:
		return false
	}
}

func (a AccountLink) Validate() error {
	if a.ID == uuid.Nil {
		return errors.New("id must not be nil")
	}

	if a.UserID == "" {
		return errors.New("userId must not be blank")
	}

	if a.ExternalInstitution == "" {
		return errors.New("externalInstitution must not be blank")
	}

	if !a.Status.IsValid() {
		return errors.New("status must be one of: PENDING, ACTIVE, FAILED")
	}

	return nil
}

func NewAccountLink(id uuid.UUID, userID, externalInstitution string, status LinkStatus) (AccountLink, error) {
	link := AccountLink{
		ID:                  id,
		UserID:              userID,
		ExternalInstitution: externalInstitution,
		Status:              status,
	}
	if err := link.Validate(); err != nil {
		return AccountLink{}, err
	}
	return link, nil
}
