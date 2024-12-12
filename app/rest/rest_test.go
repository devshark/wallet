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

func TestHandleHealthCheck(t *testing.T) {
	mockRepo := repository.NewMockRepository(t)
	handlers := rest.NewRestHandlers(mockRepo)

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handlers.HandleHealthCheck)

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "OK", rr.Body.String())
}

func TestGetAccountBalance(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockAccount := &api.Account{
			AccountId: "user1",
			Currency:  "USD",
			Balance:   decimal.NewFromFloat(100.00),
		}
		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "user1").Return(mockAccount, nil)

		req, err := http.NewRequest("GET", "/", nil)
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

	t.Run("Not Found", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "user1").Return(nil, api.ErrAccountNotFound)

		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)
		req.SetPathValue("accountId", "user1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetAccountBalance)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, response.ErrorCode)
		require.Equal(t, api.ErrAccountNotFound.Error(), response.Message)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid requests", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		requests := []struct {
			accountId     string
			currency      string
			expectedError error
		}{
			{accountId: "user1", currency: "", expectedError: api.ErrInvalidCurrency},
			{accountId: "", currency: "EUR", expectedError: api.ErrInvalidAccountId},
		}

		for _, request := range requests {
			req, err := http.NewRequest("GET", "/", nil)
			require.NoError(t, err)
			req.SetPathValue("accountId", request.accountId)
			req.SetPathValue("currency", request.currency)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.GetAccountBalance)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)

			var response api.ErrorResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, response.ErrorCode)
			require.Equal(t, request.expectedError.Error(), response.Message)
		}
	})

	t.Run("Repo failed", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "account1").Return(nil, errors.New("some error"))

		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)
		req.SetPathValue("accountId", "account1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetAccountBalance)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.ErrorCode)
		require.Equal(t, api.ErrFailedToGetTransaction.Error(), response.Message)

		mockRepo.AssertExpectations(t)
	})
}

func TestGetTransactions(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockTxs := []*api.Transaction{
			{TxID: "tx1", AccountId: "user1", Currency: "USD", Amount: decimal.NewFromFloat(50.00)},
			{TxID: "tx2", AccountId: "user1", Currency: "USD", Amount: decimal.NewFromFloat(25.00)},
		}
		mockRepo.EXPECT().GetTransactions(mock.Anything, "USD", "user1").Return(mockTxs, nil)

		req, err := http.NewRequest("GET", "/", nil)
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

	t.Run("Empty", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetTransactions(mock.Anything, "USD", "user1").Return(nil, nil)

		req, err := http.NewRequest("GET", "/", nil)
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
		require.Len(t, response, 0)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetTransactions(mock.Anything, "USD", "Account1").Return(nil, api.ErrTransactionNotFound)

		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)

		req.SetPathValue("accountId", "Account1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransactions)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, response.ErrorCode)
		require.Equal(t, api.ErrTransactionNotFound.Error(), response.Message)
	})

	t.Run("Invalid requests", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		requests := []struct {
			accountId     string
			currency      string
			expectedError error
		}{
			{accountId: "user1", currency: "", expectedError: api.ErrInvalidCurrency},
			{accountId: "", currency: "EUR", expectedError: api.ErrInvalidAccountId},
		}

		for _, request := range requests {
			req, err := http.NewRequest("GET", "/", nil)
			require.NoError(t, err)

			req.SetPathValue("accountId", request.accountId)
			req.SetPathValue("currency", request.currency)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.GetTransactions)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)

			var response api.ErrorResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, response.ErrorCode)
			require.Equal(t, request.expectedError.Error(), response.Message)
		}
	})

	t.Run("Repo failed", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetTransactions(mock.Anything, "USD", "account1").Return(nil, errors.New("some error"))

		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)
		req.SetPathValue("accountId", "account1")
		req.SetPathValue("currency", "USD")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransactions)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.ErrorCode)
		require.Equal(t, api.ErrFailedToGetTransaction.Error(), response.Message)
	})
}

