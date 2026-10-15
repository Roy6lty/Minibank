package ledger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Roy6lty/minibank/internal/httpx"
	"github.com/Roy6lty/minibank/internal/logging"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	appModels "github.com/Roy6lty/minibank/models"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ledgerService interface {
	GetBalance(context.Context, authModels.Principal, uuid.UUID) (int64, error)
	GetEntryByID(context.Context, authModels.Principal, uuid.UUID) (db.LedgerEntry, error)
	GetEntriesByTransfer(context.Context, authModels.Principal, uuid.UUID) ([]db.LedgerEntry, error)
	GetEntriesByAccount(context.Context, authModels.Principal, uuid.UUID, int32, int32) ([]db.LedgerEntry, error)
}

type LedgerHandler struct {
	service ledgerService
}

func NewLedgerHandler(service ledgerService) *LedgerHandler {
	return &LedgerHandler{service: service}
}

func (h *LedgerHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseLedgerUUID(w, r, "accountID")
	if !ok {
		return
	}
	principal, ok := ledgerPrincipal(w, r)
	if !ok {
		return
	}

	balance, err := h.service.GetBalance(r.Context(), principal, accountID)
	if err != nil {
		writeLedgerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "balance retrieved",
		Success: true,
		Data:    balance,
	})
}

func (h *LedgerHandler) GetEntryByID(w http.ResponseWriter, r *http.Request) {
	entryID, ok := parseLedgerUUID(w, r, "entryID")
	if !ok {
		return
	}
	principal, ok := ledgerPrincipal(w, r)
	if !ok {
		return
	}

	entry, err := h.service.GetEntryByID(r.Context(), principal, entryID)
	if err != nil {
		writeLedgerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "ledger entry retrieved",
		Success: true,
		Data:    entry,
	})
}

func (h *LedgerHandler) GetEntriesByTransfer(w http.ResponseWriter, r *http.Request) {
	transferID, ok := parseLedgerUUID(w, r, "transferID")
	if !ok {
		return
	}
	principal, ok := ledgerPrincipal(w, r)
	if !ok {
		return
	}

	entries, err := h.service.GetEntriesByTransfer(r.Context(), principal, transferID)
	if err != nil {
		writeLedgerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "ledger entries retrieved",
		Success: true,
		Data:    entries,
	})
}

func (h *LedgerHandler) GetEntriesByAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseLedgerUUID(w, r, "accountID")
	if !ok {
		return
	}
	principal, ok := ledgerPrincipal(w, r)
	if !ok {
		return
	}

	offset, limit, err := parseLedgerPagination(r)
	if err != nil {
		writeLedgerError(w, r, err)
		return
	}

	entries, err := h.service.GetEntriesByAccount(r.Context(), principal, accountID, offset, limit)
	if err != nil {
		writeLedgerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "ledger entries retrieved",
		Success: true,
		Data:    entries,
	})
}

func parseLedgerUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeLedgerError(w, r, fmt.Errorf("invalid %s: %w", name, err))
		return uuid.Nil, false
	}

	return id, true
}

func parseLedgerPagination(r *http.Request) (int32, int32, error) {
	query := r.URL.Query()
	offset := int32(0)
	limit := int32(20)

	if value := query.Get("offset"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid offset")
		}
		offset = int32(parsed)
	}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid limit")
		}
		limit = int32(parsed)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("offset cannot be negative")
	}
	if limit <= 0 {
		return 0, 0, fmt.Errorf("limit must be greater than zero")
	}

	return offset, limit, nil
}

func writeLedgerError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, authModels.ErrForbidden) {
		status = http.StatusForbidden
	} else if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "invalid") ||
		strings.Contains(err.Error(), "must") ||
		strings.Contains(err.Error(), "cannot") {
		status = http.StatusBadRequest
	}

	slog.ErrorContext(r.Context(), "ledger request failed", logging.Error(err))
	http.Error(w, http.StatusText(status), status)
}

func ledgerPrincipal(w http.ResponseWriter, r *http.Request) (authModels.Principal, bool) {
	principal, ok := authModels.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}
	return principal, ok
}
