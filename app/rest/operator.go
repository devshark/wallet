package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/devshark/wallet/api"
)

const (
	IdempotencyKeyHeader = "X-Idempotency-Key"
)

func (h *Handlers) HandleDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.DepositRequest{}

	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)

		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get(IdempotencyKeyHeader)
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrMissingIdempotencyKey)

		return
	}

	if request.ToAccountID == "" || request.Currency == "" || request.Amount.IsZero() || request.Amount.IsNegative() {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)

		return
	}

	if strings.EqualFold(request.ToAccountID, api.CompanyAccountID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrCompanyAccount)

		return
	}

	payload := &api.TransferRequest{
		FromAccountID: api.CompanyAccountID,
		ToAccountID:   strings.TrimSpace(request.ToAccountID),
		Currency:      strings.TrimSpace(request.Currency),
		Amount:        request.Amount,
		Remarks:       strings.TrimSpace(request.Remarks),
	}

	// create double entry transaction, returns both transaction result
	tx, err := h.repo.Transfer(ctx, payload, idempotencyKey)

	handled := h.HandleTransferError(w, err)
	if handled {
		return
	}

	// return the transaction receipt containing the relevant transfer details
	for _, t := range tx {
		if strings.EqualFold(string(t.Type), string(api.CREDIT)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)

			err = json.NewEncoder(w).Encode(t)
			if err != nil {
				// we can't respond with an error payload anymore, because the headers have already been sent
				// headers must be written before the content, so if writing the content fails, we can't go back
				// just log it
				h.logger.Printf("encoding error: %v", err)
			}

			return
		}
	}

	// if the account id was not found in the transaction receipt
	h.HandleError(w, http.StatusUnprocessableEntity, api.ErrIncompleteTransaction)
}

func (h *Handlers) HandleWithdrawal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.WithdrawRequest{}

	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)

		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get(IdempotencyKeyHeader)
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrMissingIdempotencyKey)

		return
	}

	if request.FromAccountID == "" || request.Currency == "" || request.Amount.IsZero() || request.Amount.IsNegative() {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)

		return
	}

	if strings.EqualFold(request.FromAccountID, api.CompanyAccountID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrCompanyAccount)

		return
	}

	payload := &api.TransferRequest{
		FromAccountID: strings.TrimSpace(request.FromAccountID),
		ToAccountID:   api.CompanyAccountID,
		Currency:      strings.TrimSpace(request.Currency),
		Amount:        request.Amount,
		Remarks:       strings.TrimSpace(request.Remarks),
	}

	// create double entry transaction, returns both transaction result
	tx, err := h.repo.Transfer(ctx, payload, idempotencyKey)

	handled := h.HandleTransferError(w, err)
	if handled {
		return
	}

	// return the transaction receipt containing the relevant transfer details
	for _, t := range tx {
		if strings.EqualFold(string(t.Type), string(api.DEBIT)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)

			err = json.NewEncoder(w).Encode(t)
			if err != nil {
				// we can't respond with an error payload anymore, because the headers have already been sent
				// headers must be written before the content, so if writing the content fails, we can't go back
				// just log it
				h.logger.Printf("encoding error: %v", err)
			}

			return
		}
	}

	// if the account id was not found in the transaction receipt
	h.HandleError(w, http.StatusUnprocessableEntity, api.ErrIncompleteTransaction)
}

// Allows transfers from user to another user
func (h *Handlers) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.TransferRequest{}

	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)

		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get(IdempotencyKeyHeader)
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrMissingIdempotencyKey)

		return
	}

	if request.FromAccountID == "" || request.ToAccountID == "" || request.Currency == "" || request.Amount.IsZero() || request.Amount.IsNegative() {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)

		return
	}

	if strings.EqualFold(request.FromAccountID, request.ToAccountID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrSameAccountIDs)

		return
	}

	if strings.EqualFold(request.FromAccountID, api.CompanyAccountID) || strings.EqualFold(request.ToAccountID, api.CompanyAccountID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrCompanyAccount)

		return
	}

	// makes sure we compose and pass only the sanitized payload
	payload := &api.TransferRequest{
		FromAccountID: strings.TrimSpace(request.FromAccountID),
		ToAccountID:   strings.TrimSpace(request.ToAccountID),
		Currency:      strings.TrimSpace(request.Currency),
		Amount:        request.Amount,
		Remarks:       strings.TrimSpace(request.Remarks),
	}

	// create double entry transaction, returns both transaction result
	tx, err := h.repo.Transfer(ctx, payload, idempotencyKey)

	handled := h.HandleTransferError(w, err)
	if handled {
		return
	}

	// return the transaction receipt containing the relevant transfer details
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(w).Encode(tx)
	if err != nil {
		// we can't respond with an error payload anymore, because the headers have already been sent
		// headers must be written before the content, so if writing the content fails, we can't go back
		// just log it
		h.logger.Printf("encoding error: %v", err)
	}
}
