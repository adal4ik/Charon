package httpapi

import "github.com/go-chi/chi/v5"

// NewRouter creates the Ledger HTTP router.
func NewRouter(handler *Handler) chi.Router {
	router := chi.NewRouter()

	router.Post("/accounts", handler.createAccount)
	router.Get("/accounts/{id}", handler.getAccount)
	router.Post("/accounts/{id}/deposit", handler.deposit)
	router.Post("/transfers", handler.transfer)

	return router
}
