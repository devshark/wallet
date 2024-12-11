package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devshark/wallet/api"
)

func (h *RestHandlers) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	currency := r.PathValue("currency")
	accountId := r.PathValue("accountId")

	if currency == "" {
		h.HandleError(w, http.StatusBadRequest, "invalid currency")
		return
	}

	if accountId == "" {
		h.HandleError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	account, err := h.repo.GetAccountBalance(ctx, currency, accountId)
	if err != nil {
		h.logger.Printf("failed to get account balance: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, "failed to get account balance")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(account)
}

func (h *RestHandlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currency := r.PathValue("currency")
	accountId := r.PathValue("accountId")

	if currency == "" {
		h.HandleError(w, http.StatusBadRequest, "invalid currency")
		return
	}

	if accountId == "" {
		h.HandleError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	transactions, err := h.repo.GetTransactions(ctx, currency, accountId)
	if errors.Is(err, api.ErrTransactionNotFound) || transactions == nil {
		h.HandleError(w, http.StatusNotFound, "transaction not found")
		return
	}

	if err != nil {
		h.logger.Printf("failed to get transactions: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, "failed to get transactions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(transactions)
}

func (h *RestHandlers) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txId := r.PathValue("txId")

	if txId == "" {
		h.HandleError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	tx, err := h.repo.GetTransaction(ctx, txId)
	if errors.Is(err, api.ErrTransactionNotFound) || tx == nil {
		h.HandleError(w, http.StatusNotFound, "transaction not found")
		return
	}

	if err != nil {
		h.logger.Printf("failed to get transaction: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, "failed to get transaction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tx)
}
