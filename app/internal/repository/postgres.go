package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/devshark/wallet/api"
	"github.com/shopspring/decimal"
)

type PostgresRepository struct {
	db     *sql.DB
	logger *log.Logger
}

const (
	insertStatement = `INSERT INTO transactions (account_id, amount, debit_credit, description, group_id) 
		VALUES ($1, $2, $3, $4, $5) RETURNING id`

	selectLockAccount = `SELECT id, balance
		FROM accounts
		WHERE user_id = $2 AND currency = $1
		FOR NO KEY UPDATE;`
	selectGroupExists    = `SELECT count(1) FROM transactions WHERE group_id = $1`
	selectAccountBalance = `SELECT balance
		FROM accounts
		WHERE currency = $1 AND user_id = $2;`

	selectTransaction = `
		SELECT transactions.id, 
			accounts.user_id, accounts.currency, transactions.amount, transactions.debit_credit, 
			accounts.balance, transactions.description, transactions.created_at 
		FROM transactions 
		JOIN accounts ON transactions.account_id = accounts.id
		WHERE transactions.id = $1`

	selectTransactions = `
		SELECT transactions.id, 
			accounts.user_id, accounts.currency, transactions.amount, transactions.debit_credit, 
			accounts.balance, transactions.description, transactions.created_at 
		FROM transactions 
		JOIN accounts ON transactions.account_id = accounts.id
		WHERE accounts.currency = $1 AND accounts.user_id = $2
		ORDER BY transactions.created_at DESC`

	selectTransactionPair = `
		SELECT transactions.id, 
			accounts.user_id, accounts.currency, transactions.amount, transactions.debit_credit, 
			accounts.balance, transactions.description, transactions.created_at 
		FROM transactions 
		JOIN accounts ON transactions.account_id = accounts.id
		WHERE transactions.id in ($1, $2)
		ORDER BY transactions.created_at DESC`

	updateAccountBalance = `UPDATE accounts SET balance = balance + $1 WHERE user_id = $2 AND currency = $3`

	upsertAccount = `
		INSERT INTO accounts (user_id, currency)
		VALUES ($1, $2)
		ON CONFLICT (user_id, currency)
		DO UPDATE SET user_id=EXCLUDED.user_id, currency=EXCLUDED.currency
		RETURNING id, balance;`
)

func NewPostgresRepository(db *sql.DB) Repository {
	return &PostgresRepository{
		db:     db,
		logger: log.Default(),
	}
}

func (r *PostgresRepository) WithCustomLogger(logger *log.Logger) Repository {
	r.logger = logger
	return r
}

func (r *PostgresRepository) GetAccountBalance(ctx context.Context, currency, accountId string) (*api.Account, error) {
	return r.getAccountBalance(ctx, r.db, currency, accountId)
}

type DbOrTxType interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func (r *PostgresRepository) getAccountBalance(ctx context.Context, dbOrTx DbOrTxType, currency, accountId string) (*api.Account, error) {
	account := &api.Account{
		Currency:  strings.ToUpper(strings.TrimSpace(currency)),
		AccountId: strings.TrimSpace(accountId),
	}

	if account.Currency == "" {
		return nil, api.ErrInvalidCurrency
	}

	if account.AccountId == "" {
		return nil, api.ErrInvalidAccountId
	}

	// the last transaction for the account currency
	row := dbOrTx.QueryRowContext(ctx, selectAccountBalance, account.Currency, account.AccountId)

	err := row.Scan(&account.Balance)
	if err != nil {
		// if the account is the company account, the initial balance must be 0
		if errors.Is(err, sql.ErrNoRows) && strings.EqualFold(account.AccountId, api.COMPANY_ACCOUNT_ID) {
			return &api.Account{
				Currency:  account.Currency,
				AccountId: account.AccountId,
				Balance:   decimal.NewFromInt(0),
			}, nil
		}
		return account, fmt.Errorf("failed to get account balance: %v: %w", err, api.ErrAccountNotFound)
	}

	return account, nil
}