func TestGetTransaction(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
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

	t.Run("Invalid transaction id", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		req, err := http.NewRequest("GET", "/transactions/tx1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransaction)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, response.ErrorCode)
		require.Equal(t, api.ErrInvalidTxID.Error(), response.Message)
	})

	t.Run("Repo failed", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetTransaction(mock.Anything, "tx1").Return(nil, errors.New("some error"))

		req, err := http.NewRequest("GET", "/transactions/tx1", nil)
		require.NoError(t, err)

		req.SetPathValue("txId", "tx1")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransaction)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, response.ErrorCode)
		require.Equal(t, api.ErrFailedToGetTransaction.Error(), response.Message)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Tx not found", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetTransaction(mock.Anything, "tx1").Return(nil, api.ErrTransactionNotFound)

		req, err := http.NewRequest("GET", "/transactions/tx1", nil)
		require.NoError(t, err)

		req.SetPathValue("txId", "tx1")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.GetTransaction)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNotFound, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, response.ErrorCode)
		require.Equal(t, api.ErrTransactionNotFound.Error(), response.Message)

		mockRepo.AssertExpectations(t)
	})
}

func TestHandleDeposit(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
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

	t.Run("Missing Idempotency Key", func(t *testing.T) {
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
		require.Equal(t, http.StatusBadRequest, errorResponse.ErrorCode)
		require.Equal(t, api.ErrMissingIdempotencyKey.Error(), errorResponse.Message)
	})

	t.Run("Invalid request", func(t *testing.T) {
		requests := []*api.DepositRequest{
			{
				ToAccountId: "",
				Currency:    "USD",
				Amount:      decimal.NewFromFloat(100.00),
			},
			{
				ToAccountId: "user1",
				Currency:    "",
				Amount:      decimal.NewFromFloat(100.00),
			},
			{
				ToAccountId: "user1",
				Currency:    "USD",
				Amount:      decimal.NewFromFloat(-100.00),
			},
			{
				ToAccountId: "user1",
				Currency:    "USD",
				Amount:      decimal.NewFromFloat(0.00),
			},
			nil,
		}

		handlers := rest.NewRestHandlers(nil)

		for _, request := range requests {
			var body []byte = []byte(":nul'") // Invalid JSON
			var req *http.Request
			var err error
			if request != nil {
				body, _ = json.Marshal(request)
			}

			req, err = http.NewRequest("POST", "/deposit", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleDeposit)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errorResponse api.ErrorResponse
			json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			require.Equal(t, api.ErrInvalidRequest.Error(), errorResponse.Message)
		}
	})

	t.Run("Deposit to Company Account", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		depositRequest := &api.DepositRequest{
			ToAccountId: api.COMPANY_ACCOUNT_ID,
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest("POST", "/deposit", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleDeposit)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrCompanyAccount.Error(), response.Message)
		require.Equal(t, http.StatusBadRequest, response.ErrorCode)

	})

	t.Run("Repo failed", func(t *testing.T) {
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
		req.Header.Set("X-Idempotency-Key", "test-key")

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(nil, errors.New("failed"))

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleDeposit)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrTransferFailed.Error(), response.Message)
		require.Equal(t, http.StatusInternalServerError, response.ErrorCode)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Handled Errors", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		requests := &api.DepositRequest{
			ToAccountId: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		mockedCases := []struct {
			errorMessage error
			errorCode    int
		}{
			{errorMessage: api.ErrSameAccountIds, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAccountId, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidCurrency, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAmount, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidRequest, errorCode: http.StatusBadRequest},

			{errorMessage: api.ErrInsufficientBalance, errorCode: http.StatusUnprocessableEntity},
			{errorMessage: api.ErrDuplicateTransaction, errorCode: http.StatusUnprocessableEntity},

			{errorMessage: api.ErrTransferFailed, errorCode: http.StatusInternalServerError},
		}

		for _, mockedCase := range mockedCases {
			mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).
				Return(nil, mockedCase.errorMessage).
				Once()

			body, _ := json.Marshal(requests)
			req, err := http.NewRequest("POST", "/deposit", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleDeposit)

			handler.ServeHTTP(rr, req)

			require.Equal(t, mockedCase.errorCode, rr.Code)

			var response api.ErrorResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, mockedCase.errorMessage.Error(), response.Message)
			require.Equal(t, mockedCase.errorCode, response.ErrorCode)

			mockRepo.AssertExpectations(t)
		}
	})

	t.Run("Incomplete receipts", func(t *testing.T) {
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
		req.Header.Set("X-Idempotency-Key", "test-key")

		// only one was present in the response
		receipts := []*api.Transaction{
			{
				TxID:      "tx1",
				AccountId: api.COMPANY_ACCOUNT_ID,
				Currency:  "USD",
				Type:      api.DEBIT,
				Amount:    decimal.NewFromFloat(100.00),
			},
		}

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(receipts, nil)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleDeposit)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrIncompleteTransaction.Error(), response.Message)
		require.Equal(t, http.StatusUnprocessableEntity, response.ErrorCode)

		mockRepo.AssertExpectations(t)
	})
}

