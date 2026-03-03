package domain

import (
	"errors"

	"github.com/google/uuid"
)

type LinkStatus string

const (
	LinkStatusPending LinkStatus = "PENDING"
	LinkStatusActive  LinkStatus = "ACTIVE"
	LinkStatusFailed  LinkStatus = "FAILED"
)

type AccountLink struct {
	ID                  uuid.UUID  `json:"id"`
	UserID              string     `json:"userId"`
	ExternalInstitution string     `json:"externalInstitution"`
	Status              LinkStatus `json:"status"`
}

func NewAccountLink(id uuid.UUID, userID, externalInstitution string, status LinkStatus) (AccountLink, error) {
	if id == uuid.Nil {
		return AccountLink{}, errors.New("id must not be nil")
	}
	if userID == "" {
		return AccountLink{}, errors.New("userId must not be blank")
	}
	if externalInstitution == "" {
		return AccountLink{}, errors.New("externalInstitution must not be blank")
	}
	if status == "" {
		return AccountLink{}, errors.New("status must not be blank")
	}
	return AccountLink{ID: id, UserID: userID, ExternalInstitution: externalInstitution, Status: status}, nil
}