func (r *PostgresRepository) GetTransaction(ctx context.Context, txId string) (*api.Transaction, error) {
	txId = strings.TrimSpace(txId)
	if txId == "" {
		return nil, api.ErrInvalidTxID
	}

	row := r.db.QueryRowContext(ctx, selectTransaction, txId)

	tx := &api.Transaction{}
	err := row.Scan(&tx.TxID, &tx.AccountId, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to get transaction: %v: %w", err, api.ErrTransactionNotFound)
		}
		return nil, err
	}

	return tx, nil
}

func (r *PostgresRepository) GetTransactions(ctx context.Context, currency, accountId string) ([]*api.Transaction, error) {
	if currency == "" {
		return nil, api.ErrInvalidCurrency
	}

	if accountId == "" {
		return nil, api.ErrInvalidAccountId
	}

	rows, err := r.db.QueryContext(ctx, selectTransactions, currency, accountId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to get transaction: %v: %w", err, api.ErrTransactionNotFound)
		}
		return nil, err
	}

	defer rows.Close()

	var transactions []*api.Transaction
	for rows.Next() {
		tx := &api.Transaction{}

		err := rows.Scan(&tx.TxID, &tx.AccountId, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("failed to get transaction: %v: %w", err, api.ErrTransactionNotFound)
			}
			return nil, err
		}

		transactions = append(transactions, tx)
	}

	return transactions, nil
}

func (r *PostgresRepository) Transfer(ctx context.Context, request *api.TransferRequest, idempotencyKey string) ([]*api.Transaction, error) {
	request.Currency = strings.ToUpper(strings.TrimSpace(request.Currency))
	request.FromAccountId = strings.TrimSpace(request.FromAccountId)
	request.ToAccountId = strings.TrimSpace(request.ToAccountId)

	if request.Currency == "" {
		return nil, api.ErrInvalidCurrency
	}

	if request.FromAccountId == "" {
		return nil, api.ErrInvalidAccountId
	}

	if request.ToAccountId == "" {
		return nil, api.ErrInvalidAccountId
	}

	if strings.EqualFold(request.FromAccountId, request.ToAccountId) {
		return nil, api.ErrSameAccountIds
	}

	if request.Amount.IsNegative() {
		return nil, api.ErrNegativeAmount
	}

	// check if the tx already exists
	existingTx := r.db.QueryRowContext(ctx, selectGroupExists, idempotencyKey)
	var existingCount int
	err := existingTx.Scan(&existingCount)
	if err != nil {
		return nil, err
	}

	if existingCount > 0 {
		return nil, api.ErrDuplicateTransaction
	}

	fromAccountDbId, toAccountDbId, err := getAccountDbIds(ctx, r.db, request)
	if err != nil { // may include api.ErrInsufficientBalance
		return nil, err
	}

	// start of the transaction
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}

	if err := lockAccounts(ctx, tx, request); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	newTxIdFromTransfer, newTxIdToTransfer, err := createDoubleEntry(ctx, tx, request, fromAccountDbId, toAccountDbId, idempotencyKey)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := updateBalances(ctx, tx, request); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	txs, err := r.getTransactionsByIds(ctx, newTxIdFromTransfer, newTxIdToTransfer)
	if err != nil {
		return nil, err
	}

	return txs, nil
}

// Create new double entry transactions of from and to accounts, respectively.
func createDoubleEntry(ctx context.Context, tx *sql.Tx, request *api.TransferRequest, fromAccountDbId, toAccountDbId, idempotencyKey string) (string, string, error) {
	// Prepare the reusable statement for optimized performance of repeated queries.
	newTxStatement, err := tx.PrepareContext(ctx, insertStatement)
	if err != nil {
		_ = tx.Rollback()
		return "", "", err
	}

	defer newTxStatement.Close()

	var newIdFromAccount string
	err = newTxStatement.QueryRowContext(ctx, fromAccountDbId, request.Amount, api.DEBIT, request.Remarks, idempotencyKey).Scan(&newIdFromAccount)
	if err != nil {
		return "", "", err
	}

	var newIdToAccount string
	err = newTxStatement.QueryRowContext(ctx, toAccountDbId, request.Amount, api.CREDIT, request.Remarks, idempotencyKey).Scan(&newIdToAccount)
	if err != nil {
		return "", "", err
	}

	return newIdFromAccount, newIdToAccount, nil
}

