package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type LedgerRepository interface {
	Create(
		ctx context.Context,
		arg db.CreateLedgerEntryParams,
	) (db.LedgerEntry, error)

	GetByID(
		ctx context.Context,
		id pgtype.UUID,
	) (db.LedgerEntry, error)

	GetByTransferID(
		ctx context.Context,
		transferID pgtype.UUID,
	) ([]db.LedgerEntry, error)

	GetByAccountID(
		ctx context.Context,
		accountID pgtype.UUID,
		offset int32,
		limit int32,
	) ([]db.LedgerEntry, error)
}

type ledgerRepository struct {
	queries *db.Queries
}

func NewLedgerRepository(queries *db.Queries) LedgerRepository {
	return &ledgerRepository{queries: queries}
}

func (r *ledgerRepository) Create(
	ctx context.Context,
	arg db.CreateLedgerEntryParams,
) (db.LedgerEntry, error) {
	return r.queries.CreateLedgerEntry(ctx, arg)
}

func (r *ledgerRepository) GetByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.LedgerEntry, error) {
	return r.queries.GetLedgerEntry(ctx, id)
}

func (r *ledgerRepository) GetByTransferID(
	ctx context.Context,
	transferID pgtype.UUID,
) ([]db.LedgerEntry, error) {
	return r.queries.ListLedgerEntriesByTransfer(ctx, transferID)
}

func (r *ledgerRepository) GetByAccountID(
	ctx context.Context,
	accountID pgtype.UUID,
	offset int32,
	limit int32,
) ([]db.LedgerEntry, error) {
	return r.queries.ListLedgerEntriesByAccount(ctx, db.ListLedgerEntriesByAccountParams{
		AccountID: accountID,
		Limit:     limit,
		Offset:    offset,
	})
}
