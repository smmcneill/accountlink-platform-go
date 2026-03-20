package persistence

import (
	"context"

	"accountlink-platform-go/internal/app"
	"accountlink-platform-go/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyStore struct {
	pool *pgxpool.Pool
}

func NewIdempotencyStore(pool *pgxpool.Pool) *IdempotencyStore {
	return &IdempotencyStore{pool: pool}
}

func (s *IdempotencyStore) FindByKey(ctx context.Context, key string) (domain.IdempotencyRecord, bool, error) {
	const q = `
SELECT idem_key, request_hash, account_link_id
FROM idempotency_keys
WHERE idem_key = $1`

	var rec domain.IdempotencyRecord
	if err := s.pool.QueryRow(ctx, q, key).Scan(&rec.Key, &rec.RequestHash, &rec.AccountLinkID); err != nil {
		if err == pgx.ErrNoRows {
			return domain.IdempotencyRecord{}, false, nil
		}

		return domain.IdempotencyRecord{}, false, err
	}

	return rec, true, nil
}

func (s *IdempotencyStore) TryInsert(ctx context.Context, tx app.Tx, rec domain.IdempotencyRecord) (bool, error) {
	const q = `
INSERT INTO idempotency_keys (idem_key, request_hash, account_link_id)
VALUES ($1, $2, $3)
ON CONFLICT (idem_key) DO NOTHING`

	ct, err := unwrapTx(tx).Exec(ctx, q, rec.Key, rec.RequestHash, rec.AccountLinkID)
	if err != nil {
		return false, err
	}

	return ct.RowsAffected() == 1, nil
}
