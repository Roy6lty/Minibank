package transactions

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Roy6lty/minibank/internal/httpx"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	appModels "github.com/Roy6lty/minibank/models"
	"github.com/go-chi/chi/v5"
)

type transactionService interface {
	CreateTransaction(context.Context, CreateTransactionRequest) error
	GetTransactionByID(context.Context, GetTransactionsRequest) (db.Transaction, error)
	UpdateTransaction(context.Context, string, UpdateTransactionRequest) error
}

type TransactionHandler struct {
	service transactionService
}

func NewTransactionHandler(
	service transactionService,
) *TransactionHandler {
	return &TransactionHandler{service: service}
}

func (h *TransactionHandler) CreateTransaction(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request CreateTransactionRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}

	if err := h.service.CreateTransaction(r.Context(), request); err != nil {
		writeTransactionError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, appModels.APIResponse{
		Message: "transaction created",
		Success: true,
	})
}

func (h *TransactionHandler) GetTransactionByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := chi.URLParam(r, "transactionID")
	transaction, err := h.service.GetTransactionByID(r.Context(), GetTransactionsRequest{
		ID: &id,
	})
	if err != nil {
		writeTransactionError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "transaction retrieved",
		Data:    transaction,
	})
}

func (h *TransactionHandler) UpdateTransaction(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request UpdateTransactionRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}

	id := chi.URLParam(r, "transactionID")
	if err := h.service.UpdateTransaction(r.Context(), id, request); err != nil {
		writeTransactionError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "transaction updated",
	})
}

func writeTransactionError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "invalid") {
		status = http.StatusBadRequest
	}

	slog.Error("transaction request failed", "error", err)
	http.Error(w, http.StatusText(status), status)
}
