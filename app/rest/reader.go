package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devshark/wallet/api"
)

func (h *Handlers) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	currency := r.PathValue("currency")
	accountID := r.PathValue("accountId")

	if currency == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidCurrency)

		return
	}

	if accountID == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidAccountID)

		return
	}

	account, err := h.repo.GetAccountBalance(ctx, currency, accountID)
	if errors.Is(err, api.ErrAccountNotFound) {
		h.HandleError(w, http.StatusNotFound, api.ErrAccountNotFound)

		return
	}

	if err != nil {
		h.logger.Printf("failed to get account balance: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, api.ErrFailedToGetTransaction)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(account)
	if err != nil {
		// we can't respond with an error payload anymore, because the headers have already been sent
		// headers must be written before the content, so if writing the content fails, we can't go back
		// just log it
		h.logger.Printf("encoding error: %v", err)
	}
}

func (h *Handlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currency := r.PathValue("currency")
	accountID := r.PathValue("accountId")

	if currency == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidCurrency)

		return
	}

	if accountID == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidAccountID)

		return
	}

	transactions, err := h.repo.GetTransactions(ctx, currency, accountID)
	if errors.Is(err, api.ErrTransactionNotFound) {
		h.HandleError(w, http.StatusNotFound, api.ErrTransactionNotFound)

		return
	}

	if err != nil {
		h.logger.Printf("failed to get transactions: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, api.ErrFailedToGetTransaction)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if transactions == nil {
		_, _ = w.Write([]byte("[]"))

		return
	}

	err = json.NewEncoder(w).Encode(transactions)
	if err != nil {
		// we can't respond with an error payload anymore, because the headers have already been sent
		// headers must be written before the content, so if writing the content fails, we can't go back
		// just log it
		h.logger.Printf("encoding error: %v", err)
	}
}

func (h *Handlers) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txID := r.PathValue("txId")

	if txID == "" {
		h.HandleError(w, http.StatusBadRequest, api.ErrInvalidTxID)

		return
	}

	tx, err := h.repo.GetTransaction(ctx, txID)
	if errors.Is(err, api.ErrTransactionNotFound) {
		h.HandleError(w, http.StatusNotFound, api.ErrTransactionNotFound)

		return
	}

	if err != nil {
		h.logger.Printf("failed to get transaction: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, api.ErrFailedToGetTransaction)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(tx)
	if err != nil {
		// we can't respond with an error payload anymore, because the headers have already been sent
		// headers must be written before the content, so if writing the content fails, we can't go back
		// just log it
		h.logger.Printf("encoding error: %v", err)
	}
}
