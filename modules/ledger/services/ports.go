package services

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransferPort interface {
	Create(context.Context, db.CreateTransferParams) (db.Transfer, error)
	GetByID(context.Context, pgtype.UUID) (db.Transfer, error)
	CompletePendingTransfer(context.Context, pgtype.UUID) (db.Transfer, error)
}

type LedgerPort interface {
	Create(context.Context, db.CreateLedgerEntryParams) (db.LedgerEntry, error)
	GetByID(context.Context, pgtype.UUID) (db.LedgerEntry, error)
	GetByTransferID(context.Context, pgtype.UUID) ([]db.LedgerEntry, error)
	GetByAccountID(context.Context, pgtype.UUID, int32, int32) ([]db.LedgerEntry, error)
}
