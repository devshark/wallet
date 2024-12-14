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
	"go.uber.org/goleak"
)

func TestHandleHealthCheck(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockRepo := repository.NewMockRepository(t)
	handlers := rest.NewRestHandlers(mockRepo)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handlers.HandleHealthCheck)

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "OK", rr.Body.String())
}

func TestGetAccountBalance(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("OK", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockAccount := &api.Account{
			AccountID: "user1",
			Currency:  "USD",
			Balance:   decimal.NewFromFloat(100.00),
		}
		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "user1").Return(mockAccount, nil)

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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
		require.Equal(t, mockAccount.AccountID, response.AccountID)
		require.Equal(t, mockAccount.Currency, response.Currency)
		require.True(t, mockAccount.Balance.Equal(response.Balance))

		mockRepo.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockRepo.EXPECT().GetAccountBalance(mock.Anything, "USD", "user1").Return(nil, api.ErrAccountNotFound)

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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
			accountID     string
			currency      string
			expectedError error
		}{
			{accountID: "user1", currency: "", expectedError: api.ErrInvalidCurrency},
			{accountID: "", currency: "EUR", expectedError: api.ErrInvalidAccountID},
		}

		for _, request := range requests {
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)
			req.SetPathValue("accountId", request.accountID)
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

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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
	defer goleak.VerifyNone(t)

	t.Run("OK", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockTxs := []*api.Transaction{
			{TxID: "tx1", AccountID: "user1", Currency: "USD", Amount: decimal.NewFromFloat(50.00)},
			{TxID: "tx2", AccountID: "user1", Currency: "USD", Amount: decimal.NewFromFloat(25.00)},
		}
		mockRepo.EXPECT().GetTransactions(mock.Anything, "USD", "user1").Return(mockTxs, nil)

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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
			accountID     string
			currency      string
			expectedError error
		}{
			{accountID: "user1", currency: "", expectedError: api.ErrInvalidCurrency},
			{accountID: "", currency: "EUR", expectedError: api.ErrInvalidAccountID},
		}

		for _, request := range requests {
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			req.SetPathValue("accountId", request.accountID)
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

		req, err := http.NewRequest(http.MethodGet, "/", nil)
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
	defer goleak.VerifyNone(t)

	t.Run("OK", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		mockTx := &api.Transaction{
			TxID:      "tx1",
			AccountID: "user1",
			Currency:  "USD",
			Amount:    decimal.NewFromFloat(50.00),
		}
		mockRepo.EXPECT().GetTransaction(mock.Anything, "tx1").Return(mockTx, nil)

		req, err := http.NewRequest(http.MethodGet, "/transactions/tx1", nil)
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
		require.Equal(t, mockTx.AccountID, response.AccountID)
		require.Equal(t, mockTx.Currency, response.Currency)
		require.True(t, mockTx.Amount.Equal(response.Amount))

		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid transaction id", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		req, err := http.NewRequest(http.MethodGet, "/transactions/tx1", nil)
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

		req, err := http.NewRequest(http.MethodGet, "/transactions/tx1", nil)
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

		req, err := http.NewRequest(http.MethodGet, "/transactions/tx1", nil)
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

func TestHandleDepositOK(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockRepo := repository.NewMockRepository(t)
	handlers := rest.NewRestHandlers(mockRepo)

	depositRequest := &api.DepositRequest{
		ToAccountID: "user1",
		Currency:    "USD",
		Amount:      decimal.NewFromFloat(100.00),
	}
	mockTxs := []*api.Transaction{
		{TxID: "tx1", AccountID: api.CompanyAccountID, Type: api.DEBIT, Currency: "USD", Amount: decimal.NewFromFloat(-100.00)},
		{TxID: "tx2", AccountID: "user1", Currency: "USD", Type: api.CREDIT, Amount: decimal.NewFromFloat(100.00)},
	}
	mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(mockTxs, nil)

	body, err := json.Marshal(depositRequest)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
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
	require.Equal(t, mockTxs[1].AccountID, response.AccountID)
	require.Equal(t, mockTxs[1].Currency, response.Currency)
	require.True(t, mockTxs[1].Amount.Equal(response.Amount))

	mockRepo.AssertExpectations(t)
}

func TestHandleDepositInputValidation(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Missing Idempotency Key", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		depositRequest := &api.DepositRequest{
			ToAccountID: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		// Intentionally not setting X-Idempotency-Key

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleDeposit)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errorResponse api.ErrorResponse

		err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, errorResponse.ErrorCode)
		require.Equal(t, api.ErrMissingIdempotencyKey.Error(), errorResponse.Message)
	})

	t.Run("Invalid request", func(t *testing.T) {
		requests := []*api.DepositRequest{
			{
				ToAccountID: "",
				Currency:    "USD",
				Amount:      decimal.NewFromFloat(100.00),
			},
			{
				ToAccountID: "user1",
				Currency:    "",
				Amount:      decimal.NewFromFloat(100.00),
			},
			{
				ToAccountID: "user1",
				Currency:    "USD",
				Amount:      decimal.NewFromFloat(-100.00),
			},
			{
				ToAccountID: "user1",
				Currency:    "USD",
				Amount:      decimal.NewFromFloat(0.00),
			},
			nil,
		}

		handlers := rest.NewRestHandlers(nil)

		for _, request := range requests {
			body := []byte(":nul'") // Invalid JSON

			var req *http.Request

			var err error

			if request != nil {
				body, _ = json.Marshal(request)
			}

			req, err = http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleDeposit)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)

			var errorResponse api.ErrorResponse

			err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			require.NoError(t, err)
			require.Equal(t, api.ErrInvalidRequest.Error(), errorResponse.Message)
		}
	})
}

