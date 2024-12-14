package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devshark/wallet/api"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestAccountOperatorClient_Deposit(t *testing.T) {
	t.Run("Successful deposit", func(t *testing.T) {
		mockTransaction := &api.Transaction{
			TxID:      "tx123",
			AccountID: "acc123",
			Amount:    decimal.NewFromFloat(100.50),
			Type:      api.CREDIT,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/deposit", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "test-key-1", r.Header.Get("X-Idempotency-Key"))

			err := json.NewEncoder(w).Encode(mockTransaction)

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.DepositRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		transaction, err := client.Deposit(context.Background(), request, "test-key-1")

		require.NoError(t, err)
		require.Equal(t, mockTransaction.TxID, transaction.TxID)
		require.Equal(t, mockTransaction.AccountID, transaction.AccountID)
		require.True(t, mockTransaction.Amount.Equal(transaction.Amount))
		require.Equal(t, mockTransaction.Type, transaction.Type)
	})

	t.Run("Server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.DepositRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		_, err := client.Deposit(context.Background(), request, "test-key-1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 500")
	})

	t.Run("Context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)

			err := json.NewEncoder(w).Encode(&api.Transaction{})

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.DepositRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.Deposit(ctx, request, "test-key-1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestAccountOperatorClient_Withdraw(t *testing.T) {
	t.Run("Successful withdrawal", func(t *testing.T) {
		mockTransaction := &api.Transaction{
			TxID:      "tx124",
			AccountID: "acc123",
			Amount:    decimal.NewFromFloat(50.25),
			Type:      api.DEBIT,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/withdraw", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "test-key-2", r.Header.Get("X-Idempotency-Key"))

			err := json.NewEncoder(w).Encode(mockTransaction)

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.WithdrawRequest{
			Currency:      "USD",
			FromAccountID: "acc123",
			Amount:        decimal.NewFromFloat(50.25),
		}

		transaction, err := client.Withdraw(context.Background(), request, "test-key-2")

		require.NoError(t, err)
		require.Equal(t, mockTransaction.TxID, transaction.TxID)
		require.Equal(t, mockTransaction.AccountID, transaction.AccountID)
		require.True(t, mockTransaction.Amount.Equal(transaction.Amount))
		require.Equal(t, mockTransaction.Type, transaction.Type)
	})

	t.Run("Server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.WithdrawRequest{
			Currency:      "USD",
			FromAccountID: "acc123",
			Amount:        decimal.NewFromFloat(100.50),
		}

		_, err := client.Withdraw(context.Background(), request, "test-key-1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 500")
	})

	t.Run("Context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)

			err := json.NewEncoder(w).Encode(&api.Transaction{})

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.WithdrawRequest{
			Currency:      "USD",
			FromAccountID: "acc123",
			Amount:        decimal.NewFromFloat(100.50),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.Withdraw(ctx, request, "test-key-1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestAccountOperatorClient_Transfer(t *testing.T) {
	t.Run("Successful transfer", func(t *testing.T) {
		mockTransaction := &api.Transaction{
			TxID:      "tx125",
			AccountID: "acc124",
			Amount:    decimal.NewFromFloat(75.00),
			Type:      api.CREDIT,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/transfer", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "test-key-3", r.Header.Get("X-Idempotency-Key"))

			err := json.NewEncoder(w).Encode(mockTransaction)

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.TransferRequest{
			FromAccountID: "acc123",
			ToAccountID:   "acc124",
			Amount:        decimal.NewFromFloat(75.00),
			Currency:      "USD",
		}

		transaction, err := client.Transfer(context.Background(), request, "test-key-3")

		require.NoError(t, err)
		require.Equal(t, mockTransaction.TxID, transaction.TxID)
		require.Equal(t, mockTransaction.AccountID, transaction.AccountID)
		require.True(t, mockTransaction.Amount.Equal(transaction.Amount))
		require.Equal(t, mockTransaction.Type, transaction.Type)
	})

	t.Run("Server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.TransferRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		_, err := client.Transfer(context.Background(), request, "test-key-1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 500")
	})

	t.Run("Context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)

			err := json.NewEncoder(w).Encode(&api.Transaction{})

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountOperatorClient(server.URL)
		client.WithName("AccountOperatorClient")

		request := &api.TransferRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.Transfer(ctx, request, "test-key-1")

		require.Error(t, err)
		require.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestAccountOperatorClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("invalid json"))

		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewAccountOperatorClient(server.URL)
	client.WithName("AccountOperatorClient")

	t.Run("Deposit with invalid JSON", func(t *testing.T) {
		request := &api.DepositRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		_, err := client.Deposit(context.Background(), request, "test-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("Withdraw with invalid JSON", func(t *testing.T) {
		request := &api.WithdrawRequest{
			Currency:      "USD",
			FromAccountID: "acc123",
			Amount:        decimal.NewFromFloat(100.50),
		}

		_, err := client.Withdraw(context.Background(), request, "test-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("Transfer with invalid JSON", func(t *testing.T) {
		request := &api.TransferRequest{
			Currency:      "USD",
			FromAccountID: "acc123",
			Amount:        decimal.NewFromFloat(100.50),
		}

		_, err := client.Transfer(context.Background(), request, "test-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")
	})
}

func TestAccountOperatorClient_NoSuchHost(t *testing.T) {
	client := NewAccountOperatorClient("http://nonexistent.example.com")
	client.WithName("AccountOperatorClient")

	t.Run("Deposit with no such host", func(t *testing.T) {
		request := &api.DepositRequest{
			Currency:    "USD",
			ToAccountID: "acc123",
			Amount:      decimal.NewFromFloat(100.50),
		}

		_, err := client.Deposit(context.Background(), request, "test-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such host")
	})

	t.Run("Withdraw with no such host", func(t *testing.T) {
		request := &api.WithdrawRequest{
			Currency:      "USD",
			FromAccountID: "acc123",
			Amount:        decimal.NewFromFloat(100.50),
		}

		_, err := client.Withdraw(context.Background(), request, "test-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such host")
	})

	t.Run("Transfer with no such host", func(t *testing.T) {
		request := &api.TransferRequest{
			FromAccountID: "acc123",
			ToAccountID:   "acc124",
			Amount:        decimal.NewFromFloat(75.00),
			Currency:      "USD",
		}

		_, err := client.Transfer(context.Background(), request, "test-key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such host")
	})
}
