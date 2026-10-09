package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransferRepository interface {
	Create(
		ctx context.Context,
		arg db.CreateTransferParams,
	) (db.Transfer, error)

	GetByID(
		ctx context.Context,
		id pgtype.UUID,
	) (db.Transfer, error)

	GetForPosting(
		ctx context.Context,
		id pgtype.UUID,
	) (db.Transfer, error)

	GetTransfersByReference(
		ctx context.Context,
		referenceID string,
	) (db.Transfer, error)

	GetAllTransfers(
		ctx context.Context,
		offset int32,
		limit int32,
	) ([]db.Transfer, error)

	GetAllTransfersToAndFrom(
		ctx context.Context,
		sourceID pgtype.UUID,
		destinationID pgtype.UUID,
		limit int32,
		offset int32,
	) ([]db.Transfer, error)

	GetOutGoingTransfers(
		ctx context.Context,
		sourceID pgtype.UUID,
		offset int32,
		limit int32,
	) ([]db.Transfer, error)

	GetByAccountID(
		ctx context.Context,
		accountID pgtype.UUID,
		offset int32,
		limit int32,
	) ([]db.Transfer, error)

	GetAllIncomingAndOutGoingTransfers(
		ctx context.Context,
		accountNumber string,
		offset int32,
		limit int32,
	) ([]db.Transfer, error)

	UpdateTransfers(
		ctx context.Context,
		id pgtype.UUID,
		currentStatus string,
		status string,
	) (db.Transfer, error)
}

type transferRepository struct {
	queries *db.Queries
}

func NewTransferRepository(queries *db.Queries) TransferRepository {
	return &transferRepository{queries: queries}
}

func (t *transferRepository) Create(
	ctx context.Context,
	arg db.CreateTransferParams,
) (db.Transfer, error) {
	return t.queries.CreateTransfer(ctx, arg)
}

func (t *transferRepository) GetByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.Transfer, error) {
	return t.queries.GetTransfer(ctx, id)
}

func (t *transferRepository) GetForPosting(
	ctx context.Context,
	id pgtype.UUID,
) (db.Transfer, error) {
	return t.queries.GetTransferForPosting(ctx, id)
}

func (t *transferRepository) GetTransfersByReference(
	ctx context.Context,
	referenceID string,
) (db.Transfer, error) {
	return t.queries.GetTransferByReference(ctx, referenceID)
}

func (t *transferRepository) GetAllTransfers(
	ctx context.Context,
	offset int32,
	limit int32,
) ([]db.Transfer, error) {
	return t.queries.ListTransfers(ctx, db.ListTransfersParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (t *transferRepository) GetAllIncomingAndOutGoingTransfers(
	ctx context.Context,
	accountNumber string,
	offset int32,
	limit int32,
) ([]db.Transfer, error) {
	return t.queries.ListIncomingAndOutgoingTransfers(ctx, db.ListIncomingAndOutgoingTransfersParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (t *transferRepository) GetAllTransfersToAndFrom(
	ctx context.Context,
	sourceID pgtype.UUID,
	destinationID pgtype.UUID,
	limit int32,
	offset int32,
) ([]db.Transfer, error) {
	return t.queries.ListTransfersBetweenAccounts(ctx, db.ListTransfersBetweenAccountsParams{
		SourceAccountID:      sourceID,
		DestinationAccountID: destinationID,
		Limit:                limit,
		Offset:               offset,
	})
}

func (t *transferRepository) GetOutGoingTransfers(
	ctx context.Context,
	sourceID pgtype.UUID,
	offset int32,
	limit int32,
) ([]db.Transfer, error) {
	return t.queries.ListOutgoingTransfers(ctx, db.ListOutgoingTransfersParams{
		SourceAccountID: sourceID,
		Limit:           limit,
		Offset:          offset,
	})
}

func (t *transferRepository) GetByAccountID(
	ctx context.Context,
	accountID pgtype.UUID,
	offset int32,
	limit int32,
) ([]db.Transfer, error) {
	return t.queries.ListTransfersByAccount(ctx, db.ListTransfersByAccountParams{
		SourceAccountID: accountID,
		Limit:           limit,
		Offset:          offset,
	})
}

func (t *transferRepository) UpdateTransfers(
	ctx context.Context,
	id pgtype.UUID,
	currentStatus string,
	status string,
) (db.Transfer, error) {
	return t.queries.UpdateTransferStatus(ctx, db.UpdateTransferStatusParams{
		NewStatus:     status,
		ID:            id,
		CurrentStatus: currentStatus,
	})
}
