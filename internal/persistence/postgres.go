package persistence

import (
	"context"
	"errors"
	"fmt"

	"accountlink-platform-go/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type (
	PostgresTx struct {
		tx pgx.Tx
	}

	TxManager struct {
		pool *pgxpool.Pool
	}
)

func (t *PostgresTx) Commit(ctx context.Context) error {
	if t.tx == nil {
		return nil
	}
	err := t.tx.Commit(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}
	return err
}

func (t *PostgresTx) Rollback(ctx context.Context) error {
	if t.tx == nil {
		return nil
	}
	err := t.tx.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}
	return err
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (m *TxManager) Begin(ctx context.Context) (domain.Tx, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &PostgresTx{tx: tx}, nil
}

func unwrapTx(tx domain.Tx) pgx.Tx {
	pgtx, ok := tx.(*PostgresTx)
	if !ok || pgtx.tx == nil {
		panic(fmt.Sprintf("unexpected tx type %T", tx))
	}
	return pgtx.tx
}
