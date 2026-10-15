package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Roy6lty/minibank/database/repository"
	"github.com/Roy6lty/minibank/internal/audit"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	accountModels "github.com/Roy6lty/minibank/modules/account"
	accountService "github.com/Roy6lty/minibank/modules/account/services"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	ledgerService "github.com/Roy6lty/minibank/modules/ledger/services"
	models "github.com/Roy6lty/minibank/modules/transfers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TransferService interface {
	CreateTransfer(
		ctx context.Context,
		principal authModels.Principal,
		request models.CreateTransferRequest,
	) error

	GetTransfer(
		ctx context.Context,
		principal authModels.Principal,
		request models.GetTransferDetailsRequest,
	) (models.TransferRequestResponse, error)

	GetTransferByReferenceID(
		ctx context.Context,
		principal authModels.Principal,
		request models.GetTransferDetailsRequest,
	) (models.TransferRequestResponse, error)

	GetTransfers(
		ctx context.Context,
		principal authModels.Principal,
		accountNumber string,
		offset int32,
		limit int32,
	) ([]models.TransferRequestResponse, error)

	UpdateTransfers(
		ctx context.Context,
		principal authModels.Principal,
		transferID string,
		request models.UpdateTransferRequest,
	) error
}
type transferService struct {
	accounts  accountService.AccountService
	transfers repository.TransferRepository
	uow       repository.UnitOfWork
}

func NewTransferService(
	accounts accountService.AccountService,
	transfers repository.TransferRepository,
	uow repository.UnitOfWork,
) TransferService {
	return &transferService{
		accounts:  accounts,
		transfers: transfers,
		uow:       uow,
	}
}

func ValidateTransfer() {
	/**
	    1. Both account mt be of the same currency
		2. The balance of the sending account must be greater than the amount being sent
		3. The minimum balanace must not be excedded
		4. The account being sent to must not be suspended (status)
		5. The account being used must not be susupended
	*/
}

func TransferCurrencyCheck(account1, account2 accountModels.TransferAccount) error {
	if account1.Currency != account2.Currency {
		return fmt.Errorf("accounts are of different currency")
	}
	return nil
}

func transferMatchesRequest(
	transfer db.Transfer,
	source accountModels.TransferAccount,
	destination accountModels.TransferAccount,
	request models.CreateTransferRequest,
) bool {
	return transfer.Reference == request.IdempotencyKey &&
		transfer.SourceAccountID == source.ID &&
		transfer.DestinationAccountID == destination.ID &&
		transfer.Amount == request.Amount &&
		transfer.Currency == source.Currency
}