func TestHandleDeposit(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Deposit to Company Account", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		depositRequest := &api.DepositRequest{
			ToAccountID: api.CompanyAccountID,
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
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
			ToAccountID: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
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
			ToAccountID: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		mockedCases := []struct {
			errorMessage error
			errorCode    int
		}{
			{errorMessage: api.ErrSameAccountIDs, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAccountID, errorCode: http.StatusBadRequest},
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
			req, err := http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
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
			ToAccountID: "user1",
			Currency:    "USD",
			Amount:      decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest(http.MethodPost, "/deposit", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		// only one was present in the response
		receipts := []*api.Transaction{
			{
				TxID:      "tx1",
				AccountID: api.CompanyAccountID,
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

func TestHandleWithdrawalOK(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockRepo := repository.NewMockRepository(t)
	handlers := rest.NewRestHandlers(mockRepo)

	withdrawRequest := &api.WithdrawRequest{
		FromAccountID: "user1",
		Currency:      "USD",
		Amount:        decimal.NewFromFloat(50.00),
	}
	mockTxs := []*api.Transaction{
		{TxID: "tx1", AccountID: "user1", Currency: "USD", Type: api.DEBIT, Amount: decimal.NewFromFloat(-50.00)},
		{TxID: "tx2", AccountID: api.CompanyAccountID, Type: api.CREDIT, Currency: "USD", Amount: decimal.NewFromFloat(50.00)},
	}
	mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(mockTxs, nil)

	body, err := json.Marshal(withdrawRequest)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
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
	require.Equal(t, mockTxs[0].AccountID, response.AccountID)
	require.Equal(t, mockTxs[0].Currency, response.Currency)
	require.True(t, mockTxs[0].Amount.Equal(response.Amount))

	mockRepo.AssertExpectations(t)
}

func TestHandleWithdrawalInputValidation(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Invalid Requests", func(t *testing.T) {
		requests := []*api.WithdrawRequest{
			{
				FromAccountID: "",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
			},
			{
				FromAccountID: "user1",
				Currency:      "",
				Amount:        decimal.NewFromFloat(100.00),
			},
			{
				FromAccountID: "user1",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(-100.00),
			},
			{
				FromAccountID: "user1",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(0.00),
			},
			nil,
		}

		handlers := rest.NewRestHandlers(nil)

		for _, request := range requests {
			body := []byte(":nul'") // Invalid JSON

			var req *http.Request

			var err error

			if request != nil {
				body, _ = json.Marshal(request)
			}

			req, err = http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleWithdrawal)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)

			var errorResponse api.ErrorResponse

			err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			require.NoError(t, err)
			require.Equal(t, api.ErrInvalidRequest.Error(), errorResponse.Message)
		}
	})

	t.Run("Missing Idempotency Key", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.WithdrawRequest{
			FromAccountID: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(request)
		req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		// Intentionally not setting X-Idempotency-Key

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errorResponse api.ErrorResponse

		err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, errorResponse.ErrorCode)
		require.Equal(t, api.ErrMissingIdempotencyKey.Error(), errorResponse.Message)
	})
}