func TestHandleWithdrawal(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
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

	t.Run("Invalid Requests", func(t *testing.T) {
		requests := []*api.WithdrawRequest{
			{
				FromAccountId: "",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
			},
			{
				FromAccountId: "user1",
				Currency:      "",
				Amount:        decimal.NewFromFloat(100.00),
			},
			{
				FromAccountId: "user1",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(-100.00),
			},
			{
				FromAccountId: "user1",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(0.00),
			},
			nil,
		}

		handlers := rest.NewRestHandlers(nil)

		for _, request := range requests {
			var body []byte = []byte(":nul'") // Invalid JSON
			var req *http.Request
			var err error
			if request != nil {
				body, _ = json.Marshal(request)
			}

			req, err = http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleWithdrawal)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errorResponse api.ErrorResponse
			json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			require.Equal(t, api.ErrInvalidRequest.Error(), errorResponse.Message)
		}
	})

	t.Run("Missing Idempotency Key", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.WithdrawRequest{
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(request)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		// Intentionally not setting X-Idempotency-Key

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)
		var errorResponse api.ErrorResponse
		json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.Equal(t, http.StatusBadRequest, errorResponse.ErrorCode)
		require.Equal(t, api.ErrMissingIdempotencyKey.Error(), errorResponse.Message)
	})

	t.Run("Insufficient Balance", func(t *testing.T) {
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

	t.Run("Withdraw from Company Account", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		depositRequest := &api.WithdrawRequest{
			FromAccountId: api.COMPANY_ACCOUNT_ID,
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrCompanyAccount.Error(), response.Message)
		require.Equal(t, http.StatusBadRequest, response.ErrorCode)

	})

	t.Run("Repo failed", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.WithdrawRequest{
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(request)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(nil, errors.New("failed"))

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrTransferFailed.Error(), response.Message)
		require.Equal(t, http.StatusInternalServerError, response.ErrorCode)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Handled Errors", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.WithdrawRequest{
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		mockedCases := []struct {
			errorMessage error
			errorCode    int
		}{
			{errorMessage: api.ErrSameAccountIds, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAccountId, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidCurrency, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAmount, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidRequest, errorCode: http.StatusBadRequest},

			{errorMessage: api.ErrInsufficientBalance, errorCode: http.StatusUnprocessableEntity},
			{errorMessage: api.ErrDuplicateTransaction, errorCode: http.StatusUnprocessableEntity},

			{errorMessage: api.ErrTransferFailed, errorCode: http.StatusInternalServerError},
		}

		for _, mockedCase := range mockedCases {
			mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).
				Return(nil, mockedCase.errorMessage).
				Once()

			body, _ := json.Marshal(request)
			req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleWithdrawal)

			handler.ServeHTTP(rr, req)

			require.Equal(t, mockedCase.errorCode, rr.Code)

			var response api.ErrorResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, mockedCase.errorMessage.Error(), response.Message)
			require.Equal(t, mockedCase.errorCode, response.ErrorCode)

			mockRepo.AssertExpectations(t)
		}
	})

	t.Run("Incomplete receipts", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		depositRequest := &api.WithdrawRequest{
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest("POST", "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		// only one was present in the response
		receipts := []*api.Transaction{
			{
				TxID:      "tx1",
				AccountId: api.COMPANY_ACCOUNT_ID,
				Currency:  "USD",
				Type:      api.CREDIT,
				Amount:    decimal.NewFromFloat(100.00),
			},
		}

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(receipts, nil)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrIncompleteTransaction.Error(), response.Message)
		require.Equal(t, http.StatusUnprocessableEntity, response.ErrorCode)

		mockRepo.AssertExpectations(t)
	})
}

