package persistence

import (
	"context"

	"accountlink-platform-go/internal/app"
	"accountlink-platform-go/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountLinkStore struct {
	pool *pgxpool.Pool
}

func NewAccountLinkStore(pool *pgxpool.Pool) *AccountLinkStore {
	return &AccountLinkStore{pool: pool}
}

func (s *AccountLinkStore) FindByID(ctx context.Context, id uuid.UUID) (domain.AccountLink, bool, error) {
	const q = `
SELECT id, user_id, external_institution, status
FROM account_links
WHERE id = $1`

	var (
		rowID  uuid.UUID
		userID string
		extIns string
		status string
	)
	if err := s.pool.QueryRow(ctx, q, id).Scan(&rowID, &userID, &extIns, &status); err != nil {
		if err == pgx.ErrNoRows {
			return domain.AccountLink{}, false, nil
		}

		return domain.AccountLink{}, false, err
	}

	link, err := domain.NewAccountLink(rowID, userID, extIns, domain.LinkStatus(status))
	if err != nil {
		return domain.AccountLink{}, false, err
	}

	return link, true, nil
}

func (s *AccountLinkStore) Save(ctx context.Context, tx app.Tx, link domain.AccountLink) (domain.AccountLink, error) {
	const q = `
INSERT INTO account_links (id, user_id, external_institution, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  external_institution = EXCLUDED.external_institution,
  status = EXCLUDED.status,
  updated_at = NOW()`

	_, err := unwrapTx(tx).Exec(ctx, q, link.ID, link.UserID, link.ExternalInstitution, string(link.Status))
	if err != nil {
		return domain.AccountLink{}, err
	}

	return link, nil
}