func (t *transferService) CreateTransfer(
	ctx context.Context,
	principal authModels.Principal,
	request models.CreateTransferRequest,
) error {

	if request.SenderAccountNumber == "" ||
		request.RecieverAccountNumber == "" {
		return fmt.Errorf("sender and receiver account numbers are required")
	}
	if request.IdempotencyKey == "" {
		return fmt.Errorf("idempotency key is required")
	}

	if request.SenderAccountNumber == request.RecieverAccountNumber {
		return fmt.Errorf("sender and receiver accounts must be different")
	}

	if request.Amount <= 0 {
		return fmt.Errorf("transfer amount must be greater than zero")
	}

	err := t.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		transactionAccounts := accountService.NewAccountService(repos.Accounts)
		sourceAccount, err := transactionAccounts.GetAccountForTransfer(
			ctx,
			request.SenderAccountNumber,
		)
		if err != nil {
			return fmt.Errorf("get sender account: %w", err)
		}

		destinationAccount, err := transactionAccounts.GetAccountForTransfer(
			ctx,
			request.RecieverAccountNumber,
		)
		if err != nil {
			return fmt.Errorf("get receiver account: %w", err)
		}
		if !accountOwnedBy(principal, sourceAccount) {
			return fmt.Errorf("source account not found")
		}

		if sourceAccount.Status != "ACTIVE" {
			return fmt.Errorf("sender account is not active")
		}
		if destinationAccount.Status != "ACTIVE" {
			return fmt.Errorf("receiver account is not active")
		}
		if err := TransferCurrencyCheck(sourceAccount, destinationAccount); err != nil {
			return err
		}

		existing, err := repos.Transfers.GetTransfersByReference(ctx, request.IdempotencyKey)
		if err == nil {
			if !transferMatchesRequest(existing, sourceAccount, destinationAccount, request) {
				return fmt.Errorf("idempotency key is already used for a different transfer")
			}
			if existing.Status == string(models.TransferStatusCompleted) {
				return nil
			}
			return fmt.Errorf("transfer with idempotency key already exists")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check idempotency key: %w", err)
		}

		transactionLedger := ledgerService.NewTransferPostingService(
			transactionAccounts,
			repos.TransferPosting,
			repos.Ledger,
			repos.Transactions,
			repos.Audit,
		)
		_, err = transactionLedger.CreateAndPostTransfer(ctx, db.CreateTransferParams{
			Reference:            request.IdempotencyKey,
			SourceAccountID:      sourceAccount.ID,
			DestinationAccountID: destinationAccount.ID,
			Amount:               request.Amount,
			Currency:             sourceAccount.Currency,
		})
		return err
	})
	if err != nil {
		if existing, lookupErr := t.transfers.GetTransfersByReference(ctx, request.IdempotencyKey); lookupErr == nil {
			// A concurrent request may have won the unique reference constraint.
			sourceAccount, sourceErr := t.accounts.GetAccountForTransfer(ctx, request.SenderAccountNumber)
			destinationAccount, destinationErr := t.accounts.GetAccountForTransfer(ctx, request.RecieverAccountNumber)
			if existing.Status == "COMPLETED" && sourceErr == nil && destinationErr == nil &&
				transferMatchesRequest(existing, sourceAccount, destinationAccount, request) {
				return nil
			}
		}
		return fmt.Errorf("create transfer: %w", err)
	}

	return nil
}

func (t *transferService) GetTransfer(
	ctx context.Context,
	principal authModels.Principal,
	request models.GetTransferDetailsRequest,
) (models.TransferRequestResponse, error) {
	var response models.TransferRequestResponse

	if request.TransferID == nil || *request.TransferID == "" {
		return response, fmt.Errorf("transfer ID is required")
	}

	transferUID, err := uuid.Parse(*request.TransferID)
	if err != nil {
		return response, fmt.Errorf("invalid transfer ID: %w", err)
	}

	transfer, err := t.transfers.GetByID(
		ctx,
		pgtype.UUID{
			Bytes: transferUID,
			Valid: true,
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return response, fmt.Errorf("transfer not found")
		}

		return response, fmt.Errorf("get transfer: %w", err)
	}
	if err := t.ensureTransferAccess(ctx, principal, transfer); err != nil {
		return response, err
	}

	response = models.TransferRequestResponse{
		Reference: transfer.Reference,
		Amount:    transfer.Amount,
		Curreny:   transfer.Currency,
		Status:    transfer.Status,
		CreatedAt: transfer.CreatedAt.Time,
	}

	return response, nil
}

func (t *transferService) GetTransferByReferenceID(
	ctx context.Context,
	principal authModels.Principal,
	request models.GetTransferDetailsRequest,
) (models.TransferRequestResponse, error) {
	var response models.TransferRequestResponse

	if request.ReferenceID == nil || *request.ReferenceID == "" {
		return response, fmt.Errorf("reference ID is required")
	}

	transfer, err := t.transfers.GetTransfersByReference(
		ctx,
		*request.ReferenceID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return response, fmt.Errorf("transfer not found")
		}

		return response, fmt.Errorf("get transfer by reference: %w", err)
	}
	if err := t.ensureTransferAccess(ctx, principal, transfer); err != nil {
		return response, err
	}

	return models.TransferRequestResponse{
		Reference: transfer.Reference,
		Amount:    transfer.Amount,
		Curreny:   transfer.Currency,
		Status:    transfer.Status,
		CreatedAt: transfer.CreatedAt.Time,
	}, nil
}

