package middlewares_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devshark/wallet/pkg/middlewares"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRedisCacheMiddleware(t *testing.T) {
	t.Run("Cache miss", func(t *testing.T) {
		mockRedis := middlewares.NewMockGetterAndSetter(t)
		mockRedis.On("Get", mock.Anything, "/test").Return(redis.NewStringResult("", redis.Nil))
		mockRedis.On("Set", mock.Anything, "/test", mock.Anything, 5*time.Minute).Return(redis.NewStatusResult("OK", nil))

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Hello, World!"})
		})

		middleware := middlewares.NewRedisCacheMiddleware(mockRedis, 5*time.Minute)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware(handler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"message":"Hello, World!"}`, rec.Body.String())
		assert.Empty(t, rec.Header().Get("X-Cache"))

		mockRedis.AssertExpectations(t)
	})

	t.Run("Cache hit", func(t *testing.T) {
		mockRedis := middlewares.NewMockGetterAndSetter(t)
		cachedResponse, _ := json.Marshal(map[string]string{"message": "Cached response"})
		mockRedis.On("Get", mock.Anything, "/test").Return(redis.NewStringResult(string(cachedResponse), nil))

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called on cache hit")
		})

		middleware := middlewares.NewRedisCacheMiddleware(mockRedis, 5*time.Minute)
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		middleware(handler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"message":"Cached response"}`, rec.Body.String())
		assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))

		mockRedis.AssertExpectations(t)
	})

	t.Run("Non-GET request", func(t *testing.T) {
		mockRedis := middlewares.NewMockGetterAndSetter(t)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "POST request"})
		})

		middleware := middlewares.NewRedisCacheMiddleware(mockRedis, 5*time.Minute)
		req := httptest.NewRequest("POST", "/test", nil)
		rec := httptest.NewRecorder()

		middleware(handler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"message":"POST request"}`, rec.Body.String())
		assert.Empty(t, rec.Header().Get("X-Cache"))

		// Ensure no Redis operations were performed
		mockRedis.AssertNotCalled(t, "Get")
		mockRedis.AssertNotCalled(t, "Set")
	})
}