func TestHandleWithdrawal(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Insufficient Balance", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		withdrawRequest := &api.WithdrawRequest{
			FromAccountID: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(1000.00),
		}

		mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(nil, api.ErrInsufficientBalance)

		body, _ := json.Marshal(withdrawRequest)
		req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleWithdrawal)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusUnprocessableEntity, rr.Code)

		var errorResponse api.ErrorResponse

		err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		require.Equal(t, "insufficient balance", errorResponse.Message)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Withdraw from Company Account", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		depositRequest := &api.WithdrawRequest{
			FromAccountID: api.CompanyAccountID,
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
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
			FromAccountID: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(request)
		req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
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
			FromAccountID: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		mockedCases := []struct {
			errorMessage error
			errorCode    int
		}{
			{errorMessage: api.ErrSameAccountIDs, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAccountID, errorCode: http.StatusBadRequest},
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
			req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
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
			FromAccountID: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(depositRequest)
		req, err := http.NewRequest(http.MethodPost, "/withdraw", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		// only one was present in the response
		receipts := []*api.Transaction{
			{
				TxID:      "tx1",
				AccountID: api.CompanyAccountID,
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

func TestHandleTransferOK(t *testing.T) {
	defer goleak.VerifyNone(t)

	mockRepo := repository.NewMockRepository(t)
	handlers := rest.NewRestHandlers(mockRepo)

	transferRequest := &api.TransferRequest{
		FromAccountID: "user1",
		ToAccountID:   "user2",
		Currency:      "USD",
		Amount:        decimal.NewFromFloat(75.00),
	}
	mockTxs := []*api.Transaction{
		{TxID: "tx1", AccountID: "user1", Currency: "USD", Type: api.DEBIT, Amount: decimal.NewFromFloat(-75.00)},
		{TxID: "tx2", AccountID: "user2", Currency: "USD", Type: api.CREDIT, Amount: decimal.NewFromFloat(75.00)},
	}
	mockRepo.EXPECT().Transfer(mock.Anything, mock.AnythingOfType("*api.TransferRequest"), mock.AnythingOfType("string")).Return(mockTxs, nil)

	body, err := json.Marshal(transferRequest)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
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
}

func TestHandleTransferInputValidations(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Missing Idempotency Key", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		transferRequest := &api.TransferRequest{
			FromAccountID: "user1",
			ToAccountID:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(75.00),
		}

		body, _ := json.Marshal(transferRequest)
		req, err := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
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
				FromAccountID: "user1",
			}, {
				ToAccountID: "user2",
			}, {
				Amount: decimal.NewFromFloat(75.00),
			}, {
				Remarks: "test",
			},
			nil,
		}

		for _, request := range requests {
			body := []byte(":nul'") // Invalid JSON

			var req *http.Request

			var err error

			if request != nil {
				body, _ = json.Marshal(request)
			}

			req, err = http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Idempotency-Key", "test-key")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlers.HandleTransfer)

			handler.ServeHTTP(rr, req)

			require.Equal(t, http.StatusBadRequest, rr.Code)

			var errorResponse api.ErrorResponse

			err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, errorResponse.ErrorCode)
			require.Equal(t, api.ErrInvalidRequest.Error(), errorResponse.Message)
		}
	})

	t.Run("Same Account IDs", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		transferRequest := &api.TransferRequest{
			FromAccountID: "user1",
			ToAccountID:   "user1", // Same as FromAccountID, which is invalid
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(50.00),
		}

		body, _ := json.Marshal(transferRequest)
		req, err := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", "test-key")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(handlers.HandleTransfer)

		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusBadRequest, rr.Code)

		var errorResponse api.ErrorResponse

		err = json.Unmarshal(rr.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		require.Equal(t, api.ErrSameAccountIDs.Error(), errorResponse.Message)
	})

	t.Run("Company Account", func(t *testing.T) {
		handlers := rest.NewRestHandlers(nil)

		requests := []*api.TransferRequest{
			{
				ToAccountID:   "user1",
				FromAccountID: api.CompanyAccountID,
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
			},
			{
				ToAccountID:   api.CompanyAccountID,
				FromAccountID: "user2",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
			},
		}

		for _, request := range requests {
			body, _ := json.Marshal(request)
			req, err := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
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
}

func TestHandleTransfer(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("Repo failed", func(t *testing.T) {
		mockRepo := repository.NewMockRepository(t)
		handlers := rest.NewRestHandlers(mockRepo)

		request := &api.TransferRequest{
			FromAccountID: "user1",
			ToAccountID:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		body, _ := json.Marshal(request)
		req, err := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
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
			ToAccountID:   "user2",
			FromAccountID: "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		mockedCases := []struct {
			errorMessage error
			errorCode    int
		}{
			{errorMessage: api.ErrSameAccountIDs, errorCode: http.StatusBadRequest},
			{errorMessage: api.ErrInvalidAccountID, errorCode: http.StatusBadRequest},
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
			req, err := http.NewRequest(http.MethodPost, "/transfer", bytes.NewBuffer(body))
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
