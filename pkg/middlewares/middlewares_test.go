package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devshark/wallet/pkg/middlewares"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareChain(t *testing.T) {
	t.Run("Empty chain", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		chainedHandler := middlewares.MiddlewareChain()(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		res := httptest.NewRecorder()

		chainedHandler.ServeHTTP(res, req)

		require.Equal(t, http.StatusOK, res.Code)
	})

	t.Run("Single middleware", func(t *testing.T) {
		called := false
		middleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				called = true

				next(w, r)
			}
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		chainedHandler := middlewares.MiddlewareChain(middleware)(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		res := httptest.NewRecorder()

		chainedHandler.ServeHTTP(res, req)

		require.True(t, called)
		require.Equal(t, http.StatusOK, res.Code)
	})

	t.Run("Multiple middlewares", func(t *testing.T) {
		order := []string{}
		middleware1 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "mw1 before")

				next(w, r)

				order = append(order, "mw1 after")
			}
		}
		middleware2 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "mw2 before")

				next(w, r)

				order = append(order, "mw2 after")
			}
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			order = append(order, "handler")

			w.WriteHeader(http.StatusOK)
		})

		chainedHandler := middlewares.MiddlewareChain(middleware1, middleware2)(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		res := httptest.NewRecorder()

		chainedHandler.ServeHTTP(res, req)

		require.Equal(t, http.StatusOK, res.Code)
		require.Equal(t, []string{
			"mw2 before",
			"mw1 before",
			"handler",
			"mw1 after",
			"mw2 after",
		}, order)
	})
}
