package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Roy6lty/minibank/database/repository"
	"github.com/Roy6lty/minibank/internal/audit"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	models "github.com/Roy6lty/minibank/modules/transactions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransactionService interface {
	CreateTransaction(
		ctx context.Context,
		args models.CreateTransactionRequest,
	) error

	GetTransactionByID(
		ctx context.Context,
		request models.GetTransactionsRequest,
	) (db.Transaction, error)

	UpdateTransaction(
		ctx context.Context,
		id string,
		request models.UpdateTransactionRequest,
	) error
}

type transactionService struct {
	transactions repository.TransactionRepository
	uow          repository.UnitOfWork
}

func NewTransactionService(
	transaction repository.TransactionRepository,
	uow repository.UnitOfWork,
) TransactionService {
	return &transactionService{transactions: transaction, uow: uow}
}

func validStatus(status string) bool {
	switch models.TransactionsStatus(status) {
	case models.TransactionsStatusPending,
		models.TransactionsStatusProcessing,
		models.TransactionsStatusCompleted,
		models.TransactionsStatusFailed,
		models.TransactionsStatusReversed:
		return true
	default:
		return false
	}
}

func (t *transactionService) CreateTransaction(
	ctx context.Context,
	args models.CreateTransactionRequest,
) error {
	if args.Reference == "" {
		return fmt.Errorf("transaction reference is required")
	}
	if args.Type == "" {
		return fmt.Errorf("transaction type is required")
	}
	if args.Type == models.TransactionTypeTransfer {
		return fmt.Errorf("transfer transactions are created with a transfer")
	}
	if args.Amount <= 0 {
		return fmt.Errorf("transaction amount must be greater than zero")
	}
	if len(args.Currency) != 3 {
		return fmt.Errorf("transaction currency must be 3 characters")
	}

	status := args.Status
	if status == "" {
		status = string(models.TransactionsStatusPending)
	}
	if !validStatus(status) {
		return fmt.Errorf("invalid transaction status")
	}

	err := t.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		transaction, err := repos.Transactions.Create(ctx, db.CreateTransactionParams{
			Reference:       args.Reference,
			TransactionType: args.Type,
			Status:          status,
			Amount:          args.Amount,
			Currency:        args.Currency,
			Description: pgtype.Text{
				String: args.Description,
				Valid:  args.Description != "",
			},
		})
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		actorUserID, _ := audit.ActorUserID(ctx)
		if err := recordAudit(ctx, repos.Audit, audit.Event{
			EventType:    audit.EventTransactionCreated,
			Outcome:      audit.OutcomeSuccess,
			ActorUserID:  actorUserID,
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceTransaction,
			ResourceID:   uuid.UUID(transaction.ID.Bytes),
		}); err != nil {
			return fmt.Errorf("record transaction creation: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (t *transactionService) GetTransactionByID(
	ctx context.Context,
	request models.GetTransactionsRequest,
) (db.Transaction, error) {
	if request.ID == nil || *request.ID == "" {
		return db.Transaction{}, fmt.Errorf("transaction ID is required")
	}

	transactionID, err := uuid.Parse(*request.ID)
	if err != nil {
		return db.Transaction{}, fmt.Errorf("invalid transaction ID: %w", err)
	}

	transaction, err := t.transactions.GetByID(ctx, pgtype.UUID{
		Bytes: transactionID,
		Valid: true,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Transaction{}, fmt.Errorf("transaction not found")
		}
		return db.Transaction{}, fmt.Errorf("get transaction: %w", err)
	}

	return transaction, nil
}

func (t *transactionService) UpdateTransaction(
	ctx context.Context,
	id string,
	request models.UpdateTransactionRequest,
) error {
	if id == "" {
		return fmt.Errorf("transaction ID is required")
	}
	if request.Status == nil || !validStatus(string(*request.Status)) {
		return fmt.Errorf("valid transaction status is required")
	}

	transactionID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid transaction ID: %w", err)
	}

	err = t.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		_, err := repos.Transactions.UpdateTransactionStatus(ctx, pgtype.UUID{
			Bytes: transactionID,
			Valid: true,
		}, string(*request.Status))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("transaction not found")
			}
			return fmt.Errorf("update transaction: %w", err)
		}

		actorUserID, _ := audit.ActorUserID(ctx)
		if err := recordAudit(ctx, repos.Audit, audit.Event{
			EventType:    audit.EventTransactionStatusUpdated,
			Outcome:      audit.OutcomeSuccess,
			ActorUserID:  actorUserID,
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceTransaction,
			ResourceID:   transactionID,
			Details:      audit.StatusDetails(string(*request.Status)),
		}); err != nil {
			return fmt.Errorf("record transaction status update: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func recordAudit(ctx context.Context, recorder audit.Recorder, event audit.Event) error {
	if recorder == nil {
		return nil
	}
	return recorder.Record(ctx, event)
}
