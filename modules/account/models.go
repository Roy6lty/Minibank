package account

import "github.com/jackc/pgx/v5/pgtype"

type AccountStatus string

const (
	AccountStatusActive AccountStatus = "ACTIVE"
	AccountStatusFrozen AccountStatus = "FROZEN"
	AccountStatusClosed AccountStatus = "CLOSED"
)

type AccountType string

const (
	AccountTypeSavings AccountType = "SAVINGS"
	AccountTypeCurrent AccountType = "CURRENT"
)

type Currency string

const (
	CurrencyNGN Currency = "NGN"
	CurrencyUSD Currency = "USD"
	CurrencyGBP Currency = "GBP"
	CurrencyEUR Currency = "EUR"
)

type CreateAccountRequest struct {
	CustomerID  string      `json:"customer_id" validate:"omitempty,uuid"`
	AccountType AccountType `json:"account_type" validate:"required,oneof=SAVINGS CURRENT"`
	Currency    Currency    `json:"currency" validate:"required,oneof=NGN USD GBP EUR"`
}

type UpdateAccountRequest struct {
	AccountNumber string `json:"account_number"`
	Status        string `json:"status"`
}

type DeactivateAccountRequest struct {
	CustomerID    string `json:"customer_id"`
	AccountNumber string `json:"account_number"`
}

type GetAccountRequest struct {
	AccountNumber *string `json:"account_number"`
	AccountType   *string `json:"account_type"`
}
type GetAccountResponse struct {
	AccountNumber string `json:"account_number"`
	AccountType   string `json:"account_type"`
	Currency      string `json:"currency"`
	AccountStatus string `json:"account_status"`
}

type TransferAccount struct {
	ID         pgtype.UUID
	CustomerID pgtype.UUID
	Currency   string
	Status     string
	Balance    int64
}

type SearchRequest struct {
	AccountNumber *string `json:"account_number"`
	AccountType   *string `json:"account_type"`
}

type UpdateAccountByAccountNumberRequest struct {
	Status        AccountStatus `json:"status" validate:"required,oneof=ACTIVE FROZEN CLOSED"`
	AccountNumber string        `json:"account_number"`
}
