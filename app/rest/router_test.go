package rest

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/devshark/wallet/app/internal/repository"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestNewRestApiServer(t *testing.T) {
	mockRepo := &repository.MockRepository{}
	server := NewRestApiServer(mockRepo)

	require.NotNil(t, server)
	require.Equal(t, mockRepo, server.repo)
	require.NotNil(t, server.logger)
	require.Empty(t, server.middlewares)
}

func TestWithCacheMiddleware(t *testing.T) {
	mockRepo := &repository.MockRepository{}
	server := NewRestApiServer(mockRepo)

	mockRedisClient := &redis.Client{}
	expiration := 5 * time.Minute

	updatedServer := server.WithCacheMiddleware(mockRedisClient, expiration)

	require.Equal(t, server, updatedServer)
	require.Len(t, server.middlewares, 1)
}

func TestWithCustomLogger(t *testing.T) {
	mockRepo := &repository.MockRepository{}
	server := NewRestApiServer(mockRepo)

	customLogger := log.New(os.Stdout, "TEST: ", log.Ldate|log.Ltime|log.Lshortfile)
	updatedServer := server.WithCustomLogger(customLogger)

	require.Equal(t, server, updatedServer)
	require.Equal(t, customLogger, server.logger)
}

func TestHttpServer(t *testing.T) {
	mockRepo := &repository.MockRepository{}
	server := NewRestApiServer(mockRepo)

	port := int64(8080)
	readTimeout := 5 * time.Second
	writeTimeout := 10 * time.Second

	httpServer := server.HttpServer(port, readTimeout, writeTimeout)

	require.NotNil(t, httpServer)
	require.Equal(t, fmt.Sprintf(":%d", port), httpServer.Addr)
	require.Equal(t, readTimeout, httpServer.ReadTimeout)
	require.Equal(t, writeTimeout, httpServer.WriteTimeout)

	// Test that routes are set up correctly
	mux, ok := httpServer.Handler.(*http.ServeMux)
	require.True(t, ok)

	// You can test individual routes if needed, for example:
	healthHandler, pattern := mux.Handler(httptest.NewRequest("GET", "/health", nil))
	require.NotNil(t, healthHandler)
	require.NotNil(t, pattern)
	require.NotEmpty(t, pattern)
}
