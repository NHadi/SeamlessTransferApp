package http

import (
	"encoding/json"
	"errors"
	"internal-transfers/transaction-service/internal/application"
	"internal-transfers/transaction-service/internal/domain"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// TransactionHandler handles HTTP requests for transactions
type TransactionHandler struct {
	transactionService application.TransactionService
	validator          *validator.Validate
}

// NewTransactionHandler creates a new instance of TransactionHandler
func NewTransactionHandler(transactionService application.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
		validator:          validator.New(),
	}
}

// RegisterHandlers registers all transaction-related routes
func RegisterHandlers(r chi.Router, h *TransactionHandler) {
	r.Post("/transactions", h.SubmitTransaction)
	r.Get("/transactions/{id}", h.GetTransaction)
}

// SubmitTransactionRequest represents the request body for submitting a transaction
type SubmitTransactionRequest struct {
	SourceAccountID      int64  `json:"source_account_id" validate:"required"`
	DestinationAccountID int64  `json:"destination_account_id" validate:"required"`
	Amount               string `json:"amount" validate:"required"`
}

// TransactionResponse represents the response for transaction queries
type TransactionResponse struct {
	ID                   int64  `json:"id"`
	SourceAccountID      int64  `json:"source_account_id"`
	DestinationAccountID int64  `json:"destination_account_id"`
	Amount               string `json:"amount"`
	Status               string `json:"status"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SubmitTransaction handles the submission of a new transaction
// @Summary Submit a new transaction
// @Description Submit a new transaction between accounts
// @Tags transactions
// @Accept json
// @Produce json
// @Param transaction body SubmitTransactionRequest true "Transaction details"
// @Success 201 "Created"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transactions [post]
func (h *TransactionHandler) SubmitTransaction(w http.ResponseWriter, r *http.Request) {
	var req SubmitTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	dto := application.TransactionDTO{
		SourceAccountID:      domain.AccountID(req.SourceAccountID),
		DestinationAccountID: domain.AccountID(req.DestinationAccountID),
		Amount:               req.Amount,
	}

	if err := h.transactionService.SubmitTransaction(r.Context(), dto); err != nil {
		switch {
		case errors.Is(err, application.ErrSameAccount):
			respondWithError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, application.ErrInvalidAmount):
			respondWithError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, application.ErrInsufficientFunds):
			respondWithError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, application.ErrAccountNotFound):
			respondWithError(w, http.StatusNotFound, err.Error())
		default:
			respondWithError(w, http.StatusInternalServerError, "Failed to process transaction")
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// GetTransaction handles the retrieval of a transaction by ID
// @Summary Get transaction details
// @Description Get details of a specific transaction
// @Tags transactions
// @Accept json
// @Produce json
// @Param id path int true "Transaction ID"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /transactions/{id} [get]
func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid transaction ID")
		return
	}

	transaction, err := h.transactionService.GetTransaction(r.Context(), domain.TransactionID(id))
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	response := TransactionResponse{
		ID:                   int64(transaction.ID),
		SourceAccountID:      int64(transaction.SourceAccountID),
		DestinationAccountID: int64(transaction.DestinationAccountID),
		Amount:               transaction.Amount,
		Status:               string(transaction.Status),
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
