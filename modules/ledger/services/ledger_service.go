package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Roy6lty/minibank/internal/audit"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	ledgerModels "github.com/Roy6lty/minibank/modules/ledger"
	transactionModels "github.com/Roy6lty/minibank/modules/transactions"
	transferModels "github.com/Roy6lty/minibank/modules/transfers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type LedgerService interface {
	GetBalance(
		ctx context.Context,
		principal authModels.Principal,
		accountID uuid.UUID,
	) (int64, error)

	GetEntryByID(
		ctx context.Context,
		principal authModels.Principal,
		entryID uuid.UUID,
	) (db.LedgerEntry, error)

	GetEntriesByTransfer(
		ctx context.Context,
		principal authModels.Principal,
		transferID uuid.UUID,
	) ([]db.LedgerEntry, error)

	GetEntriesByAccount(
		ctx context.Context,
		principal authModels.Principal,
		accountID uuid.UUID,
		offset int32,
		limit int32,
	) ([]db.LedgerEntry, error)
}

type TransferPostingService interface {
	CreateAndPostTransfer(
		ctx context.Context,
		arg db.CreateTransferParams,
	) (db.Transfer, error)
}

type ledgerService struct {
	accounts     AccountPort
	transfers    TransferPort
	ledger       LedgerPort
	transactions TransactionPort
	audits       audit.Recorder
}

func NewLedgerService(
	accounts AccountPort,
	transfers TransferPort,
	ledger LedgerPort,
	transactions TransactionPort,
) LedgerService {
	return newLedgerService(accounts, transfers, ledger, transactions, nil)
}

func NewTransferPostingService(
	accounts AccountPort,
	transfers TransferPort,
	ledger LedgerPort,
	transactions TransactionPort,
	audits audit.Recorder,
) TransferPostingService {
	return newLedgerService(accounts, transfers, ledger, transactions, audits)
}

func newLedgerService(
	accounts AccountPort,
	transfers TransferPort,
	ledger LedgerPort,
	transactions TransactionPort,
	audits audit.Recorder,
) *ledgerService {
	return &ledgerService{
		accounts:     accounts,
		transfers:    transfers,
		ledger:       ledger,
		transactions: transactions,
		audits:       audits,
	}
}

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidFromPgUUID(id pgtype.UUID) uuid.UUID {
	return uuid.UUID(id.Bytes)
}

func actorUserID(ctx context.Context) uuid.UUID {
	userID, _ := audit.ActorUserID(ctx)
	return userID
}

func (l *ledgerService) recordAudit(ctx context.Context, event audit.Event) error {
	if l.audits == nil {
		return fmt.Errorf("audit repository is not configured")
	}
	return l.audits.Record(ctx, event)
}

func (l *ledgerService) GetBalance(
	ctx context.Context,
	principal authModels.Principal,
	accountID uuid.UUID,
) (int64, error) {
	account, err := l.accounts.GetAccountForTransferByID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if !accountOwnedBy(principal, account.CustomerID) {
		return 0, fmt.Errorf("account not found")
	}
	return account.Balance, nil
}

