package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/devshark/wallet/api"
)

// HandleError writes an error response with the given code and error message to the given response writer.
// It is used to handle errors that occur during request processing.
func (h *Handlers) HandleError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	errEncode := json.NewEncoder(w).Encode(api.ErrorResponse{
		ErrorCode: code,
		Message:   err.Error(),
	})
	if errEncode != nil {
		// we can't respond with an error payload anymore, because the headers have already been sent
		// headers must be written before the content, so if writing the content fails, we can't go back
		// just log it
		h.logger.Printf("encoding error: %v", err)
	}
}

// returns true if response is handled, false otherwise ie no errors
func (h *Handlers) HandleTransferError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, api.ErrInsufficientBalance):
		fallthrough
	case errors.Is(err, api.ErrDuplicateTransaction):
		h.HandleError(w, http.StatusUnprocessableEntity, err)

		return true
	case errors.Is(err, api.ErrSameAccountIDs):
		fallthrough
	case errors.Is(err, api.ErrInvalidAmount):
		fallthrough
	case errors.Is(err, api.ErrInvalidAccountID):
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
