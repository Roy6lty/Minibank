package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/Roy6lty/minibank/database/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Roy6lty/minibank/internal/sqlc"
	models "github.com/Roy6lty/minibank/modules/account"
	authModels "github.com/Roy6lty/minibank/modules/auth"
)

type AccountService interface {
	GetAccountForTransfer(
		ctx context.Context,
		accountNumber string,
	) (models.TransferAccount, error)

	GetAccountForTransferByID(
		ctx context.Context,
		accountID uuid.UUID,
	) (models.TransferAccount, error)

	GetLockedAccountsForTransfer(
		ctx context.Context,
		sourceAccountID uuid.UUID,
		destinationAccountID uuid.UUID,
	) (models.TransferAccount, models.TransferAccount, error)

	AdjustBalance(
		ctx context.Context,
		accountID uuid.UUID,
		delta int64,
	) error

	CreateAccount(
		ctx context.Context,
		principal authModels.Principal,
		request models.CreateAccountRequest,
	) (models.GetAccountResponse, error)

	GetAccountByAccountNumber(
		ctx context.Context,
		principal authModels.Principal,
		request models.GetAccountRequest,
	) (models.GetAccountResponse, error)

	SearchAccounts(
		ctx context.Context,
		principal authModels.Principal,
		request models.SearchRequest,
	) ([]models.GetAccountResponse, error)

	ArchiveAccount(
		ctx context.Context,
		principal authModels.Principal,
		request models.UpdateAccountByAccountNumberRequest,
	) error
}

type accountService struct {
	accounts repository.AccountRepository
}

func NewAccountService(accounts repository.AccountRepository) AccountService {
	return &accountService{accounts: accounts}
}

func generateAccountNumber() (string, error) {
	number, err := rand.Int(
		rand.Reader,
		big.NewInt(10_000_000_000),
	)
	if err != nil {
		return "", fmt.Errorf(
			"generate account number: %w",
			err,
		)
	}

	return fmt.Sprintf("%010d", number.Int64()), nil
}

func isAccountNumberConflict(err error) bool {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}

func validateAccountNumber(accountNumber string) error {
	if len(accountNumber) != 10 {
		return fmt.Errorf("account number must be exactly 10 digits")
	}

	for _, char := range accountNumber {
		if char < '0' || char > '9' {
			return fmt.Errorf("account number must contain only numbers")
		}
	}

	return nil
}
func (a *accountService) CreateAccount(
	ctx context.Context,
	principal authModels.Principal,
	accountRequest models.CreateAccountRequest,
) (models.GetAccountResponse, error) {
	if !validAccountType(accountRequest.AccountType) {
		return models.GetAccountResponse{}, fmt.Errorf("invalid account type")
	}
	if !validCurrency(accountRequest.Currency) {
		return models.GetAccountResponse{}, fmt.Errorf("invalid currency")
	}

	if !authModels.IsAdmin(principal) {
		accountRequest.CustomerID = principal.CustomerID.String()
	}

	var accountResponse models.GetAccountResponse
	customerID, err := uuid.Parse(accountRequest.CustomerID)
	if err != nil {
		return accountResponse, fmt.Errorf(
			"invalid customer ID: %w",
			err,
		)
	}

	for i := 0; i < 5; i++ {

		accountNumber, err := generateAccountNumber()
		if err != nil {
			return accountResponse, err
		}

		request := db.CreateAccountParams{
			CustomerID: pgtype.UUID{
				Bytes: customerID,
				Valid: true,
			},
			AccountNumber: accountNumber,
			AccountType:   string(accountRequest.AccountType),
			Currency:      string(accountRequest.Currency),
		}

		account, err := a.accounts.Create(ctx, request)

		if err == nil {
			return models.GetAccountResponse{
				AccountNumber: account.AccountNumber,
				AccountType:   account.AccountType,
				Currency:      account.Currency,
				AccountStatus: account.Status,
			}, nil
		}

		if isAccountNumberConflict(err) {
			continue
		}

		return accountResponse, fmt.Errorf(
			"create account: %w",
			err,
		)
	}

	return accountResponse,
		fmt.Errorf("unable to generate unique account number")
}

