package transfers

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
	appModels "github.com/Roy6lty/minibank/models"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	"github.com/go-chi/chi/v5"
)

type transferService interface {
	CreateTransfer(context.Context, authModels.Principal, CreateTransferRequest) error
	GetTransfer(context.Context, authModels.Principal, GetTransferDetailsRequest) (TransferRequestResponse, error)
	GetTransferByReferenceID(context.Context, authModels.Principal, GetTransferDetailsRequest) (TransferRequestResponse, error)
	GetTransfers(context.Context, authModels.Principal, string, int32, int32) ([]TransferRequestResponse, error)
	UpdateTransfers(context.Context, authModels.Principal, string, UpdateTransferRequest) error
}

type TransferHandler struct {
	service transferService
}

func NewTransferHandler(service transferService) *TransferHandler {
	return &TransferHandler{service: service}
}

func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var request CreateTransferRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	principal, ok := transferPrincipal(w, r)
	if !ok {
		return
	}

	if err := h.service.CreateTransfer(r.Context(), principal, request); err != nil {
		writeTransferError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, appModels.APIResponse{
		Message: "transfer created",
		Success: true,
	})
}

func (h *TransferHandler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "transferID")
	principal, ok := transferPrincipal(w, r)
	if !ok {
		return
	}
	transfer, err := h.service.GetTransfer(r.Context(), principal, GetTransferDetailsRequest{
		TransferID: &id,
	})
	if err != nil {
		writeTransferError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "transfer retrieved",
		Success: true,
		Data:    transfer,
	})
}

func (h *TransferHandler) GetTransferByReferenceID(w http.ResponseWriter, r *http.Request) {
	referenceID := chi.URLParam(r, "referenceID")
	principal, ok := transferPrincipal(w, r)
	if !ok {
		return
	}
	transfer, err := h.service.GetTransferByReferenceID(r.Context(), principal, GetTransferDetailsRequest{
		ReferenceID: &referenceID,
	})
	if err != nil {
		writeTransferError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "transfer retrieved",
		Success: true,
		Data:    transfer,
	})
}

func (h *TransferHandler) GetTransfers(w http.ResponseWriter, r *http.Request) {
	offset, limit, err := parsePagination(r)
	if err != nil {
		writeTransferError(w, r, err)
		return
	}
	principal, ok := transferPrincipal(w, r)
	if !ok {
		return
	}

	transfers, err := h.service.GetTransfers(
		r.Context(),
		principal,
		r.URL.Query().Get("account_number"),
		offset,
		limit,
	)
	if err != nil {
		writeTransferError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "transfers retrieved",
		Success: true,
		Data:    transfers,
	})
}

func (h *TransferHandler) UpdateTransfer(w http.ResponseWriter, r *http.Request) {
	var request UpdateTransferRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	principal, ok := transferPrincipal(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "transferID")
	if err := h.service.UpdateTransfers(r.Context(), principal, id, request); err != nil {
		writeTransferError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "transfer updated",
		Success: true,
	})
}

func parsePagination(r *http.Request) (int32, int32, error) {
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

func writeTransferError(w http.ResponseWriter, r *http.Request, err error) {
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
	} else if strings.Contains(err.Error(), "conflict") {
		status = http.StatusConflict
	}

	slog.ErrorContext(r.Context(), "transfer request failed", logging.Error(err))
	http.Error(w, http.StatusText(status), status)
}

func transferPrincipal(w http.ResponseWriter, r *http.Request) (authModels.Principal, bool) {
	principal, ok := authModels.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}
	return principal, ok
}
