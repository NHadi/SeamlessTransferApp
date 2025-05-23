package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"internal-transfers/account-service/internal/application"
	"internal-transfers/account-service/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// AccountHandler handles HTTP requests for accounts
type AccountHandler struct {
	accountService application.AccountService
	validator      *validator.Validate
}

// CreateAccountRequest represents the request body for creating an account
type CreateAccountRequest struct {
	AccountID      int64  `json:"account_id" validate:"required,gt=0"`
	InitialBalance string `json:"initial_balance" validate:"required"`
}

// AccountResponse represents the response for account queries
type AccountResponse struct {
	AccountID int64  `json:"account_id"`
	Balance   string `json:"balance"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewAccountHandler creates a new instance of AccountHandler
func NewAccountHandler(accountService application.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
		validator:      validator.New(),
	}
}

// RegisterHandlers registers all account-related routes
func RegisterHandlers(r chi.Router, h *AccountHandler) {
	r.Post("/accounts", h.CreateAccount)
	r.Get("/accounts/{account_id}", h.GetAccount)
}

// @Summary Create a new account
// @Description Create a new account with initial balance
// @Tags accounts
// @Accept json
// @Produce json
// @Param account body CreateAccountRequest true "Account creation request"
// @Success 201 "Created"
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /accounts [post]
func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	dto := application.CreateAccountDTO{
		AccountID:      domain.AccountID(req.AccountID),
		InitialBalance: req.InitialBalance,
	}

	if err := h.accountService.CreateAccount(r.Context(), dto); err != nil {
		switch {
		case errors.Is(err, application.ErrAccountExists):
			respondWithError(w, http.StatusConflict, err.Error())
		case errors.Is(err, application.ErrInvalidAmount),
			errors.Is(err, application.ErrNegativeAmount),
			errors.Is(err, application.ErrInvalidAccountID):
			respondWithError(w, http.StatusBadRequest, err.Error())
		default:
			respondWithError(w, http.StatusInternalServerError, "Failed to create account")
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// @Summary Get account details
// @Description Get account details by ID
// @Tags accounts
// @Accept json
// @Produce json
// @Param account_id path int true "Account ID"
// @Success 200 {object} AccountResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /accounts/{account_id} [get]
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "account_id"), 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	account, err := h.accountService.GetAccount(r.Context(), domain.AccountID(accountID))
	if err != nil {
		switch {
		case errors.Is(err, application.ErrAccountNotFound):
			respondWithError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, application.ErrInvalidAccountID):
			respondWithError(w, http.StatusBadRequest, err.Error())
		default:
			respondWithError(w, http.StatusInternalServerError, "Failed to get account")
		}
		return
	}

	response := AccountResponse{
		AccountID: int64(account.ID),
		Balance:   account.Balance,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// respondWithError sends an error response with the given status code and message
func respondWithError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
