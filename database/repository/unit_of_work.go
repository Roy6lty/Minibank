package repository

import (
	"context"
	"time"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5"
)

type Repositories struct {
	Accounts        AccountRepository
	Customers       CustomerRepository
	Transfers       TransferRepository
	TransferPosting TransferPostingRepository
	Ledger          LedgerRepository
	Users           UserRepository
	RefreshTokens   RefreshTokenRepository
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
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()

	queries := db.New(tx)
	transferRepo := NewTransferRepository(queries)
	repos := Repositories{
		Accounts:        NewAccountRepository(queries),
		Customers:       NewCustomerRepository(queries),
		Transfers:       transferRepo,
		TransferPosting: transferRepo,
		Ledger:          NewLedgerRepository(queries),
		Users:           NewUserRepository(queries),
		RefreshTokens:   NewRefreshTokenRepository(queries),
	}

	if err := work(repos); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