func (t *transferService) GetTransfers(
	ctx context.Context,
	principal authModels.Principal,
	accountNumber string,
	offset int32,
	limit int32,
) ([]models.TransferRequestResponse, error) {
	if accountNumber == "" {
		return nil, fmt.Errorf("account number is required")
	}

	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}

	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	account, err := t.accounts.GetAccountForTransfer(ctx, accountNumber)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	if !accountOwnedBy(principal, account) {
		return nil, fmt.Errorf("account not found")
	}

	transfers, err := t.transfers.GetByAccountID(
		ctx,
		account.ID,
		offset,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get transfers: %w", err)
	}

	responses := make([]models.TransferRequestResponse, 0, len(transfers))

	for _, transfer := range transfers {
		responses = append(responses, models.TransferRequestResponse{
			Reference:   transfer.Reference,
			Amount:      transfer.Amount,
			Curreny:     transfer.Currency,
			Status:      transfer.Status,
			Description: transfer.Description.String,
			CreatedAt:   transfer.CreatedAt.Time,
		})
	}

	return responses, nil
}

func (t *transferService) UpdateTransfers(
	ctx context.Context,
	principal authModels.Principal,
	transferID string,
	request models.UpdateTransferRequest,
) error {
	if !authModels.IsAdmin(principal) {
		return authModels.ErrForbidden
	}
	if transferID == "" {
		return fmt.Errorf("transfer ID is required")
	}

	transferUID, err := uuid.Parse(transferID)
	if err != nil {
		return fmt.Errorf("invalid transfer ID: %w", err)
	}
	transfer, err := t.transfers.GetByID(ctx, pgtype.UUID{
		Bytes: transferUID,
		Valid: true,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("transfer not found")
		}
		return fmt.Errorf("get transfer: %w", err)
	}
	if !validTransferStatus(request.Status) {
		return fmt.Errorf("invalid transfer status")
	}
	if !canTransitionTransferStatus(models.TransferStatus(transfer.Status), request.Status) {
		return fmt.Errorf("invalid transfer status transition")
	}

	err = t.uow.WithinTx(ctx, func(repos repository.Repositories) error {
		_, err := repos.Transfers.UpdateTransfers(
			ctx,
			pgtype.UUID{
				Bytes: transferUID,
				Valid: true,
			},
			transfer.Status,
			string(request.Status),
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("transfer update conflict")
			}

			return fmt.Errorf("update transfer: %w", err)
		}

		if err := recordAudit(ctx, repos.Audit, audit.Event{
			EventType:    audit.EventTransferStatusUpdated,
			Outcome:      audit.OutcomeSuccess,
			ActorUserID:  principal.UserID,
			RequestID:    audit.RequestID(ctx),
			ResourceType: audit.ResourceTransfer,
			ResourceID:   transferUID,
			Details:      audit.StatusDetails(string(request.Status)),
		}); err != nil {
			return fmt.Errorf("record transfer status update: %w", err)
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

func validTransferStatus(status models.TransferStatus) bool {
	switch status {
	case models.TransferStatusPending,
		models.TransferStatusProcessing,
		models.TransferStatusCompleted,
		models.TransferStatusFailed,
		models.TransferStatusReversed:
		return true
	default:
		return false
	}
}

func canTransitionTransferStatus(
	current models.TransferStatus,
	next models.TransferStatus,
) bool {
	switch current {
	case models.TransferStatusPending:
		return next == models.TransferStatusFailed
	default:
		return false
	}
}

func accountOwnedBy(
	principal authModels.Principal,
	account accountModels.TransferAccount,
) bool {
	return authModels.IsAdmin(principal) ||
		uuid.UUID(account.CustomerID.Bytes) == principal.CustomerID
}

func (t *transferService) ensureTransferAccess(
	ctx context.Context,
	principal authModels.Principal,
	transfer db.Transfer,
) error {
	if authModels.IsAdmin(principal) {
		return nil
	}

	source, err := t.accounts.GetAccountForTransferByID(
		ctx,
		uuid.UUID(transfer.SourceAccountID.Bytes),
	)
	if err != nil {
		return fmt.Errorf("transfer not found")
	}
	destination, err := t.accounts.GetAccountForTransferByID(
		ctx,
		uuid.UUID(transfer.DestinationAccountID.Bytes),
	)
	if err != nil {
		return fmt.Errorf("transfer not found")
	}
	if !accountOwnedBy(principal, source) && !accountOwnedBy(principal, destination) {
		return fmt.Errorf("transfer not found")
	}

	return nil
}
