package services

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	accountModels "github.com/Roy6lty/minibank/modules/account"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type AccountPort interface {
	GetAccountForTransferByID(context.Context, uuid.UUID) (accountModels.TransferAccount, error)
	GetLockedAccountsForTransfer(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) (accountModels.TransferAccount, accountModels.TransferAccount, error)
	AdjustBalance(context.Context, uuid.UUID, int64) error
}

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
