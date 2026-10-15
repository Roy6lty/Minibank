package customer

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Roy6lty/minibank/internal/httpx"
	"github.com/Roy6lty/minibank/internal/logging"
	db "github.com/Roy6lty/minibank/internal/sqlc"
	appModels "github.com/Roy6lty/minibank/models"
	authModels "github.com/Roy6lty/minibank/modules/auth"
	"github.com/go-chi/chi/v5"
)

type customerService interface {
	CreateCustomer(context.Context, authModels.Principal, CreateCustomerRequest) error
	GetCustomerByID(context.Context, authModels.Principal, GetCustomerRequest) (db.Customer, error)
	GetCustomerByName(context.Context, authModels.Principal, GetCustomerRequest) ([]db.Customer, error)
	UpdateCustomer(context.Context, authModels.Principal, string, UpdateCustomerRequest) error
}

type CustomerHandler struct {
	service customerService
}

func NewCustomerHandler(service customerService) *CustomerHandler {
	return &CustomerHandler{service: service}
}

func (h *CustomerHandler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var request CreateCustomerRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	principal, ok := customerPrincipal(w, r)
	if !ok {
		return
	}

	if err := h.service.CreateCustomer(r.Context(), principal, request); err != nil {
		writeCustomerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, appModels.APIResponse{
		Message: "customer created",
		Success: true,
	})
}

func (h *CustomerHandler) GetCustomerByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "customerID")
	principal, ok := customerPrincipal(w, r)
	if !ok {
		return
	}
	customer, err := h.service.GetCustomerByID(r.Context(), principal, GetCustomerRequest{
		ID: &id,
	})
	if err != nil {
		writeCustomerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "customer retrieved",
		Success: true,
		Data:    customer,
	})
}

func (h *CustomerHandler) GetCustomerByName(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	firstName := query.Get("first_name")
	lastName := query.Get("last_name")
	request := GetCustomerRequest{}
	principal, ok := customerPrincipal(w, r)
	if !ok {
		return
	}

	if firstName != "" {
		request.FirstName = &firstName
	}
	if lastName != "" {
		request.LastName = &lastName
	}

	customers, err := h.service.GetCustomerByName(r.Context(), principal, request)
	if err != nil {
		writeCustomerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "customers retrieved",
		Success: true,
		Data:    customers,
	})
}

func (h *CustomerHandler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	var request UpdateCustomerRequest
	if !httpx.DecodeJSON(w, r, &request) {
		return
	}
	principal, ok := customerPrincipal(w, r)
	if !ok {
		return
	}

	id := chi.URLParam(r, "customerID")
	if err := h.service.UpdateCustomer(r.Context(), principal, id, request); err != nil {
		writeCustomerError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, appModels.APIResponse{
		Message: "customer updated",
		Success: true,
	})
}

func writeCustomerError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, authModels.ErrForbidden) {
		status = http.StatusForbidden
	} else if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "required") ||
		strings.Contains(err.Error(), "invalid") {
		status = http.StatusBadRequest
	}

	slog.ErrorContext(r.Context(), "customer request failed", logging.Error(err))
	http.Error(w, http.StatusText(status), status)
}

func customerPrincipal(w http.ResponseWriter, r *http.Request) (authModels.Principal, bool) {
	principal, ok := authModels.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	}
	return principal, ok
}
