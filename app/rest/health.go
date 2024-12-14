package rest

import (
	"context"
	"net/http"
)

type Pinger func(ctx context.Context) error

func (r *APIServer) AddPinger(p Pinger) *APIServer {
	r.pingers = append(r.pingers, p)

	return r
}

func (h *Handlers) AddPinger(p Pinger) *Handlers {
	h.pingers = append(h.pingers, p)

	return h
}

func (h *Handlers) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	for _, pinger := range h.pingers {
		if err := pinger(ctx); err != nil {
			h.HandleError(w, http.StatusInternalServerError, err)

			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
