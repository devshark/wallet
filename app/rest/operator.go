package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/devshark/wallet/api"
)

func (h *RestHandlers) HandleDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.DepositRequest{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)
		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrMissingIdempotencyKey)
		return
	}

	if request.ToAccountId == "" || request.Currency == "" || request.Amount.IsZero() || request.Amount.IsNegative() {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)
		return
	}

	if strings.EqualFold(request.ToAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrCompanyAccount)
		return
	}

	payload := &api.TransferRequest{
		FromAccountId: api.COMPANY_ACCOUNT_ID,
		ToAccountId:   strings.TrimSpace(request.ToAccountId),
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
			json.NewEncoder(w).Encode(*t)
			return
		}
	}

	// if the account id was not found in the transaction receipt
	h.HandleError(w, http.StatusUnprocessableEntity, api.ErrIncompleteTransaction)
}

func (h *RestHandlers) HandleWithdrawal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.WithdrawRequest{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)
		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrMissingIdempotencyKey)
		return
	}

	if request.FromAccountId == "" || request.Currency == "" || request.Amount.IsZero() || request.Amount.IsNegative() {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)
		return
	}

	if strings.EqualFold(request.FromAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrCompanyAccount)
		return
	}

	payload := &api.TransferRequest{
		FromAccountId: strings.TrimSpace(request.FromAccountId),
		ToAccountId:   api.COMPANY_ACCOUNT_ID,
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
			json.NewEncoder(w).Encode(t)
			return
		}
	}

	// if the account id was not found in the transaction receipt
	h.HandleError(w, http.StatusUnprocessableEntity, api.ErrIncompleteTransaction)
}

// Allows transfers from user to another user
func (h *RestHandlers) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.TransferRequest{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)
		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrMissingIdempotencyKey)
		return
	}

	if request.FromAccountId == "" || request.ToAccountId == "" || request.Currency == "" || request.Amount.IsZero() || request.Amount.IsNegative() {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidRequest)
		return
	}

	if strings.EqualFold(request.FromAccountId, request.ToAccountId) {
		h.HandleError(w, http.StatusBadRequest, api.ErrSameAccountIds)
		return
	}

	if strings.EqualFold(request.FromAccountId, api.COMPANY_ACCOUNT_ID) || strings.EqualFold(request.ToAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, api.ErrCompanyAccount)
		return
	}

	// makes sure we compose and pass only the sanitized payload
	payload := &api.TransferRequest{
		FromAccountId: strings.TrimSpace(request.FromAccountId),
		ToAccountId:   strings.TrimSpace(request.ToAccountId),
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
	json.NewEncoder(w).Encode(tx)
}