func (a *accountService) GetAccountByAccountNumber(
	ctx context.Context,
	principal authModels.Principal,
	accountRequest models.GetAccountRequest,
) (models.GetAccountResponse, error) {

	var accountResponse models.GetAccountResponse
	err := validateAccountNumber(*accountRequest.AccountNumber)
	if err != nil {
		return accountResponse, err
	}

	account, err := a.accounts.GetByAccountNumber(ctx, *accountRequest.AccountNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return accountResponse, fmt.Errorf("customer not found")
		}

		return accountResponse, err
	}
	if !accountOwnedBy(principal, uuid.UUID(account.CustomerID.Bytes)) {
		return accountResponse, fmt.Errorf("account not found")
	}

	return models.GetAccountResponse{
		AccountNumber: account.AccountNumber,
		AccountType:   account.AccountType,
		Currency:      account.Currency,
		AccountStatus: account.Status,
	}, err

}

func (a *accountService) GetAccountForTransfer(
	ctx context.Context,
	accountNumber string,
) (models.TransferAccount, error) {
	if err := validateAccountNumber(accountNumber); err != nil {
		return models.TransferAccount{}, err
	}

	account, err := a.accounts.GetByAccountNumber(ctx, accountNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.TransferAccount{}, fmt.Errorf("account not found")
		}
		return models.TransferAccount{}, fmt.Errorf("get account: %w", err)
	}

	return a.transferAccount(account), nil
}

func (a *accountService) GetAccountForTransferByID(
	ctx context.Context,
	accountID uuid.UUID,
) (models.TransferAccount, error) {
	if accountID == uuid.Nil {
		return models.TransferAccount{}, fmt.Errorf("account ID is required")
	}

	account, err := a.accounts.GetByID(ctx, pgtype.UUID{
		Bytes: accountID,
		Valid: true,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.TransferAccount{}, fmt.Errorf("account not found")
		}
		return models.TransferAccount{}, fmt.Errorf("get account: %w", err)
	}

	return a.transferAccount(account), nil
}

func (a *accountService) GetLockedAccountsForTransfer(
	ctx context.Context,
	sourceAccountID uuid.UUID,
	destinationAccountID uuid.UUID,
) (models.TransferAccount, models.TransferAccount, error) {
	if sourceAccountID == uuid.Nil || destinationAccountID == uuid.Nil {
		return models.TransferAccount{}, models.TransferAccount{}, fmt.Errorf("account IDs are required")
	}
	if sourceAccountID == destinationAccountID {
		return models.TransferAccount{}, models.TransferAccount{}, fmt.Errorf("source and destination accounts must be different")
	}

	firstID, secondID := sourceAccountID, destinationAccountID
	if bytes.Compare(firstID[:], secondID[:]) > 0 {
		firstID, secondID = secondID, firstID
	}

	firstAccount, err := a.accounts.LockByID(ctx, pgtype.UUID{
		Bytes: firstID,
		Valid: true,
	})
	if err != nil {
		return models.TransferAccount{}, models.TransferAccount{}, fmt.Errorf("lock account: %w", err)
	}
	secondAccount, err := a.accounts.LockByID(ctx, pgtype.UUID{
		Bytes: secondID,
		Valid: true,
	})
	if err != nil {
		return models.TransferAccount{}, models.TransferAccount{}, fmt.Errorf("lock account: %w", err)
	}

	sourceAccount, destinationAccount := firstAccount, secondAccount
	if firstID != sourceAccountID {
		sourceAccount, destinationAccount = secondAccount, firstAccount
	}

	return a.transferAccount(sourceAccount), a.transferAccount(destinationAccount), nil
}

func (a *accountService) AdjustBalance(
	ctx context.Context,
	accountID uuid.UUID,
	delta int64,
) error {
	if accountID == uuid.Nil {
		return fmt.Errorf("account ID is required")
	}

	_, err := a.accounts.AdjustBalance(ctx, pgtype.UUID{
		Bytes: accountID,
		Valid: true,
	}, delta)
	return err
}

