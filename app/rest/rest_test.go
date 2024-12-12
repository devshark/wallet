package rest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devshark/wallet/api"
	"github.com/devshark/wallet/app/internal/repository"
	"github.com/devshark/wallet/app/rest"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRestHandlers(t *testing.T) {
	t.Run("HandleHealthCheck", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		req, err := http.NewRequest("GET", "/health", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleHealthCheck)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		require.Equal(t, "OK", rr.Body.String())
	})

	t.Run("GetAccountBalance", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockAccount := &api.Account{
			AccountId: "user1",
			Currency:  "USD",
			Balance:   decimal.NewFromFloat(100.00),
		}
		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "user1").Return(mockAccount, nil)

		req, err := http.NewRequest("GET", "/account/user1/USD", nil)
		require.NoError(t, err)
		req.SetPathValue("accountId", "user1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetAccountBalance)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var response api.Account
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, mockAccount.AccountId, response.AccountId)
		require.Equal(t, mockAccount.Currency, response.Currency)
		require.True(t, mockAccount.Balance.Equal(response.Balance))

		mockRepo.AssertExpectations(t)
	})

	t.Run("GetTransaction", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockTx := &api.Transaction{
			TxID:      "tx1",
			AccountId: "user1",
			Currency:  "USD",
			Amount:    decimal.NewFromFloat(50.00),
		}
		mockRepo.EXPECT().GetTransaction(mock.Anything, "tx1").Return(mockTx, nil)

		req, err := http.NewRequest("GET", "/transactions/tx1", nil)
		require.NoError(t, err)

		req.SetPathValue("txId", "tx1")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransaction)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var response api.Transaction
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, mockTx.TxID, response.TxID)
		require.Equal(t, mockTx.AccountId, response.AccountId)
		require.Equal(t, mockTx.Currency, response.Currency)
		require.True(t, mockTx.Amount.Equal(response.Amount))

		mockRepo.AssertExpectations(t)
	})

	t.Run("GetTransactions", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockTxs := []*api.Transaction{
			{TxID: "tx1", AccountId: "user1", Currency: "USD", Amount: decimal.NewFromFloat(50.00)},
			{TxID: "tx2", AccountId: "user1", Currency: "USD", Amount: decimal.NewFromFloat(25.00)},
		}
		mockRepo.EXPECT().GetTransactions(mock.Anything, "USD", "user1").Return(mockTxs, nil)

		req, err := http.NewRequest("GET", "/transactions/user1/USD", nil)
		require.NoError(t, err)

		req.SetPathValue("accountId", "user1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransactions)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var response []*api.Transaction
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Len(t, response, 2)
		require.Equal(t, mockTxs[0].TxID, response[0].TxID)
		require.Equal(t, mockTxs[1].TxID, response[1].TxID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("HandleWithdrawal", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		withdrawRequest := &api.WithdrawRequest{
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(50.00),
		}
		mockTxs := []*api.Transaction{
			{TxID: "tx1", AccountId: "user1", Currency: "USD", Type: api.DEBIT, Amount: decimal.NewFromFloat(-50.00)},
			{TxID: "tx2", AccountId: api.COMPANY_ACCOUNT_ID, Type: api.CREDIT, Currency: "USD", Amount: decimal.NewFromFloat(50.00)},
		}
		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(mockTxs, nil)

		body, _ := json.Marshal(withdrawRequest)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)

		var response api.Transaction
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.NoError(t, err)
		require.Equal(t, mockTxs[0].TxID, response.TxID)
		require.Equal(t, mockTxs[0].AccountId, response.AccountId)
		require.Equal(t, mockTxs[0].Currency, response.Currency)
		require.True(t, mockTxs[0].Amount.Equal(response.Amount))

		mockRepo.AssertExpectations(t)
	})

	t.Run("HandleDeposit", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		depositRequest := &api.DepositRequest{
			ToAccountId: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}
		mockTxs := []*api.Transaction{
			{TxID: "tx1", AccountId: api.COMPANY_ACCOUNT_ID, Type: api.DEBIT, Currency: "USD", Amount: decimal.NewFromFloat(-100.00)},
			{TxID: "tx2", AccountId: "user1", Currency: "USD", Type: api.CREDIT, Amount: decimal.NewFromFloat(100.00)},
		}
		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(mockTxs, nil)

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest("POST", "/deposit", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleDeposit)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)

		var response api.Transaction
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, mockTxs[1].TxID, response.TxID)
		require.Equal(t, mockTxs[1].AccountId, response.AccountId)
		require.Equal(t, mockTxs[1].Currency, response.Currency)
		require.True(t, mockTxs[1].Amount.Equal(response.Amount))

		mockRepo.AssertExpectations(t)
	})

	t.Run("HandleTransfer", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		transferRequest := &api.TransferRequest{
			FromAccountId: "user1",
			ToAccountId:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(75.00),
		}
		mockTxs := []*api.Transaction{
			{TxID: "tx1", AccountId: "user1", Currency: "USD", Type: api.DEBIT, Amount: decimal.NewFromFloat(-75.00)},
			{TxID: "tx2", AccountId: "user2", Currency: "USD", Type: api.CREDIT, Amount: decimal.NewFromFloat(75.00)},
		}
		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(mockTxs, nil)

		body, _ := json.Marshal(transferRequest)
		req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleTransfer)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)

		var response []*api.Transaction
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Len(t, response, 2)
		require.Equal(t, mockTxs[0].TxID, response[0].TxID)
		require.Equal(t, mockTxs[1].TxID, response[1].TxID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("HandleTransfer - Missing Idempotency Key", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		transferRequest := &api.TransferRequest{
			FromAccountId: "user1",
			ToAccountId:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(75.00),
		}

		body, _ := json.Marshal(transferRequest)
		req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		// Intentionally not setting X-Idempotency-Key

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleTransfer)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "missing idempotency key")
	})

	t.Run("HandleTransfer - Invalid Request", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		invalidRequest := struct {
			InvalidField string `json:"invalid_field"`
		}{
			InvalidField: "invalid",
		}

		body, _ := json.Marshal(invalidRequest)
		req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleTransfer)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "invalid request")
	})

	t.Run("HandleWithdrawal_InvalidRequest", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		invalidRequest := &api.WithdrawRequest{
			FromAccountId: "", // Invalid: empty account ID
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(0), // Invalid: zero amount
		}

		body, _ := json.Marshal(invalidRequest)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		var errorResponse api.ErrorResponse
		json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.Equal(t, "invalid request", errorResponse.Message)
	})

	t.Run("HandleDeposit_MissingIdempotencyKey", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		depositRequest := &api.DepositRequest{
			ToAccountId: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest("POST", "/deposit", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		// Intentionally not setting X-Idempotency-Key

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleDeposit)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		var errorResponse api.ErrorResponse
		json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.Equal(t, "missing idempotency key", errorResponse.Message)
	})

	t.Run("HandleWithdrawal_InsufficientBalance", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		withdrawRequest := &api.WithdrawRequest{
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(1000.00),
		}

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(nil, api.ErrInsufficientBalance)

		body, _ := json.Marshal(withdrawRequest)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		var errorResponse api.ErrorResponse
		json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.Equal(t, "insufficient balance", errorResponse.Message)

		mockRepo.AssertExpectations(t)
	})

	t.Run("HandleTransfer_InvalidAccountIDs", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		transferRequest := &api.TransferRequest{
			FromAccountId: "user1",
			ToAccountId:   "user1", // Same as FromAccountId, which is invalid
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(50.00),
		}

		body, _ := json.Marshal(transferRequest)
		req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleTransfer)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		var errorResponse api.ErrorResponse
		json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.Equal(t, "cannot transfer to the same account", errorResponse.Message)
	})

	t.Run("GetAccountBalance_RepositoryError", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "user1").Return(nil, errors.New("database error"))

		req, err := http.NewRequest("GET", "/account/user1/USD", nil)
		require.NoError(t, err)
		req.SetPathValue("accountId", "user1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetAccountBalance)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)
		var errorResponse api.ErrorResponse
		json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.Equal(t, "failed to get account balance", errorResponse.Message)

		mockRepo.AssertExpectations(t)
	})
}
