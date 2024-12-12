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

type RestApiServer struct {
	repo        repository.Repository
	middlewares []middlewares.Middleware
	logger      *log.Logger
}

func NewRestApiServer(repo repository.Repository) *RestApiServer {
	return &RestApiServer{
		repo:   repo,
		logger: log.Default(),
		// let's allocate 10 capacity just for demo's sake.
		middlewares: make([]middlewares.Middleware, 0, 10),
	}
}

func (r *RestApiServer) WithCacheMiddleware(redisClient *redis.Client, redisExpiration time.Duration) *RestApiServer {
	// only caches GET requests
	cacheMiddleware := middlewares.NewRedisCacheMiddleware(redisClient, time.Hour)
	r.middlewares = append(r.middlewares, cacheMiddleware)

	return r
}

func (r *RestApiServer) WithCustomLogger(logger *log.Logger) *RestApiServer {
	r.logger = logger
	return r
}

func (r *RestApiServer) HttpServer(port int64, httpReadTimeout, httpWriteTimeout time.Duration) *http.Server {
	mux := http.NewServeMux()

	handler := &RestHandlers{
		repo:   r.repo,
		logger: r.logger,
	}

	middlewareChain := middlewares.MiddlewareChain(r.middlewares...)

	// pointless to cache health check
	mux.HandleFunc("GET /health", handler.HandleHealthCheck)
	// don't cache account balance, as it may change frequently
	mux.HandleFunc("GET /account/{accountId}/{currency}", (handler.GetAccountBalance))
	// only cache transactions, as they are fixed
	mux.HandleFunc("GET /transactions/{accountId}/{currency}", middlewareChain(handler.GetTransactions))
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
		MaxHeaderBytes:    1 << 20,
	}
}