func TestHandleTransfer(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
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

	t.Run("Missing Idempotency Key", func(t *testing.T) {
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

	t.Run("Invalid Requests", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		requests := []*api.TransferRequest{
			{},
			{
				Currency: "USD",
			}, {
				FromAccountId: "user1",
			}, {
				ToAccountId: "user2",
			}, {
				Amount: decimal.NewFromFloat(75.00),
			}, {
				Remarks: "test",
			},
			nil,
		}

		for _, request := range requests {
			var body []byte = []byte(":nul'") // Invalid JSON
			var req *http.Request
			var err error
			if request != nil {
				body, _ = json.Marshal(request)
			}

			req, err = http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleTransfer)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var errorResponse api.ErrorResponse
			json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			require.Equal(t, http.StatusBadRequest, errorResponse.ErrorCode)
			require.Equal(t, api.ErrInvalidRequest.Error(), errorResponse.Message)
		}
	})

	t.Run("Same Account IDs", func(t *testing.T) {
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
		require.Equal(t, api.ErrSameAccountIds.Error(), errorResponse.Message)
	})

	t.Run("Company Account", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		requests := []*api.TransferRequest{
			{
				ToAccountId:   "user1",
				FromAccountId: api.COMPANY_ACCOUNT_ID,
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
			},
			{
				ToAccountId:   api.COMPANY_ACCOUNT_ID,
				FromAccountId: "user2",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
			},
		}

		for _, request := range requests {
			body, _ := json.Marshal(request)
			req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleTransfer)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)

			var response api.ErrorResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, api.ErrCompanyAccount.Error(), response.Message)
			require.Equal(t, http.StatusBadRequest, response.ErrorCode)
		}
	})

	t.Run("Repo failed", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.TransferRequest{
			FromAccountId: "user1",
			ToAccountId:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(request)
		req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(nil, errors.New("failed"))

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleTransfer)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusInternalServerError, rr.Code)

		var response api.ErrorResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, api.ErrTransferFailed.Error(), response.Message)
		require.Equal(t, http.StatusInternalServerError, response.ErrorCode)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Handled Errors", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.TransferRequest{
			ToAccountId:   "user2",
			FromAccountId: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		mockedCases := []struct {
			errorMessage error
			errorCode    int
		}{
			{errorMessage: api.ErrSameAccountIds, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAccountId, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidCurrency, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAmount, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidRequest, errorCode: http.StatusBadRequest},

			{errorMessage: api.ErrInsufficientBalance, errorCode: http.StatusUnprocessableEntity},
			{errorMessage: api.ErrDuplicateTransaction, errorCode: http.StatusUnprocessableEntity},

			{errorMessage: api.ErrTransferFailed, errorCode: http.StatusInternalServerError},
		}

		for _, mockedCase := range mockedCases {
			mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).
				Return(nil, mockedCase.errorMessage).
				Once()

			body, _ := json.Marshal(request)
			req, err := http.NewRequest("POST", "/transfer", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleTransfer)

			handler.ServeHTTP(rr, req)

			require.Equal(t, mockedCase.errorCode, rr.Code)

			var response api.ErrorResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, mockedCase.errorMessage.Error(), response.Message)
			require.Equal(t, mockedCase.errorCode, response.ErrorCode)

			mockRepo.AssertExpectations(t)
		}
	})
}
