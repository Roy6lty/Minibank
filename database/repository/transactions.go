package repository

import (
	"context"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransactionRepository interface {
	Create(
		ctx context.Context,
		arg db.CreateTransactionParams,
	) (db.Transaction, error)

	GetByID(
		ctx context.Context,
		id pgtype.UUID,
	) (db.Transaction, error)

	GetByReference(
		ctx context.Context,
		reference string,
	) (db.Transaction, error)

	GetAllTransactions(
		ctx context.Context,
		offset int32,
		limit int32,
	) ([]db.Transaction, error)

	GetTransactionsByType(
		ctx context.Context,
		transactionType string,
		offset int32,
		limit int32,
	) ([]db.Transaction, error)

	GetTransactionsByStatus(
		ctx context.Context,
		status string,
		offset int32,
		limit int32,
	) ([]db.Transaction, error)

	UpdateTransactionStatus(
		ctx context.Context,
		id pgtype.UUID,
		status string,
	) (db.Transaction, error)
}

type transactionRepository struct {
	queries *db.Queries
}

func NewTransactionRepository(queries *db.Queries) TransactionRepository {
	return &transactionRepository{queries: queries}
}

func (r *transactionRepository) Create(
	ctx context.Context,
	arg db.CreateTransactionParams,
) (db.Transaction, error) {
	return r.queries.CreateTransaction(ctx, arg)
}

func (r *transactionRepository) GetByID(
	ctx context.Context,
	id pgtype.UUID,
) (db.Transaction, error) {
	return r.queries.GetTransaction(ctx, id)
}

func (r *transactionRepository) GetByReference(
	ctx context.Context,
	reference string,
) (db.Transaction, error) {
	return r.queries.GetTransactionByReference(ctx, reference)
}

func (r *transactionRepository) GetAllTransactions(
	ctx context.Context,
	offset int32,
	limit int32,
) ([]db.Transaction, error) {
	return r.queries.GetTransactions(ctx, db.GetTransactionsParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *transactionRepository) GetTransactionsByType(
	ctx context.Context,
	transactionType string,
	offset int32,
	limit int32,
) ([]db.Transaction, error) {
	return r.queries.GetTransactionsByType(ctx, db.GetTransactionsByTypeParams{
		TransactionType: transactionType,
		Limit:           limit,
		Offset:          offset,
	})
}

func (r *transactionRepository) GetTransactionsByStatus(
	ctx context.Context,
	status string,
	offset int32,
	limit int32,
) ([]db.Transaction, error) {
	return r.queries.GetTransactionsByStatus(ctx, db.GetTransactionsByStatusParams{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *transactionRepository) UpdateTransactionStatus(
	ctx context.Context,
	id pgtype.UUID,
	status string,
) (db.Transaction, error) {
	return r.queries.UpdateTransactionStatus(ctx, db.UpdateTransactionStatusParams{
		ID:     id,
		Status: status,
	})
}
