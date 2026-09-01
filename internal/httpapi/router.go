package httpapi

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the Ledger HTTP router.
func NewRouter(handler *Handler) chi.Router {
	router := chi.NewRouter()

	// RequestID is what ties every log record back to a single client call.
	router.Use(middleware.RequestID)

	router.Post("/accounts", handler.createAccount)
	router.Get("/accounts/{id}", handler.getAccount)
	router.Post("/accounts/{id}/deposit", handler.deposit)
	router.Post("/transfers", handler.transfer)

	return router
}
