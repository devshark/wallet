package repository_test

import (
	"context"
	"database/sql"
	"fmt"
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
		var err error

		defer db.Close()

		_, err = db.ExecContext(context.Background(), "TRUNCATE TABLE accounts CASCADE;")
		require.NoError(t, err)
	})
}

func TestGetAccountBalance(t *testing.T) {
	db := setupTestDB(t)

	defer goleak.VerifyNone(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		var err error

		setCleanUp(t, db)

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
}

func TestGetTransaction(t *testing.T) {
	db := setupTestDB(t)

	defer goleak.VerifyNone(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		var err error

		setCleanUp(t, db)

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

func TestGetTransactions(t *testing.T) {
	db := setupTestDB(t)

	defer goleak.VerifyNone(t)

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

	defer goleak.VerifyNone(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		setCleanUp(t, db)

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
}

func TestConcurrentTransfers(t *testing.T) {
	defer goleak.VerifyNone(t)

	db := setupTestDB(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		setCleanUp(t, db)

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
