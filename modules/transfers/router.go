package transfers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(
	transferHandler *TransferHandler,
	lowRisk func(http.Handler) http.Handler,
	highRisk func(http.Handler) http.Handler,
) chi.Router {
	r := chi.NewRouter()

	r.With(highRisk).Post("/", transferHandler.CreateTransfer)
	r.With(lowRisk).Get("/", transferHandler.GetTransfers)
	r.With(lowRisk).Get("/reference/{referenceID}", transferHandler.GetTransferByReferenceID)
	r.With(lowRisk).Get("/{transferID}", transferHandler.GetTransfer)
	r.With(highRisk).Patch("/{transferID}", transferHandler.UpdateTransfer)

	return r
}
