package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devshark/wallet/api"
)

// HandleError writes an error response with the given code and error message to the given response writer.
// It is used to handle errors that occur during request processing.
func (h *RestHandlers) HandleError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(api.ErrorResponse{
		ErrorCode: code,
		Message:   err.Error(),
	})
}

// returns true if response is handled, false otherwise ie no errors
func (h *RestHandlers) HandleTransferError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, api.ErrInsufficientBalance):
		fallthrough
	case errors.Is(err, api.ErrDuplicateTransaction):
		h.HandleError(w, http.StatusUnprocessableEntity, err)
		return true
	case errors.Is(err, api.ErrSameAccountIds):
		fallthrough
	case errors.Is(err, api.ErrInvalidAmount):
		fallthrough
	case errors.Is(err, api.ErrInvalidAccountId):
		fallthrough
	case errors.Is(err, api.ErrInvalidCurrency):
		fallthrough
	case errors.Is(err, api.ErrInvalidRequest):
		h.HandleError(w, http.StatusBadRequest, err)
		return true
	case errors.Is(err, nil):
		return false
	default:
		h.logger.Printf("failed to transfer: %v\n", err)
		h.HandleError(w, http.StatusInternalServerError, api.ErrTransferFailed)
		return true
	}

}
