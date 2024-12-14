package rest

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/devshark/wallet/app/internal/repository"
	"github.com/devshark/wallet/pkg/middlewares"
	"github.com/go-redis/redis/v8"
)

const (
	maxHeaderBytes = 1 << 20

	// let's allocate 10 capacity just for demo's sake.
	middlewaresInitialCapacity = 10
)

type APIServer struct {
	repo        repository.Repository
	middlewares []middlewares.Middleware
	logger      *log.Logger
	pingers     []Pinger
}

func NewAPIServer(repo repository.Repository) *APIServer {
	return &APIServer{
		repo:        repo,
		pingers:     []Pinger{},
		logger:      log.Default(),
		middlewares: make([]middlewares.Middleware, 0, middlewaresInitialCapacity),
	}
}

func (r *APIServer) WithCacheMiddleware(redisClient *redis.Client, redisExpiration time.Duration) *APIServer {
	// only caches GET requests
	cacheMiddleware := middlewares.NewRedisCacheMiddleware(redisClient, redisExpiration)
	r.middlewares = append(r.middlewares, cacheMiddleware)

	return r
}

func (r *APIServer) WithCustomLogger(logger *log.Logger) *APIServer {
	r.logger = logger

	return r
}

func (r *APIServer) HTTPServer(port int64, httpReadTimeout, httpWriteTimeout time.Duration) *http.Server {
	mux := http.NewServeMux()

	handler := &Handlers{
		repo:    r.repo,
		logger:  r.logger,
		pingers: r.pingers,
	}

	middlewareChain := middlewares.MiddlewareChain(r.middlewares...)

	// pointless to cache health check
	mux.HandleFunc("GET /health", handler.HandleHealthCheck)
	// don't cache account balance, as it may change frequently
	mux.HandleFunc("GET /account/{accountId}/{currency}", (handler.GetAccountBalance))
	// only cache transactions, as they are fixed
	mux.HandleFunc("GET /transactions/{accountId}/{currency}", (handler.GetTransactions))
	mux.HandleFunc("GET /transactions/{txId}", middlewareChain(handler.GetTransaction))

	// don't cache mutable endpoints
	mux.HandleFunc("POST /deposit", (handler.HandleDeposit))
	mux.HandleFunc("POST /withdraw", (handler.HandleWithdrawal))
	mux.HandleFunc("POST /transfer", (handler.HandleTransfer))

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		ReadHeaderTimeout: 0,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}
