package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrForbidden           = errors.New("forbidden")
)

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return "too many requests"
}

type RateLimitUnavailableError struct {
	Err error
}

func (e *RateLimitUnavailableError) Error() string {
	return "rate limit unavailable"
}

func (e *RateLimitUnavailableError) Unwrap() error {
	return e.Err
}

type RegisterRequest struct {
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"required,max=100"`
	Email     string `json:"email" validate:"required,email,max=254"`
	Phone     string `json:"phone" validate:"omitempty,max=32"`
	Password  string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=512"`
}

type Permission string

const (
	PermissionCustomersAccess    Permission = "customers:access"
	PermissionAccountsAccess     Permission = "accounts:access"
	PermissionTransfersAccess    Permission = "transfers:access"
	PermissionLedgerAccess       Permission = "ledger:access"
	PermissionTransactionsAccess Permission = "transactions:access"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type Principal struct {
	UserID      uuid.UUID
	CustomerID  uuid.UUID
	Role        string
	Permissions []Permission
}

func IsAdmin(principal Principal) bool {
	return strings.EqualFold(principal.Role, "admin")
}
