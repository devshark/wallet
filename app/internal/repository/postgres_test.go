package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"sync"
	"testing"

	"github.com/devshark/wallet/api"
	"github.com/devshark/wallet/app/internal/migration"
	"github.com/devshark/wallet/app/internal/repository"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// simple build-time test to ensure repository.PostgresRepository implements repository.Repository
func ImplTest() repository.Repository { //nolint:ireturn,nolintlint
	return &repository.PostgresRepository{}
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping test in short mode, as it requires a database")
	}

	// if running using docker compose:
	// db, err := sql.Open("postgres", "user=postgres password=postgres host=postgres port=5432 dbname=postgres sslmode=disable")
	// if running outside the container:
	// db, err := sql.Open("postgres", "user=postgres password=postgres host=localhost port=5433 dbname=postgres sslmode=disable")
	db, err := sql.Open("postgres", "user=postgres password=postgres host=postgres port=5432 dbname=postgres sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	// the migration files are here
	migrator := migration.NewMigrator(db, "../../../migrations")
	err = migrator.Up(context.Background())
	require.NoError(t, err)

	return db
}

type dummyAccount struct {
	accountID string
	userID    string
}

type dummyTransaction struct {
	transactionID string
}

func createAccount(ctx context.Context, t *testing.T, db *sql.DB, subject *api.Account) dummyAccount {
	t.Helper()

	var accountID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO accounts (user_id, currency, balance)
		VALUES ($1, $2, $3) RETURNING id`, subject.AccountID, subject.Currency, subject.Balance).Scan(&accountID)
	require.NoError(t, err)
	require.NotEmpty(t, accountID)

	return dummyAccount{
		accountID: accountID,
		userID:    subject.AccountID,
	}
}

func createAccounts(ctx context.Context, t *testing.T, db *sql.DB, count int) []dummyAccount {
	t.Helper()

	dummyAccounts := make([]dummyAccount, count)
	for i := range count {
		dummyAccounts[i] = createAccount(ctx, t, db, &api.Account{
			AccountID: "account_" + strconv.Itoa(i),
			Currency:  "USD",
			Balance:   decimal.NewFromInt(0),
		})
	}

	return dummyAccounts
}

func createTransaction(ctx context.Context, t *testing.T, db *sql.DB, accountID string, subject *api.Transaction) dummyTransaction {
	t.Helper()

	var transactionID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO transactions (account_id, amount, group_id, description, debit_credit)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		accountID, subject.Amount, subject.Remarks, subject.Remarks, subject.Type).Scan(&transactionID)
	require.NoError(t, err)

	return dummyTransaction{transactionID: transactionID}
}

func createTransactions(ctx context.Context, t *testing.T, db *sql.DB, dummyAccounts []dummyAccount) []dummyTransaction {
	t.Helper()

	dummyTransactions := make([]dummyTransaction, len(dummyAccounts))

	for i, dummyAccount := range dummyAccounts {
		dummyTransactions[i] = createTransaction(ctx, t, db, dummyAccount.accountID, &api.Transaction{
			AccountID: dummyAccount.accountID,
			Amount:    decimal.NewFromFloat(50.00),
			Remarks:   "CreateTransactions",
			Type:      api.CREDIT,
		})
	}

	return dummyTransactions
}

func createAccountTransactions(ctx context.Context, t *testing.T, db *sql.DB, dbAccountID string, account *api.Account, count int) []dummyTransaction {
	t.Helper()

	dummyTransactions := make([]dummyTransaction, count)

	for i := range count {
		dummyTransactions[i] = createTransaction(ctx, t, db, dbAccountID, &api.Transaction{
			AccountID: account.AccountID,
			Amount:    decimal.NewFromFloat(50.00),
			Remarks:   "CreateAccountTransactions" + strconv.Itoa(i),
			Type:      api.CREDIT,
		})
	}

	return dummyTransactions
}

func setCleanUp(t *testing.T, db *sql.DB) {
	t.Helper()

	t.Cleanup(func() {
		defer goleak.VerifyNone(t)

		var err error

		defer db.Close()

		_, err = db.ExecContext(context.Background(), "TRUNCATE TABLE accounts CASCADE;")
		require.NoError(t, err)
	})
}

func TestGetAccountBalance(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)
	repo.WithCustomLogger(log.Default())

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		var err error

		subject := &api.Account{
			AccountID: "GetAccountBalance_user1",
			Currency:  "USD",
			Balance:   decimal.NewFromFloat(100.00),
		}

		_ = createAccount(ctx, t, db, subject)

		account, err := repo.GetAccountBalance(ctx, subject.Currency, subject.AccountID)
		require.NoError(t, err)
		require.NotNil(t, account)
		require.Equal(t, subject.Currency, account.Currency)
		require.Equal(t, subject.AccountID, account.AccountID)
		require.True(t, account.Balance.Equal(decimal.NewFromFloat(100.00)))
	})

	// no record in db, but can still return 0 for company balance
	t.Run("Company", func(t *testing.T) {
		ctx := context.Background()

		var err error

		subject := &api.Account{
			AccountID: api.CompanyAccountID,
			Currency:  "USD",
		}

		account, err := repo.GetAccountBalance(ctx, subject.Currency, subject.AccountID)
		require.NoError(t, err)
		require.NotNil(t, account)
		require.Equal(t, subject.Currency, account.Currency)
		require.Equal(t, subject.AccountID, account.AccountID)
		require.True(t, account.Balance.IsZero())
	})
}

func TestGetAccountBalanceFail(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)
	repo.WithCustomLogger(log.Default())

	t.Run("Validation Failed", func(t *testing.T) {
		ctx := context.Background()

		type tSubject struct {
			expectedError error
			account       *api.Account
		}

		subjects := []tSubject{
			{
				account: &api.Account{
					Currency: "USD",
					Balance:  decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidAccountID,
			},
			{
				account: &api.Account{
					Currency:  "USD",
					AccountID: "GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1", // mroe than 255 chars
					Balance:   decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidAccountID,
			},
			{
				account: &api.Account{
					Currency:  "ABCDEFGHIJKL", // more than 10 chars
					AccountID: "GetTransactions_user1",
					Balance:   decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidCurrency,
			},
			{
				account: &api.Account{
					AccountID: "GetTransactions_user1",
					Balance:   decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidCurrency,
			},
		}

		for _, subject := range subjects {
			account, err := repo.GetAccountBalance(ctx, subject.account.Currency, subject.account.AccountID)
			require.Error(t, err)
			require.ErrorIs(t, subject.expectedError, err)
			require.Nil(t, account)
		}
	})

	// no record in db and not company should return error
	t.Run("Account Not Found", func(t *testing.T) {
		ctx := context.Background()

		subject := &api.Account{
			AccountID: "randomguy",
			Currency:  "USD",
		}

		account, err := repo.GetAccountBalance(ctx, subject.Currency, subject.AccountID)
		require.Nil(t, account)
		require.ErrorIs(t, err, api.ErrAccountNotFound)
	})
}

func TestGetTransaction(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		var err error

		dummyAccounts := createAccounts(ctx, t, db, 5)
		dummyTransactions := createTransactions(ctx, t, db, dummyAccounts)

		tx, err := repo.GetTransaction(ctx, dummyTransactions[0].transactionID)
		require.NoError(t, err)
		require.NotNil(t, tx)

		require.NotEmpty(t, tx.TxID)
		require.Equal(t, dummyTransactions[0].transactionID, tx.TxID)
		require.Equal(t, dummyAccounts[0].userID, tx.AccountID)
		require.Equal(t, "USD", tx.Currency)
		require.True(t, tx.Amount.Equal(decimal.NewFromFloat(50.00)))
	})
}

func TestGetTransactionFail(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)
	repo.WithCustomLogger(log.Default())

	t.Run("Empty Tx ID", func(t *testing.T) {
		ctx := context.Background()

		tx, err := repo.GetTransaction(ctx, "")
		require.Nil(t, tx)
		require.ErrorIs(t, err, api.ErrInvalidTxID)
	})

	t.Run("No Record", func(t *testing.T) {
		ctx := context.Background()

		tx, err := repo.GetTransaction(ctx, "9dbbb6d8-7c13-482b-a381-ac282c52c51e")
		require.Nil(t, tx)
		require.ErrorIs(t, err, api.ErrTransactionNotFound)
	})
}

func TestGetTransactionsFail(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)
	repo.WithCustomLogger(log.Default())

	t.Run("Validation Failed", func(t *testing.T) {
		ctx := context.Background()

		type tSubject struct {
			expectedError error
			account       *api.Account
		}

		subjects := []tSubject{
			{
				account: &api.Account{
					Currency: "USD",
					Balance:  decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidAccountID,
			},
			{
				account: &api.Account{
					Currency:  "USD",
					AccountID: "GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1", // mroe than 255 chars
					Balance:   decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidAccountID,
			},
			{
				account: &api.Account{
					Currency:  "ABCDEFGHIJKL", // more than 10 chars
					AccountID: "GetTransactions_user1",
					Balance:   decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidCurrency,
			},
			{
				account: &api.Account{
					AccountID: "GetTransactions_user1",
					Balance:   decimal.NewFromFloat(100.00),
				},
				expectedError: api.ErrInvalidCurrency,
			},
		}

		for _, subject := range subjects {
			account, err := repo.GetTransactions(ctx, subject.account.Currency, subject.account.AccountID)
			require.Error(t, err)
			require.ErrorIs(t, subject.expectedError, err)
			require.Nil(t, account)
		}
	})

	t.Run("No transactions", func(t *testing.T) {
		ctx := context.Background()

		tx, err := repo.GetTransactions(ctx, "USD", "random-account999")
		require.NoError(t, err)
		require.NotNil(t, tx)
		require.Len(t, tx, 0)
		require.Empty(t, tx)
	})
}

func TestGetTransactions(t *testing.T) {
	db := setupTestDB(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		var err error

		setCleanUp(t, db)

		subject := &api.Account{
			AccountID: "user1",
			Currency:  "USD",
			Balance:   decimal.NewFromInt(0),
		}

		dbAccountID := createAccount(ctx, t, db, subject)
		_ = createAccountTransactions(ctx, t, db, dbAccountID.accountID, subject, 5)

		txs, err := repo.GetTransactions(ctx, subject.Currency, subject.AccountID)
		require.NoError(t, err)
		require.NotEmpty(t, txs)
		require.Len(t, txs, 5)

		for _, tx := range txs {
			require.Equal(t, "USD", tx.Currency)
			require.Equal(t, "user1", tx.AccountID)
			require.NotEmpty(t, tx.TxID)
			require.True(t, tx.Amount.Equal(decimal.NewFromFloat(50.00)))
		}
	})
}

func TestTransfer(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		request := &api.TransferRequest{
			FromAccountID: api.CompanyAccountID,
			ToAccountID:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
			Remarks:       "TestTransfer",
		}

		txs, err := repo.Transfer(ctx, request, "some-idempotency-key")
		require.NoError(t, err)
		require.Len(t, txs, 2) // Expecting two transactions for a transfer
	})

	t.Run("Refund", func(t *testing.T) {
		ctx := context.Background()

		requests := []*api.TransferRequest{
			{
				FromAccountID: api.CompanyAccountID,
				ToAccountID:   "user2",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "Test Transfer",
			},
			{
				FromAccountID: "user2",
				ToAccountID:   api.CompanyAccountID,
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "Refund Transfer",
			},
		}

		for i, request := range requests {
			txs, err := repo.Transfer(ctx, request, "some-idempotency-key-"+strconv.Itoa(i))
			require.NoError(t, err)
			require.Len(t, txs, 2) // Expecting two transactions for a transfer

			require.True(t, txs[0].Amount.Equal(txs[1].Amount))
			require.NotZero(t, txs[0].Amount)
			require.NotZero(t, txs[1].Amount)
			require.Equal(t, api.OppositeType(txs[0].Type), txs[1].Type)
			require.Equal(t, api.OppositeType(txs[1].Type), txs[0].Type)
			require.NotEmpty(t, txs[0].Remarks)
			require.NotEmpty(t, txs[1].Remarks)
			require.Equal(t, txs[0].Remarks, txs[1].Remarks)
			require.True(t, txs[0].RunningBalance.Equal(txs[1].RunningBalance.Neg()))
		}
	})
}

func TestTransferFail(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)

	t.Run("Validation Fail", func(t *testing.T) {
		ctx := context.Background()

		type requestWithError struct {
			api.TransferRequest
			expectedError error
		}

		requests := []*requestWithError{
			{
				expectedError: api.ErrInvalidAccountID,
				TransferRequest: api.TransferRequest{
					ToAccountID: api.CompanyAccountID,
					Currency:    "USD",
					Amount:      decimal.NewFromFloat(100.00),
					Remarks:     "ErrInvalidAccountID missing from",
				},
			},
			{
				expectedError: api.ErrInvalidAccountID,
				TransferRequest: api.TransferRequest{
					FromAccountID: api.CompanyAccountID,
					Currency:      "USD",
					Amount:        decimal.NewFromFloat(100.00),
					Remarks:       "ErrInvalidAccountID missing to",
				},
			},
			{
				expectedError: api.ErrInvalidCurrency,
				TransferRequest: api.TransferRequest{
					FromAccountID: api.CompanyAccountID,
					ToAccountID:   "user2",
					Amount:        decimal.NewFromFloat(100.00),
					Remarks:       "ErrInvalidCurrency missing",
				},
			},
			{
				expectedError: api.ErrNegativeAmount,
				TransferRequest: api.TransferRequest{
					FromAccountID: api.CompanyAccountID,
					ToAccountID:   "user2",
					Currency:      "USD",
					Amount:        decimal.NewFromFloat(-100.00),
					Remarks:       "ErrNegativeAmount",
				},
			},
			{
				expectedError: api.ErrInvalidAmount,
				TransferRequest: api.TransferRequest{
					FromAccountID: api.CompanyAccountID,
					ToAccountID:   "user2",
					Currency:      "USD",
					Remarks:       "ErrInvalidAmount missing",
				},
			},
			{
				expectedError: api.ErrInvalidCurrency,
				TransferRequest: api.TransferRequest{
					FromAccountID: api.CompanyAccountID,
					ToAccountID:   "user2",
					Currency:      "ABCDEFGHIJKL", // more than 10 chars
					Amount:        decimal.NewFromFloat(100.00),
					Remarks:       "ErrInvalidCurrency long currency",
				},
			},
			{
				expectedError: api.ErrInvalidAccountID,
				TransferRequest: api.TransferRequest{
					FromAccountID: api.CompanyAccountID,
					ToAccountID:   "GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1",
					Currency:      "USD",
					Amount:        decimal.NewFromFloat(100.00),
					Remarks:       "ErrInvalidAccountID to long",
				},
			},
			{
				expectedError: api.ErrInvalidAccountID,
				TransferRequest: api.TransferRequest{
					ToAccountID:   api.CompanyAccountID,
					FromAccountID: "GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1GetTransactions_user1",
					Currency:      "USD",
					Amount:        decimal.NewFromFloat(100.00),
					Remarks:       "ErrInvalidAccountID from long",
				},
			},
			{
				expectedError: api.ErrSameAccountIDs,
				TransferRequest: api.TransferRequest{
					ToAccountID:   api.CompanyAccountID,
					FromAccountID: api.CompanyAccountID,
					Currency:      "USD",
					Amount:        decimal.NewFromFloat(100.00),
					Remarks:       "ErrSameAccountIDs",
				},
			},
		}

		for _, request := range requests {
			txs, err := repo.Transfer(ctx, &request.TransferRequest, "doesnt-matter")
			require.Error(t, err)
			require.ErrorIs(t, err, request.expectedError)
			require.Nil(t, txs)
		}
	})

	t.Run("Duplicate Request", func(t *testing.T) {
		ctx := context.Background()

		request := &api.TransferRequest{
			FromAccountID: api.CompanyAccountID,
			ToAccountID:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
			Remarks:       "TestTransfer",
		}

		_, err := repo.Transfer(ctx, request, "duplicate-idempotency-key")
		require.NoError(t, err)

		txs, err := repo.Transfer(ctx, request, "duplicate-idempotency-key")
		require.Error(t, err)
		require.ErrorIs(t, err, api.ErrDuplicateTransaction)
		require.Nil(t, txs)
	})

	t.Run("Duplicate Idempotency Key", func(t *testing.T) {
		ctx := context.Background()

		requests := []*api.TransferRequest{
			{
				FromAccountID: api.CompanyAccountID,
				ToAccountID:   "user2",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "Test Transfer",
			},
			{
				FromAccountID: "user2",
				ToAccountID:   api.CompanyAccountID,
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "Refund Transfer",
			},
		}

		_, err := repo.Transfer(ctx, requests[0], "another-duplicate-idempotency-key")
		require.NoError(t, err)

		txs, err := repo.Transfer(ctx, requests[1], "another-duplicate-idempotency-key")
		require.Error(t, err)
		require.ErrorIs(t, err, api.ErrDuplicateTransaction)
		require.Nil(t, txs)
	})

	t.Run("Insufficient Balance", func(t *testing.T) {
		ctx := context.Background()

		requests := []api.TransferRequest{
			{
				FromAccountID: "Insufficient Balance user2",
				ToAccountID:   "user1",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "TestTransfer",
			},
			{
				FromAccountID: "Insufficient Balance user1",
				ToAccountID:   "user2",
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "TestTransfer",
			},
			{
				FromAccountID: "Insufficient Balance user2",
				ToAccountID:   api.CompanyAccountID,
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "TestTransfer",
			},
			{
				FromAccountID: "Insufficient Balance user1",
				ToAccountID:   api.CompanyAccountID,
				Currency:      "USD",
				Amount:        decimal.NewFromFloat(100.00),
				Remarks:       "TestTransfer",
			},
		}

		for _, request := range requests {
			txs, err := repo.Transfer(ctx, &request, "insufficient-balance-key")
			require.Error(t, err)
			require.ErrorIs(t, err, api.ErrInsufficientBalance)
			require.Nil(t, txs)
		}
	})
}

func TestTransferFailDBClosed(t *testing.T) {
	ctx := context.Background()

	defer goleak.VerifyNone(t)

	db := setupTestDB(t)

	repo := repository.NewPostgresRepository(db)

	db.Close()

	request := &api.TransferRequest{
		FromAccountID: api.CompanyAccountID,
		ToAccountID:   "user2",
		Currency:      "USD",
		Amount:        decimal.NewFromFloat(100.00),
		Remarks:       "TestTransfer",
	}

	txs, err := repo.Transfer(ctx, request, "db-error-key")
	require.Error(t, err)
	require.ErrorIs(t, err, api.ErrUnhandledDatabaseError)
	require.Nil(t, txs)
}

func TestConcurrentTransfers(t *testing.T) {
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		// Setup initial balances
		_, err := repo.Transfer(ctx, &api.TransferRequest{
			FromAccountID: api.CompanyAccountID,
			ToAccountID:   "user1",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(1000.00),
			Remarks:       "TestConcurrentTransfers",
		}, "initial-balance-user1")
		require.NoError(t, err)

		_, err = repo.Transfer(ctx, &api.TransferRequest{
			FromAccountID: api.CompanyAccountID,
			ToAccountID:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(1000.00),
			Remarks:       "TestConcurrentTransfers",
		}, "initial-balance-user2")
		require.NoError(t, err)

		// Concurrent transfers
		concurrency := 10
		transferAmount := decimal.NewFromFloat(10.00)

		var wg sync.WaitGroup

		wg.Add(concurrency)

		for i := range concurrency {
			go func() {
				defer wg.Done()

				_, errTransfer := repo.Transfer(ctx, &api.TransferRequest{
					FromAccountID: "user1",
					ToAccountID:   "user2",
					Currency:      "USD",
					Amount:        transferAmount,
					Remarks:       "TestConcurrentTransfers",
				}, fmt.Sprintf("concurrent-transfer-%s", strconv.Itoa(i)))

				require.NoError(t, errTransfer)
			}()
		}

		wg.Wait()

		// Check final balances
		user1Balance, err := repo.GetAccountBalance(ctx, "USD", "user1")
		require.NoError(t, err)
		user2Balance, err := repo.GetAccountBalance(ctx, "USD", "user2")
		require.NoError(t, err)

		expectedUser1Balance := decimal.NewFromFloat(900.00)
		expectedUser2Balance := decimal.NewFromFloat(1100.00)

		require.True(t, user1Balance.Balance.Equal(expectedUser1Balance),
			"Expected user1 balance to be %s, but got %s", expectedUser1Balance, user1Balance.Balance)
		require.True(t, user2Balance.Balance.Equal(expectedUser2Balance),
			"Expected user2 balance to be %s, but got %s", expectedUser2Balance, user2Balance.Balance)
	})
}
