package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type AccountRepository interface {
	Create(
		ctx context.Context,
		arg db.CreateAccountParams,
	) (db.Account, error)

	GetByID(
		ctx context.Context,
		id pgtype.UUID,
	) (db.Account, error)

	LockByID(
		ctx context.Context,
		id pgtype.UUID,
	) (db.Account, error)

	GetByAccountNumber(
		ctx context.Context,
		accountNumber string,
	) (db.Account, error)

	Search(
		ctx context.Context,
		arg db.SearchAccountsParams,
	) ([]db.Account, error)

	UpdateByAccountNumber(
		ctx context.Context,
		arg db.UpdateAccountByAccountNumberParams,
	) (db.Account, error)

	AdjustBalance(
		ctx context.Context,
		id pgtype.UUID,
		delta int64,
	) (db.Account, error)
}

type AccountSort string

type accountRepository struct {
	queries *db.Queries
}

func NewAccountRepository(queries *db.Queries) AccountRepository {
	return &accountRepository{queries: queries}
}

func (r *accountRepository) Create(
	ctx context.Context,
	arg db.CreateAccountParams,
) (db.Account, error) {
	return r.queries.CreateAccount(ctx, arg)
}

func (r *accountRepository) GetByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.Account, error) {
	return r.queries.GetAccount(ctx, id)
}

func (r *accountRepository) LockByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.Account, error) {
	return r.queries.LockAccount(ctx, id)
}

func (r *accountRepository) GetByAccountNumber(
	ctx context.Context,
	accountNumber string,
) (db.Account, error) {
	return r.queries.GetAccountByAccountNumber(ctx, accountNumber)
}

func (r *accountRepository) Search(
	ctx context.Context,
	arg db.SearchAccountsParams,
) ([]db.Account, error) {
	return r.queries.SearchAccounts(ctx, arg)
}

func (r *accountRepository) UpdateByAccountNumber(
	ctx context.Context,
	arg db.UpdateAccountByAccountNumberParams,
) (db.Account, error) {
	return r.queries.UpdateAccountByAccountNumber(ctx, arg)
}

func (r *accountRepository) AdjustBalance(
	ctx context.Context,
	id pgtype.UUID,
	delta int64,
) (db.Account, error) {
	return r.queries.AdjustAccountBalance(ctx, db.AdjustAccountBalanceParams{
		ID:      id,
		Balance: delta,
	})
}