func (l *ledgerService) CreateAndPostTransfer(
	ctx context.Context,
	arg db.CreateTransferParams,
) (db.Transfer, error) {
	if arg.Reference == "" {
		return db.Transfer{}, fmt.Errorf("transfer idempotency key is required")
	}
	if arg.Amount <= 0 {
		return db.Transfer{}, fmt.Errorf("transfer amount must be greater than zero")
	}
	if arg.SourceAccountID == arg.DestinationAccountID {
		return db.Transfer{}, fmt.Errorf("source and destination accounts must be different")
	}
	if arg.Currency == "" {
		return db.Transfer{}, fmt.Errorf("transfer currency is required")
	}

	transfer, err := l.transfers.Create(ctx, arg)
	if err != nil {
		return db.Transfer{}, fmt.Errorf("create transfer: %w", err)
	}
	if err := l.recordAudit(ctx, audit.Event{
		EventType:    audit.EventTransferCreated,
		Outcome:      audit.OutcomeSuccess,
		ActorUserID:  actorUserID(ctx),
		RequestID:    audit.RequestID(ctx),
		ResourceType: audit.ResourceTransfer,
		ResourceID:   uuidFromPgUUID(transfer.ID),
	}); err != nil {
		return db.Transfer{}, fmt.Errorf("record transfer creation: %w", err)
	}

	transaction, err := l.transactions.Create(ctx, db.CreateTransactionParams{
		TransferID:      transfer.ID,
		Reference:       transfer.Reference,
		TransactionType: transactionModels.TransactionTypeTransfer,
		Status:          string(transactionModels.TransactionsStatusPending),
		Amount:          transfer.Amount,
		Currency:        transfer.Currency,
		Description:     transfer.Description,
	})
	if err != nil {
		return db.Transfer{}, fmt.Errorf("create transaction record: %w", err)
	}
	if err := l.recordAudit(ctx, audit.Event{
		EventType:    audit.EventTransactionCreated,
		Outcome:      audit.OutcomeSuccess,
		ActorUserID:  actorUserID(ctx),
		RequestID:    audit.RequestID(ctx),
		ResourceType: audit.ResourceTransaction,
		ResourceID:   uuidFromPgUUID(transaction.ID),
	}); err != nil {
		return db.Transfer{}, fmt.Errorf("record transaction creation: %w", err)
	}

	return l.postTransfer(ctx, transfer, transaction.ID)
}

func (l *ledgerService) postTransfer(
	ctx context.Context,
	transfer db.Transfer,
	transactionID pgtype.UUID,
) (db.Transfer, error) {
	if transfer.Status != string(transferModels.TransferStatusPending) {
		return db.Transfer{}, fmt.Errorf("transfer has already been posted or is not pending")
	}

	sourceAccount, destinationAccount, err := l.accounts.GetLockedAccountsForTransfer(
		ctx,
		uuidFromPgUUID(transfer.SourceAccountID),
		uuidFromPgUUID(transfer.DestinationAccountID),
	)
	if err != nil {
		return db.Transfer{}, err
	}
	if sourceAccount.Status != "ACTIVE" || destinationAccount.Status != "ACTIVE" {
		return db.Transfer{}, fmt.Errorf("both accounts must be active")
	}
	if sourceAccount.Currency != transfer.Currency ||
		destinationAccount.Currency != transfer.Currency {
		return db.Transfer{}, fmt.Errorf("transfer currency does not match account currency")
	}
	if sourceAccount.Balance < transfer.Amount {
		return db.Transfer{}, fmt.Errorf("insufficient balance")
	}

	if err := l.accounts.AdjustBalance(
		ctx,
		uuidFromPgUUID(transfer.SourceAccountID),
		-transfer.Amount,
	); err != nil {
		return db.Transfer{}, fmt.Errorf("debit account balance: %w", err)
	}
	if err := l.accounts.AdjustBalance(
		ctx,
		uuidFromPgUUID(transfer.DestinationAccountID),
		transfer.Amount,
	); err != nil {
		return db.Transfer{}, fmt.Errorf("credit account balance: %w", err)
	}

	if _, err := l.ledger.Create(ctx, db.CreateLedgerEntryParams{
		TransferID: transfer.ID,
		AccountID:  transfer.SourceAccountID,
		EntryType:  string(ledgerModels.EntryTypeDebit),
		Amount:     transfer.Amount,
		Currency:   transfer.Currency,
	}); err != nil {
		return db.Transfer{}, fmt.Errorf("create debit ledger entry: %w", err)
	}
	if _, err := l.ledger.Create(ctx, db.CreateLedgerEntryParams{
		TransferID: transfer.ID,
		AccountID:  transfer.DestinationAccountID,
		EntryType:  string(ledgerModels.EntryTypeCredit),
		Amount:     transfer.Amount,
		Currency:   transfer.Currency,
	}); err != nil {
		return db.Transfer{}, fmt.Errorf("create credit ledger entry: %w", err)
	}

	completed, err := l.transfers.CompletePendingTransfer(
		ctx,
		transfer.ID,
	)
	if err != nil {
		return db.Transfer{}, fmt.Errorf("complete transfer: %w", err)
	}
	if _, err := l.transactions.UpdateTransactionStatus(
		ctx,
		transactionID,
		string(transactionModels.TransactionsStatusCompleted),
	); err != nil {
		return db.Transfer{}, fmt.Errorf("complete transaction record: %w", err)
	}
	if err := l.recordAudit(ctx, audit.Event{
		EventType:    audit.EventTransferStatusUpdated,
		Outcome:      audit.OutcomeSuccess,
		ActorUserID:  actorUserID(ctx),
		RequestID:    audit.RequestID(ctx),
		ResourceType: audit.ResourceTransfer,
		ResourceID:   uuidFromPgUUID(completed.ID),
		Details:      audit.StatusDetails(string(transferModels.TransferStatusCompleted)),
	}); err != nil {
		return db.Transfer{}, fmt.Errorf("record transfer completion: %w", err)
	}
	if err := l.recordAudit(ctx, audit.Event{
		EventType:    audit.EventTransactionStatusUpdated,
		Outcome:      audit.OutcomeSuccess,
		ActorUserID:  actorUserID(ctx),
		RequestID:    audit.RequestID(ctx),
		ResourceType: audit.ResourceTransaction,
		ResourceID:   uuidFromPgUUID(transactionID),
		Details:      audit.StatusDetails(string(transactionModels.TransactionsStatusCompleted)),
	}); err != nil {
		return db.Transfer{}, fmt.Errorf("record transaction completion: %w", err)
	}
	return completed, nil
}

