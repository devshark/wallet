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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
	if testing.Short() {
		t.Skip("Skipping test in short mode, as it requires a database")
	}

	db, err := sql.Open("postgres", "user=postgres password=postgres host=localhost port=5433 dbname=postgres sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	// the migration files are here
	migrator := migration.NewMigrator(db, "../../../migrations")
	err = migrator.Up(context.Background())
	require.NoError(t, err)

	return db
}

type dummyAccount struct {
	accountId string
	userId    string
}

type dummyTransaction struct {
	transactionId string
}

func createAccount(t *testing.T, ctx context.Context, db *sql.DB, subject *api.Account) dummyAccount {
	t.Helper()

	var accountId string
	err := db.QueryRowContext(ctx, `
		INSERT INTO accounts (user_id, currency, balance)
		VALUES ($1, $2, $3) RETURNING id`, subject.AccountId, subject.Currency, subject.Balance).Scan(&accountId)
	require.NoError(t, err)
	require.NotEmpty(t, accountId)

	return dummyAccount{
		accountId: accountId,
		userId:    subject.AccountId,
	}
}

func createAccounts(t *testing.T, ctx context.Context, db *sql.DB, count int) []dummyAccount {
	t.Helper()

	dummyAccounts := make([]dummyAccount, count)
	for i := 0; i < count; i++ {
		dummyAccounts[i] = createAccount(t, ctx, db, &api.Account{
			AccountId: "account_" + strconv.Itoa(i),
			Currency:  "USD",
			Balance:   decimal.NewFromInt(0),
		})
	}

	return dummyAccounts
}

func createTransaction(t *testing.T, ctx context.Context, db *sql.DB, accountId string, subject *api.Transaction) dummyTransaction {
	t.Helper()

	var transactionId string
	err := db.QueryRowContext(ctx, `
		INSERT INTO transactions (account_id, amount, group_id, description, debit_credit)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		accountId, subject.Amount, subject.Remarks, subject.Remarks, subject.Type).Scan(&transactionId)
	require.NoError(t, err)

	return dummyTransaction{transactionId: transactionId}
}

func createTransactions(t *testing.T, ctx context.Context, db *sql.DB, dummyAccounts []dummyAccount) []dummyTransaction {
	t.Helper()

	dummyTransactions := make([]dummyTransaction, len(dummyAccounts))

	for i, dummyAccount := range dummyAccounts {
		dummyTransactions[i] = createTransaction(t, ctx, db, dummyAccount.accountId, &api.Transaction{
			AccountId: dummyAccount.accountId,
			Amount:    decimal.NewFromFloat(50.00),
			Remarks:   "CreateTransactions",
			Type:      api.CREDIT,
		})
	}

	return dummyTransactions
}

func createAccountTransactions(t *testing.T, ctx context.Context, db *sql.DB, dbAccountId string, account *api.Account, count int) []dummyTransaction {
	t.Helper()

	dummyTransactions := make([]dummyTransaction, count)

	for i := 0; i < count; i++ {
		dummyTransactions[i] = createTransaction(t, ctx, db, dbAccountId, &api.Transaction{
			AccountId: account.AccountId,
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

		_, err = db.ExecContext(context.Background(), "TRUNCATE TABLE accounts CASCADE;")
		require.NoError(t, err)

		_, err = db.ExecContext(context.Background(), "TRUNCATE TABLE transactions")
		require.NoError(t, err)

		db.Close()
	})
}

func TestGetAccountBalance(t *testing.T) {
	db := setupTestDB(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()
		var err error

		setCleanUp(t, db)

		subject := &api.Account{
			AccountId: "GetAccountBalance_user1",
			Currency:  "USD",
			Balance:   decimal.NewFromFloat(100.00),
		}

		_ = createAccount(t, ctx, db, subject)
		// defer dummyAccount.cleanFunc()

		account, err := repo.GetAccountBalance(ctx, subject.Currency, subject.AccountId)
		require.NoError(t, err)
		require.NotNil(t, account)
		require.Equal(t, subject.Currency, account.Currency)
		require.Equal(t, subject.AccountId, account.AccountId)
		require.True(t, account.Balance.Equal(decimal.NewFromFloat(100.00)))
	})
}

func TestGetTransaction(t *testing.T) {
	db := setupTestDB(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()
		var err error

		setCleanUp(t, db)

		dummyAccounts := createAccounts(t, ctx, db, 5)
		dummyTransactions := createTransactions(t, ctx, db, dummyAccounts)

		tx, err := repo.GetTransaction(ctx, dummyTransactions[0].transactionId)
		require.NoError(t, err)
		require.NotNil(t, tx)

		require.NotEmpty(t, tx.TxID)
		require.Equal(t, dummyTransactions[0].transactionId, tx.TxID)
		require.Equal(t, dummyAccounts[0].userId, tx.AccountId)
		require.Equal(t, "USD", tx.Currency)
		require.True(t, tx.Amount.Equal(decimal.NewFromFloat(50.00)))
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
			AccountId: "user1",
			Currency:  "USD",
			Balance:   decimal.NewFromInt(0),
		}

		dbAccountId := createAccount(t, ctx, db, subject)
		_ = createAccountTransactions(t, ctx, db, dbAccountId.accountId, subject, 5)

		txs, err := repo.GetTransactions(ctx, subject.Currency, subject.AccountId)
		require.NoError(t, err)
		require.NotEmpty(t, txs)
		require.Len(t, txs, 5)

		for _, tx := range txs {
			require.Equal(t, "USD", tx.Currency)
			require.Equal(t, "user1", tx.AccountId)
			require.NotEmpty(t, tx.TxID)
			require.True(t, tx.Amount.Equal(decimal.NewFromFloat(50.00)))
		}
	})

}

func TestTransfer(t *testing.T) {
	db := setupTestDB(t)

	repo := repository.NewPostgresRepository(db)

	t.Run("OK", func(t *testing.T) {
		ctx := context.Background()

		setCleanUp(t, db)

		request := &api.TransferRequest{
			FromAccountId: api.COMPANY_ACCOUNT_ID,
			ToAccountId:   "user2",
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
	db := setupTestDB(t)

	setCleanUp(t, db)

	repo := repository.NewPostgresRepository(db)
	ctx := context.Background()

	// Setup initial balances
	_, err := repo.Transfer(ctx, &api.TransferRequest{
		FromAccountId: api.COMPANY_ACCOUNT_ID,
		ToAccountId:   "user1",
		Currency:      "USD",
		Amount:        decimal.NewFromFloat(1000.00),
		Remarks:       "TestConcurrentTransfers",
	}, "initial-balance-user1")
	require.NoError(t, err)

	_, err = repo.Transfer(ctx, &api.TransferRequest{
		FromAccountId: api.COMPANY_ACCOUNT_ID,
		ToAccountId:   "user2",
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

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			_, err := repo.Transfer(ctx, &api.TransferRequest{
				FromAccountId: "user1",
				ToAccountId:   "user2",
				Currency:      "USD",
				Amount:        transferAmount,
				Remarks:       "TestConcurrentTransfers",
			}, fmt.Sprintf("concurrent-transfer-%s", strconv.Itoa(i)))

			require.NoError(t, err)
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
}
