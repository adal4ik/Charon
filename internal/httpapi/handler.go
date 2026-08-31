package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/adal4ik/Charon/internal/domain"
	"github.com/adal4ik/Charon/internal/service"
	"github.com/go-chi/chi/v5"
)

type accountResponse struct {
	ID      int64 `json:"id"`
	Balance int64 `json:"balance"`
}

type depositRequest struct {
	Amount int64 `json:"amount"`
}

type transferRequest struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	Amount        int64 `json:"amount"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Handler handles Ledger HTTP requests.
type Handler struct {
	service *service.Service
}

func NewHandler(accountService *service.Service) *Handler {
	return &Handler{service: accountService}
}

func (h *Handler) createAccount(w http.ResponseWriter, r *http.Request) {
	account, err := h.service.CreateAccount(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusCreated, newAccountResponse(account)); err != nil {
		return
	}
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := parseAccountID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	account, err := h.service.GetAccount(r.Context(), accountID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, newAccountResponse(account)); err != nil {
		return
	}
}

func (h *Handler) deposit(w http.ResponseWriter, r *http.Request) {
	accountID, err := parseAccountID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	var request depositRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	account, err := h.service.Deposit(r.Context(), accountID, request.Amount)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, newAccountResponse(account)); err != nil {
		return
	}
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	var request transferRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if request.FromAccountID <= 0 || request.ToAccountID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	err := h.service.Transfer(
		r.Context(),
		request.FromAccountID,
		request.ToAccountID,
		request.Amount,
	)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseAccountID(value string) (int64, error) {
	accountID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || accountID <= 0 {
		return 0, errors.New("invalid account id")
	}

	return accountID, nil
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)

	var requestBody json.RawMessage
	if err := decoder.Decode(&requestBody); err != nil {
		return err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain one JSON object")
		}
		return err
	}

	requestBody = bytes.TrimSpace(requestBody)
	if len(requestBody) == 0 || requestBody[0] != '{' {
		return errors.New("request body must be a JSON object")
	}

	requestDecoder := json.NewDecoder(bytes.NewReader(requestBody))
	requestDecoder.DisallowUnknownFields()

	return requestDecoder.Decode(destination)
}

func newAccountResponse(account domain.Account) accountResponse {
	return accountResponse{
		ID:      account.ID,
		Balance: account.Balance,
	}
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidAmount):
		writeError(w, http.StatusBadRequest, domain.ErrInvalidAmount.Error())
	case errors.Is(err, domain.ErrSameAccount):
		writeError(w, http.StatusBadRequest, domain.ErrSameAccount.Error())
	case errors.Is(err, domain.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, domain.ErrAccountNotFound.Error())
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusConflict, domain.ErrInsufficientFunds.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	if err := writeJSON(w, status, errorResponse{Error: message}); err != nil {
		return
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(value)
}