func (l *ledgerService) GetEntryByID(
	ctx context.Context,
	principal authModels.Principal,
	entryID uuid.UUID,
) (db.LedgerEntry, error) {
	entry, err := l.ledger.GetByID(ctx, toPgUUID(entryID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.LedgerEntry{}, fmt.Errorf("ledger entry not found")
		}
		return db.LedgerEntry{}, err
	}
	if err := l.ensureAccountAccess(ctx, principal, uuidFromPgUUID(entry.AccountID)); err != nil {
		return db.LedgerEntry{}, fmt.Errorf("ledger entry not found")
	}
	return entry, nil
}

func (l *ledgerService) GetEntriesByTransfer(
	ctx context.Context,
	principal authModels.Principal,
	transferID uuid.UUID,
) ([]db.LedgerEntry, error) {
	transfer, err := l.transfers.GetByID(ctx, toPgUUID(transferID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("transfer not found")
		}
		return nil, err
	}
	if !authModels.IsAdmin(principal) {
		source, sourceErr := l.accounts.GetAccountForTransferByID(
			ctx,
			uuidFromPgUUID(transfer.SourceAccountID),
		)
		destination, destinationErr := l.accounts.GetAccountForTransferByID(
			ctx,
			uuidFromPgUUID(transfer.DestinationAccountID),
		)
		if sourceErr != nil || destinationErr != nil ||
			(!accountOwnedBy(principal, source.CustomerID) &&
				!accountOwnedBy(principal, destination.CustomerID)) {
			return nil, fmt.Errorf("transfer not found")
		}
	}
	return l.ledger.GetByTransferID(ctx, toPgUUID(transferID))
}

func (l *ledgerService) GetEntriesByAccount(
	ctx context.Context,
	principal authModels.Principal,
	accountID uuid.UUID,
	offset int32,
	limit int32,
) ([]db.LedgerEntry, error) {
	if err := l.ensureAccountAccess(ctx, principal, accountID); err != nil {
		return nil, err
	}
	return l.ledger.GetByAccountID(ctx, toPgUUID(accountID), offset, limit)
}

func (l *ledgerService) ensureAccountAccess(
	ctx context.Context,
	principal authModels.Principal,
	accountID uuid.UUID,
) error {
	if authModels.IsAdmin(principal) {
		return nil
	}

	account, err := l.accounts.GetAccountForTransferByID(ctx, accountID)
	if err != nil || !accountOwnedBy(principal, account.CustomerID) {
		return fmt.Errorf("account not found")
	}
	return nil
}

func accountOwnedBy(principal authModels.Principal, customerID pgtype.UUID) bool {
	return authModels.IsAdmin(principal) ||
		uuid.UUID(customerID.Bytes) == principal.CustomerID
}
