package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/devshark/wallet/api"
)

func (h *RestHandlers) HandleDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.DepositRequest{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, "failed to decode deposit request")
		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, "missing idempotency key")
		return
	}

	if request.ToAccountId == "" || request.Currency == "" || request.Amount.IsZero() {
		h.HandleError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if strings.EqualFold(request.ToAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, "cannot deposit to company account")
		return
	}

	payload := &api.TransferRequest{
		FromAccountId: api.COMPANY_ACCOUNT_ID,
		ToAccountId:   strings.TrimSpace(request.ToAccountId),
		Currency:      strings.TrimSpace(request.Currency),
		Amount:        request.Amount,
		Remarks:       strings.TrimSpace(request.Remarks),
	}

	fmt.Printf("deposit request: %#v\n", payload)
	// create double entry transaction, returns both transaction result
	tx, err := h.repo.Transfer(ctx, payload, idempotencyKey)
	handled := h.HandleTransferError(w, err)
	if handled {
		return
	}

	fmt.Printf("deposit receipts: %#v\n", tx)

	// return the transaction receipt containing the relevant transfer details
	for _, t := range tx {
		if strings.EqualFold(string(t.Type), string(api.CREDIT)) {
			h.logger.Printf("deposit receipt: %#v\n", *t)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(*t)
			return
		}
	}

	// if the account id was not found in the transaction receipt
	h.HandleError(w, http.StatusUnprocessableEntity, "your deposit was not processed successfully")
}

func (h *RestHandlers) HandleWithdrawal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.WithdrawRequest{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		h.HandleError(w, http.StatusBadRequest, "failed to decode withdrawal request")
		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, "missing idempotency key")
		return
	}

	if request.FromAccountId == "" || request.Currency == "" || request.Amount.IsZero() {
		h.HandleError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if strings.EqualFold(request.FromAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, "cannot withdraw from company account")
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
			h.logger.Printf("withdrawal receipt: %v\n", t)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(t)
			return
		}
	}

	// if the account id was not found in the transaction receipt
	h.HandleError(w, http.StatusUnprocessableEntity, "your withdrawal was not processed successfully")
}

// Allows transfers from user to another user
func (h *RestHandlers) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	request := &api.TransferRequest{}
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// idempotency key is required
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.HandleError(w, http.StatusBadRequest, "missing idempotency key")
		return
	}

	if request.FromAccountId == "" || request.ToAccountId == "" || request.Currency == "" || request.Amount.IsZero() {
		h.HandleError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if strings.EqualFold(request.FromAccountId, request.ToAccountId) {
		h.HandleError(w, http.StatusBadRequest, "cannot transfer to the same account")
		return
	}

	if strings.EqualFold(request.FromAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, "cannot transfer from company account")
		return
	}

	if strings.EqualFold(request.ToAccountId, api.COMPANY_ACCOUNT_ID) {
		h.HandleError(w, http.StatusBadRequest, "cannot transfer to company account")
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
