package domain

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyRecord struct {
	Key           string
	RequestHash   string
	AccountLinkID uuid.UUID
}

type OutboxEvent struct {
	ID            uuid.UUID
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       string
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

type PublishedEvent struct {
	OutboxID      uuid.UUID
	EventType     string
	AggregateType string
	AggregateID   string
	CreatedAt     time.Time
	PublishedAt   time.Time
	Payload       string
}
