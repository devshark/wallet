package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devshark/wallet/api"
)

func (h *RestHandlers) HandleError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(api.ErrorResponse{
		ErrorCode: code,
		Message:   message,
	})
}

// returns true if response is handled, false otherwise ie no errors
func (h *RestHandlers) HandleTransferError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, api.ErrInsufficientBalance):
		h.HandleError(w, http.StatusUnprocessableEntity, "insufficient balance")
		return true
	case errors.Is(err, api.ErrSameAccountIds):
		h.HandleError(w, http.StatusBadRequest, "cannot transfer to the same account")
		return true
	case errors.Is(err, api.ErrInvalidAccountId):
		h.HandleError(w, http.StatusBadRequest, "invalid account id")
		return true
	case errors.Is(err, api.ErrInvalidCurrency):
		h.HandleError(w, http.StatusBadRequest, "invalid currency")
		return true
	case errors.Is(err, api.ErrInvalidAmount):
		h.HandleError(w, http.StatusBadRequest, "invalid amount")
		return true
	case errors.Is(err, api.ErrDuplicateTransaction):
		h.HandleError(w, http.StatusUnprocessableEntity, "duplicate transaction")
		return true
	case errors.Is(err, api.ErrInvalidRequest):
		h.HandleError(w, http.StatusBadRequest, "invalid request")
		return true
	case errors.Is(err, nil):
		return false
	default:
		h.logger.Printf("failed to transfer: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, "failed to transfer")
		return true
	}

}
