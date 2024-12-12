package middlewares

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCacheMiddleware struct {
	client      *redis.Client
	nextHandler http.Handler
	expiration  time.Duration
}

func NewRedisCacheMiddleware(client *redis.Client, expiration time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		obj := &RedisCacheMiddleware{
			client:      client,
			nextHandler: next,
			expiration:  expiration,
		}

		return obj.serveHTTP
	}
}

func (m *RedisCacheMiddleware) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Only cache GET requests
	if r.Method != http.MethodGet {
		m.nextHandler.ServeHTTP(w, r)
		return
	}

	key := r.URL.String()
	ctx := r.Context()

	// Try to get the cached response
	cachedResponse, err := m.client.Get(ctx, key).Bytes()
	if err == nil {
		// Cache hit: return the cached response
		var response map[string]interface{}
		json.Unmarshal(cachedResponse, &response)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Cache miss: capture the response
	buf := &bytes.Buffer{}
	writer := io.MultiWriter(w, buf)
	wrappedWriter := wrapResponseWriter(w, writer)

	m.nextHandler.ServeHTTP(wrappedWriter, r)

	// Cache the response
	if wrappedWriter.statusCode == http.StatusOK {
		m.client.Set(ctx, key, buf.Bytes(), m.expiration)
	}
}

type responseWriterWrapper struct {
	http.ResponseWriter
	writer     io.Writer
	statusCode int
}

func wrapResponseWriter(w http.ResponseWriter, writer io.Writer) *responseWriterWrapper {
	return &responseWriterWrapper{ResponseWriter: w, writer: writer, statusCode: http.StatusOK}
}

func (rww *responseWriterWrapper) Write(data []byte) (int, error) {
	return rww.writer.Write(data)
}

func (rww *responseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
	rww.ResponseWriter.WriteHeader(statusCode)
}