func (a *accountService) transferAccount(
	account db.Account,
) models.TransferAccount {
	return models.TransferAccount{
		ID:         account.ID,
		CustomerID: account.CustomerID,
		Currency:   account.Currency,
		Status:     account.Status,
		Balance:    account.Balance,
	}
}

func (a *accountService) SearchAccounts(
	ctx context.Context,
	principal authModels.Principal,
	request models.SearchRequest,
) ([]models.GetAccountResponse, error) {

	var accountNumber pgtype.Text
	if request.AccountNumber != nil && *request.AccountNumber != "" {
		if err := validateAccountNumber(*request.AccountNumber); err != nil {
			return nil, err
		}

		accountNumber = pgtype.Text{
			String: *request.AccountNumber,
			Valid:  true,
		}
	}

	var accountType pgtype.Text
	if request.AccountType != nil && *request.AccountType != "" {
		if !validAccountType(models.AccountType(*request.AccountType)) {
			return nil, fmt.Errorf("invalid account type")
		}
		accountType = pgtype.Text{
			String: *request.AccountType,
			Valid:  true,
		}
	}

	var customerID pgtype.UUID
	if !authModels.IsAdmin(principal) {
		customerID = pgtype.UUID{
			Bytes: principal.CustomerID,
			Valid: true,
		}
	}

	params := db.SearchAccountsParams{
		AccountNumber: accountNumber,
		AccountType:   accountType,
		CustomerID:    customerID,
		LimitCount:    20,
		OffsetCount:   0,
	}

	accounts, err := a.accounts.Search(ctx, params)
	if err != nil {
		return nil, err
	}

	responses := make([]models.GetAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		responses = append(responses, models.GetAccountResponse{
			AccountNumber: account.AccountNumber,
			AccountType:   account.AccountType,
			Currency:      account.Currency,
			AccountStatus: account.Status,
		})
	}

	return responses, nil
}

func (a *accountService) ArchiveAccount(
	ctx context.Context,
	principal authModels.Principal,
	args models.UpdateAccountByAccountNumberRequest,
) error {
	if !authModels.IsAdmin(principal) {
		return authModels.ErrForbidden
	}

	err := validateAccountNumber(args.AccountNumber)
	if err != nil {
		return fmt.Errorf("customer not found")
	}
	if !validAccountStatus(args.Status) {
		return fmt.Errorf("invalid account status")
	}

	account, err := a.accounts.GetByAccountNumber(ctx, args.AccountNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("account not found")
		}
		return fmt.Errorf("get account: %w", err)
	}
	if !canTransitionAccountStatus(models.AccountStatus(account.Status), args.Status) {
		return fmt.Errorf("invalid account status transition")
	}

	query := db.UpdateAccountByAccountNumberParams{
		NewStatus:     string(args.Status),
		AccountNumber: args.AccountNumber,
		CurrentStatus: account.Status,
	}

	_, err = a.accounts.UpdateByAccountNumber(ctx, query)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("account update conflict")
		}
		return fmt.Errorf("error updating account %s", err.Error())
	}

	return nil

}

func accountOwnedBy(principal authModels.Principal, customerID uuid.UUID) bool {
	return authModels.IsAdmin(principal) || customerID == principal.CustomerID
}

func validAccountStatus(status models.AccountStatus) bool {
	switch status {
	case models.AccountStatusActive, models.AccountStatusFrozen, models.AccountStatusClosed:
		return true
	default:
		return false
	}
}

func validAccountType(accountType models.AccountType) bool {
	switch accountType {
	case models.AccountTypeSavings, models.AccountTypeCurrent:
		return true
	default:
		return false
	}
}

func validCurrency(currency models.Currency) bool {
	switch currency {
	case models.CurrencyNGN, models.CurrencyUSD, models.CurrencyGBP, models.CurrencyEUR:
		return true
	default:
		return false
	}
}

func canTransitionAccountStatus(current, next models.AccountStatus) bool {
	switch current {
	case models.AccountStatusActive:
		return next == models.AccountStatusFrozen || next == models.AccountStatusClosed
	case models.AccountStatusFrozen:
		return next == models.AccountStatusActive || next == models.AccountStatusClosed
	default:
		return false
	}
}
