package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devshark/wallet/api"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestAccountReaderClient_GetAccountBalance(t *testing.T) {
	t.Run("Successful request", func(t *testing.T) {
		mockAccount := &api.Account{
			AccountID: "test123",
			Currency:  "USD",
			Balance:   decimal.NewFromFloat(100.50),
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/account/test123/USD", r.URL.Path)

			err := json.NewEncoder(w).Encode(mockAccount)

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		account, err := client.GetAccountBalance(context.Background(), "USD", "test123")

		require.NoError(t, err)
		require.Equal(t, mockAccount.AccountID, account.AccountID)
		require.Equal(t, mockAccount.Currency, account.Currency)
		require.True(t, mockAccount.Balance.Equal(account.Balance))
	})

	t.Run("Server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		_, err := client.GetAccountBalance(context.Background(), "USD", "test123")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 500")
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("invalid json"))

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		_, err := client.GetAccountBalance(context.Background(), "USD", "test123")

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("Context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)

			err := json.NewEncoder(w).Encode(&api.Account{})

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)

		defer cancel()

		_, err := client.GetAccountBalance(ctx, "USD", "test123")

		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}

func TestAccountReaderClient_GetTransaction(t *testing.T) {
	t.Run("Successful request", func(t *testing.T) {
		mockTransaction := &api.Transaction{
			TxID:      "tx123",
			AccountID: "acc123",
			Amount:    decimal.NewFromFloat(50.25),
			Type:      api.CREDIT,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/transactions/tx123", r.URL.Path)

			err := json.NewEncoder(w).Encode(mockTransaction)

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		transaction, err := client.GetTransaction(context.Background(), "tx123")

		require.NoError(t, err)
		require.Equal(t, mockTransaction.TxID, transaction.TxID)
		require.Equal(t, mockTransaction.AccountID, transaction.AccountID)
		require.True(t, mockTransaction.Amount.Equal(transaction.Amount))
		require.Equal(t, mockTransaction.Type, transaction.Type)
	})

	t.Run("Not found error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		_, err := client.GetTransaction(context.Background(), "nonexistent")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 404")
	})
}

func TestAccountReaderClient_GetTransactions(t *testing.T) {
	t.Run("Successful request", func(t *testing.T) {
		mockTransactions := []*api.Transaction{
			{
				TxID:      "tx123",
				AccountID: "acc123",
				Amount:    decimal.NewFromFloat(50.25),
				Type:      api.CREDIT,
			},
			{
				TxID:      "tx124",
				AccountID: "acc123",
				Amount:    decimal.NewFromFloat(25.75),
				Type:      api.DEBIT,
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/transactions/acc123/USD", r.URL.Path)

			err := json.NewEncoder(w).Encode(mockTransactions)

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		transactions, err := client.GetTransactions(context.Background(), "USD", "acc123")

		require.NoError(t, err)
		require.Len(t, transactions, 2)
		require.Equal(t, mockTransactions[0].TxID, transactions[0].TxID)
		require.Equal(t, mockTransactions[1].TxID, transactions[1].TxID)
		require.True(t, mockTransactions[0].Amount.Equal(transactions[0].Amount))
		require.True(t, mockTransactions[1].Amount.Equal(transactions[1].Amount))
	})

	t.Run("Empty response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			err := json.NewEncoder(w).Encode([]*api.Transaction{})

			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)
		transactions, err := client.GetTransactions(context.Background(), "USD", "acc123")

		require.NoError(t, err)
		require.Empty(t, transactions)
	})
}

func TestAccountReaderClient_ErrorHandling(t *testing.T) {
	t.Run("Network error", func(t *testing.T) {
		client := NewAccountReaderClient("http://nonexistent.example.com")

		_, err := client.GetAccountBalance(context.Background(), "USD", "test123")
		require.Error(t, err)

		_, err = client.GetTransaction(context.Background(), "tx123")
		require.Error(t, err)

		_, err = client.GetTransactions(context.Background(), "USD", "acc123")
		require.Error(t, err)
	})

	t.Run("unexpected error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)

		_, err := client.GetAccountBalance(context.Background(), "USD", "test123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 418")

		_, err = client.GetTransaction(context.Background(), "tx123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 418")

		_, err = client.GetTransactions(context.Background(), "USD", "acc123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected error: 418")
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte("invalid json"))
			require.NoError(t, err)
		}))
		defer server.Close()

		client := NewAccountReaderClient(server.URL)

		_, err := client.GetAccountBalance(context.Background(), "USD", "test123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")

		_, err = client.GetTransaction(context.Background(), "tx123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")

		_, err = client.GetTransactions(context.Background(), "USD", "acc123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")
	})
}

func TestAccountReaderClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)

		err := json.NewEncoder(w).Encode(&api.Account{})

		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewAccountReaderClient(server.URL)

	t.Run("GetAccountBalance", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetAccountBalance(ctx, "USD", "test123")
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})

	t.Run("GetTransaction", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetTransaction(ctx, "tx123")
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})

	t.Run("GetTransactions", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetTransactions(ctx, "USD", "acc123")
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}

func TestAccountReaderClient_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAccountReaderClient(server.URL)

	t.Run("GetAccountBalance", func(t *testing.T) {
		_, err := client.GetAccountBalance(context.Background(), "USD", "test123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "EOF")
	})

	t.Run("GetTransaction", func(t *testing.T) {
		_, err := client.GetTransaction(context.Background(), "tx123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "EOF")
	})

	t.Run("GetTransactions", func(t *testing.T) {
		_, err := client.GetTransactions(context.Background(), "USD", "acc123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "EOF")
	})
}

func TestOppositeType(t *testing.T) {
	require.Equal(t, api.DEBIT, api.OppositeType(api.CREDIT))
	require.Equal(t, api.CREDIT, api.OppositeType(api.DEBIT))
}
