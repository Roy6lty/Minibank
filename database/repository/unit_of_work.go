package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5"
)

type Repositories struct {
	Accounts  AccountRepository
	Transfers TransferRepository
	Ledger    LedgerRepository
}

type UnitOfWork interface {
	WithinTx(context.Context, func(Repositories) error) error
}

type txBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

type unitOfWork struct {
	pool txBeginner
}

func NewUnitOfWork(pool txBeginner) UnitOfWork {
	return &unitOfWork{pool: pool}
}

func (u *unitOfWork) WithinTx(
	ctx context.Context,
	work func(Repositories) error,
) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queries := db.New(tx)
	repos := Repositories{
		Accounts:  NewAccountRepository(queries),
		Transfers: NewTransferRepository(queries),
		Ledger:    NewLedgerRepository(queries),
	}

	if err := work(repos); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