// Updates balances for both sides of the account.
func updateBalances(ctx context.Context, tx *sql.Tx, request *api.TransferRequest) error {
	// Prepare the reusable statement for optimized performance of repeated queries.
	updateBalanceStatement, err := tx.PrepareContext(ctx, updateAccountBalance)
	if err != nil {
		return err
	}

	_, err = updateBalanceStatement.ExecContext(ctx, request.Amount.Neg(), request.FromAccountId, request.Currency)
	if err != nil {
		return err
	}

	_, err = updateBalanceStatement.ExecContext(ctx, request.Amount, request.ToAccountId, request.Currency)
	if err != nil {
		return err
	}

	return nil
}

// Pessimistic lock of both accounts.
func lockAccounts(ctx context.Context, tx *sql.Tx, request *api.TransferRequest) error {
	// Prepare the reusable statement for optimized performance of repeated queries.
	lockStatement, err := tx.PrepareContext(ctx, selectLockAccount)
	if err != nil {
		return err
	}

	_, err = lockStatement.ExecContext(ctx, request.FromAccountId, request.Currency)
	if err != nil {
		return err
	}

	_, err = lockStatement.ExecContext(ctx, request.ToAccountId, request.Currency)
	if err != nil {
		return err
	}

	return nil
}

// Upsert ensures the account exists before we lock them. Returns the db ids of from and to accounts, respectively.
// May throw db errors or api.ErrInsufficientBalance if applicable.
func getAccountDbIds(ctx context.Context, db *sql.DB, request *api.TransferRequest) (string, string, error) {
	// Prepare the reusable statement for optimized performance.
	upsertAccountStatement, err := db.PrepareContext(ctx, upsertAccount)
	if err != nil {
		return "", "", err
	}

	defer upsertAccountStatement.Close()

	// Upsert the source account
	var fromAccountDbId string
	var fromAccountBalance decimal.Decimal
	if err := upsertAccountStatement.QueryRowContext(ctx, request.FromAccountId, request.Currency).Scan(&fromAccountDbId, &fromAccountBalance); err != nil {
		return "", "", err
	}

	allowNegative := strings.EqualFold(request.FromAccountId, api.COMPANY_ACCOUNT_ID)
	// check if the from account has enough balance.
	// but allow negative balance for company account.
	if fromAccountBalance.LessThan(request.Amount) && !allowNegative {
		return "", "", api.ErrInsufficientBalance
	}

	// Upsert the destination account
	var toAccountDbId string
	var toAccountBalance decimal.Decimal
	if err := upsertAccountStatement.QueryRowContext(ctx, request.ToAccountId, request.Currency).Scan(&toAccountDbId, &toAccountBalance); err != nil {
		return "", "", err
	}

	return fromAccountDbId, toAccountDbId, nil
}

func (r *PostgresRepository) lockAccountAndGetLatestBalance(ctx context.Context, tx *sql.Tx, currency, accountId string) *api.Account {
	_, _ = tx.ExecContext(ctx, selectLockAccount, currency, accountId)
	// result.LastInsertId()

	// account, err := r.getAccountBalance(ctx, tx, currency, accountId)
	account, err := r.GetAccountBalance(ctx, currency, accountId)
	if errors.Is(err, api.ErrAccountNotFound) {
		account.Balance = decimal.NewFromInt(0)
		return account
	}

	if err != nil {
		return nil
	}

	return account
}

func (r *PostgresRepository) getTransactionsByIds(ctx context.Context, txId1, txId2 string) ([]*api.Transaction, error) {
	transactions, err := r.db.QueryContext(ctx, selectTransactionPair, txId1, txId2)
	if err != nil {
		return nil, err
	}

	defer transactions.Close()

	// there are exactly 2 entries produced by every transfer
	txs := make([]*api.Transaction, 0, 2)
	for transactions.Next() {
		var tx api.Transaction
		err = transactions.Scan(&tx.TxID, &tx.AccountId, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
		if err != nil {
			return nil, err
		}

		txs = append(txs, &tx)
	}

	return txs, nil
}
