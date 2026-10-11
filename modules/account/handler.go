package account

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Roy6lty/minibank/internal/httpx"
	appModels "github.com/Roy6lty/minibank/models"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	"github.com/go-chi/chi/v5"
)

type accountService interface {
	CreateAccount(context.Context, authModels.Principal, CreateAccountRequest) (GetAccountResponse, error)
	GetAccountByAccountNumber(context.Context, authModels.Principal, GetAccountRequest) (GetAccountResponse, error)
	SearchAccounts(context.Context, authModels.Principal, SearchRequest) ([]GetAccountResponse, error)
	ArchiveAccount(context.Context, authModels.Principal, UpdateAccountByAccountNumberRequest) error
}

type AccountHandler struct {
	service accountService
}

func NewAccountHandler(service accountService) *AccountHandler {
	return &AccountHandler{service: service}
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var request CreateAccountRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	principal, ok := accountPrincipal(w, r)
	if !ok {
		return
	}

	account, err := h.service.CreateAccount(r.Context(), principal, request)
	if err != nil {
		writeAccountError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, appModels.APIResponse{
		Message: "account created",
		Success: true,
		Data:    account,
	})
}

func (h *AccountHandler) GetAccountByAccountNumber(w http.ResponseWriter, r *http.Request) {
	accountNumber := chi.URLParam(r, "accountNumber")
	principal, ok := accountPrincipal(w, r)
	if !ok {
		return
	}
	account, err := h.service.GetAccountByAccountNumber(r.Context(), principal, GetAccountRequest{
		AccountNumber: &accountNumber,
	})
	if err != nil {
		writeAccountError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "account retrieved",
		Success: true,
		Data:    account,
	})
}

func (h *AccountHandler) SearchAccounts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	accountNumber := query.Get("account_number")
	accountType := query.Get("account_type")
	request := SearchRequest{}

	if accountNumber != "" {
		request.AccountNumber = &accountNumber
	}
	if accountType != "" {
		request.AccountType = &accountType
	}
	principal, ok := accountPrincipal(w, r)
	if !ok {
		return
	}

	accounts, err := h.service.SearchAccounts(r.Context(), principal, request)
	if err != nil {
		writeAccountError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "accounts retrieved",
		Success: true,
		Data:    accounts,
	})
}

func (h *AccountHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	var request UpdateAccountByAccountNumberRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	principal, ok := accountPrincipal(w, r)
	if !ok {
		return
	}

	request.AccountNumber = chi.URLParam(r, "accountNumber")
	if err := h.service.ArchiveAccount(r.Context(), principal, request); err != nil {
		writeAccountError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "account updated",
		Success: true,
	})
}

func writeAccountError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, authModels.ErrForbidden) {
		status = http.StatusForbidden
	} else if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "invalid") ||
		strings.Contains(err.Error(), "must") {
		status = http.StatusBadRequest
	} else if strings.Contains(err.Error(), "conflict") {
		status = http.StatusConflict
	}

	slog.Error("account request failed", "error", err)
	http.Error(w, http.StatusText(status), status)
}

func accountPrincipal(w http.ResponseWriter, r *http.Request) (authModels.Principal, bool) {
	principal, ok := authModels.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}
	return principal, ok
}
