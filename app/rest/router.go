package rest

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/devshark/wallet/app/internal/repository"
)

func NewRouter(repo repository.Repository, logger *log.Logger) http.Handler {
	mux := http.NewServeMux()

	handler := &RestHandlers{
		repo:   repo,
		logger: logger,
	}

	mux.HandleFunc("GET /health", handler.HandleHealthCheck)
	mux.HandleFunc("GET /account/{accountId}/{currency}", handler.GetAccountBalance)
	mux.HandleFunc("GET /transactions/{accountId}/{currency}", handler.GetTransactions)
	mux.HandleFunc("GET /transactions/{txId}", handler.GetTransaction)

	mux.HandleFunc("POST /deposit", handler.HandleDeposit)
	mux.HandleFunc("POST /withdraw", handler.HandleWithdrawal)
	mux.HandleFunc("POST /transfer", handler.HandleTransfer)

	return mux
}

func NewHttpServer(httpHandlers http.Handler, port int64, httpReadTimeout, httpWriteTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           httpHandlers,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		ReadHeaderTimeout: 0,
		MaxHeaderBytes:    1 << 20,
	}
}
