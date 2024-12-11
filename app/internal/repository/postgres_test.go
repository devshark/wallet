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
	pgTesting "github.com/devshark/wallet/pkg/testing"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	if testing.Short() {
		t.Skip("Skipping test in short mode, as it requires a database")
	}

	connectionString, cleanup := pgTesting.SetupTestDB(t)

	db, err := sql.Open("postgres", connectionString)
	// db, err := sql.Open("postgres", "user=postgres password=postgres host=localhost port=5433 dbname=postgres sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	migrator := migration.NewMigrator(db, "../migration")
	err = migrator.Up(context.Background())
	require.NoError(t, err)

	return db, func() {
		// migrator.Down(context.Background())
		db.Close()
		cleanup()
	}
}

func TestPostgresRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.SkipNow()

	repo := repository.NewPostgresRepository(db)

	t.Run("GetAccountBalance", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		account, err := repo.GetAccountBalance(ctx, "USD", "user1")
		assert.NoError(t, err)
		assert.NotNil(t, account)
		assert.Equal(t, "USD", account.Currency)
		assert.Equal(t, "user1", account.AccountId)
	})

	t.Run("GetTransaction", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		tx, err := repo.GetTransaction(ctx, "some-transaction-id")
		assert.NoError(t, err)
		assert.NotNil(t, tx)
		// Add more specific assertions based on your implementation
	})

	t.Run("GetTransactions", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		txs, err := repo.GetTransactions(ctx, "USD", "user1")
		assert.NoError(t, err)
		assert.NotEmpty(t, txs)
		// Add more specific assertions based on your implementation
	})

	t.Run("Transfer", func(t *testing.T) {
		ctx := context.Background()

		request := &api.TransferRequest{
			FromAccountId: "user1",
			ToAccountId:   "user2",
			Currency:      "USD",
			Amount:        decimal.NewFromFloat(100.00),
		}

		txs, err := repo.Transfer(ctx, request, "some-idempotency-key")
		assert.NoError(t, err)
		assert.Len(t, txs, 2) // Expecting two transactions for a transfer
	})
}

func TestConcurrentTransfers(t *testing.T) {
	t.SkipNow()

	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewPostgresRepository(db)
	ctx := context.Background()

	// Setup initial balances
	_, err := repo.Transfer(ctx, &api.TransferRequest{
		FromAccountId: "company",
		ToAccountId:   "user1",
		Currency:      "USD",
		Amount:        decimal.NewFromFloat(1000.00),
	}, "initial-balance-user1")
	require.NoError(t, err)

	_, err = repo.Transfer(ctx, &api.TransferRequest{
		FromAccountId: "company",
		ToAccountId:   "user2",
		Currency:      "USD",
		Amount:        decimal.NewFromFloat(1000.00),
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

	assert.Empty(t, errors)

	// Check final balances
	user1Balance, err := repo.GetAccountBalance(ctx, "USD", "user1")
	require.NoError(t, err)
	user2Balance, err := repo.GetAccountBalance(ctx, "USD", "user2")
	require.NoError(t, err)

	expectedUser1Balance := decimal.NewFromFloat(900.00)
	expectedUser2Balance := decimal.NewFromFloat(1100.00)

	assert.True(t, user1Balance.Balance.Equal(expectedUser1Balance),
		"Expected user1 balance to be %s, but got %s", expectedUser1Balance, user1Balance.Balance)
	assert.True(t, user2Balance.Balance.Equal(expectedUser2Balance),
		"Expected user2 balance to be %s, but got %s", expectedUser2Balance, user2Balance.Balance)
}
