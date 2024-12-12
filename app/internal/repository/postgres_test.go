package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/devshark/wallet/api"
	"github.com/devshark/wallet/app/internal/migration"
	"github.com/devshark/wallet/app/internal/repository"
	"github.com/google/uuid"
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

	originalWd, _ := os.Getwd()
	t.Log("Current working directory:", originalWd)

	migrator := migration.NewMigrator(db, "../../../migration")
	err = migrator.Up(context.Background())
	require.NoError(t, err)

	return db
}

func TestPostgresRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewPostgresRepository(db)

	t.Run("GetAccountBalance", func(t *testing.T) {
		ctx := context.Background()

		defer db.Exec("DELETE FROM transactions WHERE description = 'GetAccountBalance';")

		_, err := db.Exec(`
			INSERT INTO transactions (account_id, currency, amount, balance, group_id, description, debit_credit)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			"user1", "USD", 100.00, 100.00, "GetAccountBalance1", "GetAccountBalance", api.CREDIT)
		require.NoError(t, err)

		account, err := repo.GetAccountBalance(ctx, "USD", "user1")
		require.NoError(t, err)
		require.NotNil(t, account)
		require.Equal(t, "USD", account.Currency)
		require.Equal(t, "user1", account.AccountId)
		require.True(t, account.Balance.Equal(decimal.NewFromFloat(100.00)))
	})

	t.Run("GetTransaction", func(t *testing.T) {
		ctx := context.Background()
		var err error

		id1 := uuid.New().String()
		id2 := uuid.New().String()

		defer db.ExecContext(ctx, "DELETE FROM transactions WHERE description = 'GetTransaction';")

		_, err = db.ExecContext(ctx, `
			INSERT INTO transactions (id, account_id, currency, amount, balance, group_id, description, debit_credit)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id1, "user1", "USD", 50.00, 80.99, "GetTransaction1", "GetTransaction", api.CREDIT)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx, `
			INSERT INTO transactions (id, account_id, currency, amount, balance, group_id, description, debit_credit)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id2, "user2", "USD", 50.00, 69.00, "GetTransaction1", "GetTransaction", api.DEBIT)
		require.NoError(t, err)

		tx, err := repo.GetTransaction(ctx, id1)
		require.NoError(t, err)
		require.NotNil(t, tx)

		require.NotEmpty(t, tx.TxID)
		require.Equal(t, "user1", tx.AccountId)
		require.Equal(t, "USD", tx.Currency)
		require.True(t, tx.Amount.Equal(decimal.NewFromFloat(50.00)))
	})

	t.Run("GetTransactions", func(t *testing.T) {
		ctx := context.Background()
		var err error

		// cleanup
		defer db.ExecContext(ctx, "DELETE FROM transactions WHERE description = 'GetTransactions'")

		for i := 0; i < 5; i++ {
			_, err = db.Exec(`
				INSERT INTO transactions (account_id, description, currency, amount, balance, group_id, debit_credit)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				"user1", "GetTransactions", "USD", 50.00, 202.53, "GetTransactions"+strconv.Itoa(i), api.CREDIT)
			require.NoError(t, err)
			_, err = db.Exec(`
				INSERT INTO transactions (account_id, description, currency, amount, balance, group_id, debit_credit)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				"user2", "GetTransactions", "USD", 50.00, 14532.3245, "GetTransactions"+strconv.Itoa(i), api.DEBIT)
			require.NoError(t, err)
		}

		txs, err := repo.GetTransactions(ctx, "USD", "user1")
		require.NoError(t, err)
		require.NotEmpty(t, txs)
		require.Len(t, txs, 5)

		for _, tx := range txs {
			require.Equal(t, "USD", tx.Currency)
			require.Equal(t, "user1", tx.AccountId)
			require.NotEmpty(t, tx.TxID)
			require.True(t, tx.Amount.Equal(decimal.NewFromFloat(50.00)))
			require.Equal(t, "GetTransactions", tx.Remarks)
		}
	})

	t.Run("Transfer", func(t *testing.T) {
		ctx := context.Background()

		request := &api.TransferRequest{
			FromAccountId: api.COMPANY_ACCOUNT_ID,
			ToAccountId:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
			Remarks:       "TestTransfer",
		}

		defer db.ExecContext(ctx, "DELETE FROM transactions WHERE description = 'TestTransfer';")

		txs, err := repo.Transfer(ctx, request, "some-idempotency-key")
		require.NoError(t, err)
		require.Len(t, txs, 2) // Expecting two transactions for a transfer
	})
}

func TestConcurrentTransfers(t *testing.T) {
	t.SkipNow()

	db := setupTestDB(t)
	defer db.Close()

	defer db.Exec("DELETE FROM transactions where description = 'TestConcurrentTransfers'")

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
	errors := make([]error, 0, concurrency)
	done := make(chan bool)

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
			if err != nil {
				errors = append(errors, err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < concurrency; i++ {
		<-done
	}

	wg.Wait()

	require.Empty(t, errors)

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
