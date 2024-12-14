package middlewares

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCacheMiddleware struct {
	client      GetterAndSetter
	nextHandler http.Handler
	expiration  time.Duration
	logger      *log.Logger
}

type GetterAndSetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

func NewRedisCacheMiddleware(client GetterAndSetter, expiration time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		obj := &RedisCacheMiddleware{
			client:      client,
			nextHandler: next,
			expiration:  expiration,
		}

		return obj.serveHTTP
	}
}

func (m *RedisCacheMiddleware) WithLogger(logger *log.Logger) {
	m.logger = logger
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
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")

		_, err := w.Write(cachedResponse)
		if err != nil {
			// we can't respond with an error payload anymore, because the headers have already been sent
			// headers must be written before the content, so if writing the content fails, we can't go back
			// just log it
			m.logger.Printf("encoding error: %v", err)
		}

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
	code, err := rww.writer.Write(data)
	if err != nil {
		return code, fmt.Errorf("cache write error: %w", err)
	}

	return code, nil
}

func (rww *responseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
	rww.ResponseWriter.WriteHeader(statusCode)
}
