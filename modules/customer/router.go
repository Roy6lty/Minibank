package customer

import "github.com/go-chi/chi/v5"

func NewRouter(customerHandler *CustomerHandler) chi.Router {
	r := chi.NewRouter()

	r.Post("/", customerHandler.CreateCustomer)
	r.Get("/search", customerHandler.GetCustomerByName)
	r.Get("/{customerID}", customerHandler.GetCustomerByID)
	r.Patch("/{customerID}", customerHandler.UpdateCustomer)

	return r
}
