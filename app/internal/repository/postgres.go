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
		WHERE user_id = $1 AND currency = $2
		FOR NO KEY UPDATE;`
	selectGroupExists    = `SELECT count(1) FROM transactions WHERE group_id = $1`
	selectAccountBalance = `SELECT balance
		FROM accounts
		WHERE user_id = $1 AND currency = $2;`
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

	updateAccountBalance = `UPDATE accounts SET balance = balance + $1 WHERE user_id = $2 AND currency = $3 RETURNING balance;`

	upsertAccount = `
		INSERT INTO accounts (user_id, currency)
		VALUES ($1, $2)
		ON CONFLICT (user_id, currency)
		DO UPDATE SET user_id=EXCLUDED.user_id, currency=EXCLUDED.currency
		RETURNING id, balance;`
)

const (
	// because it isn't simple to know the number of rows in the result, we will initiate the slice with 10 capacity
	// we will not need this if we implement a proper pagination, but we aim to only deliver a simplified version.
	defaultTransactionSliceCapacity = 10

	// we know for certain that double-entry transactions always have exactly 2 records
	transactionPairCapacity = 2
)

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: log.Default(),
	}
}

func (r *PostgresRepository) WithCustomLogger(logger *log.Logger) *PostgresRepository {
	r.logger = logger

	return r
}

func (r *PostgresRepository) GetAccountBalance(ctx context.Context, currency, accountID string) (*api.Account, error) {
	account := &api.Account{
		Currency:  strings.ToUpper(strings.TrimSpace(currency)),
		AccountID: strings.TrimSpace(accountID),
	}

	if err := validateCurrencyAndAccount(currency, accountID); err != nil {
		return nil, err
	}

	// the last transaction for the account currency
	err := r.db.QueryRowContext(ctx, selectAccountBalance, account.AccountID, account.Currency).Scan(&account.Balance)
	if err != nil {
		// if the account is the company account, the initial balance must be 0
		if errors.Is(err, sql.ErrNoRows) && strings.EqualFold(account.AccountID, api.CompanyAccountID) {
			return &api.Account{
				Currency:  account.Currency,
				AccountID: account.AccountID,
				Balance:   decimal.NewFromInt(0),
			}, nil
		}

		return nil, fmt.Errorf("failed to get account balance for %s: %s: %w", account.AccountID, err.Error(), api.ErrAccountNotFound)
	}

	return account, nil
}

func (r *PostgresRepository) GetTransaction(ctx context.Context, txID string) (*api.Transaction, error) {
	txID = strings.TrimSpace(txID)
	if txID == "" {
		return nil, api.ErrInvalidTxID
	}

	row := r.db.QueryRowContext(ctx, selectTransaction, txID)

	tx := &api.Transaction{}

	err := row.Scan(&tx.TxID, &tx.AccountID, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to get transaction: %s: %w", err.Error(), api.ErrTransactionNotFound)
		}

		return nil, formatUnknownError(err)
	}

	return tx, nil
}

func (r *PostgresRepository) GetTransactions(ctx context.Context, currency, accountID string) ([]*api.Transaction, error) {
	err := validateCurrencyAndAccount(currency, accountID)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, selectTransactions, currency, accountID)
	if err != nil {
		return nil, formatUnknownError(err)
	}

	defer rows.Close()

	transactions := make([]*api.Transaction, 0, defaultTransactionSliceCapacity)

	for rows.Next() {
		tx := &api.Transaction{}

		err = rows.Scan(&tx.TxID, &tx.AccountID, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
		if err != nil {
			return nil, formatUnknownError(err)
		}

		transactions = append(transactions, tx)
	}

	if err = rows.Err(); err != nil {
		return nil, formatUnknownError(err)
	}

	return transactions, nil
}

type account struct {
	id      string
	balance decimal.Decimal
}

type accountPairBalance struct {
	from account
	to   account
}

func (r *PostgresRepository) Transfer(ctx context.Context, request *api.TransferRequest, idempotencyKey string) ([]*api.Transaction, error) {
	request.Currency = strings.ToUpper(strings.TrimSpace(request.Currency))
	request.FromAccountID = strings.TrimSpace(request.FromAccountID)
	request.ToAccountID = strings.TrimSpace(request.ToAccountID)

	var err error

	if err = validateCurrencyAndAccount(request.Currency, request.FromAccountID); err != nil {
		return nil, err
	}

	if err = validateCurrencyAndAccount(request.Currency, request.ToAccountID); err != nil {
		return nil, err
	}

	if strings.EqualFold(request.FromAccountID, request.ToAccountID) {
		return nil, api.ErrSameAccountIDs
	}

	if request.Amount.IsZero() {
		return nil, api.ErrInvalidAmount
	}

	if request.Amount.IsNegative() {
		return nil, api.ErrNegativeAmount
	}

	// check if the tx already exists
	var existingCount int
	if err = r.db.QueryRowContext(ctx, selectGroupExists, idempotencyKey).Scan(&existingCount); err != nil {
		return nil, formatUnknownError(err)
	}

	if existingCount > 0 {
		return nil, api.ErrDuplicateTransaction
	}

	if err = upsertAccounts(ctx, r.db, request); err != nil {
		return nil, err
	}

	// start of the transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, formatUnknownError(err)
	}

	accountBalances, err := lockAccounts(ctx, tx, request)
	if err != nil {
		_ = tx.Rollback()

		return nil, err
	}

	newTxIDFromTransfer, newTxIDToTransfer, err := createDoubleEntry(ctx, tx, request, accountBalances.from.id, accountBalances.to.id, idempotencyKey)
	if err != nil {
		_ = tx.Rollback()

		return nil, err
	}

	if err = updateBalances(ctx, tx, request); err != nil {
		_ = tx.Rollback()

		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, formatUnknownError(err)
	}

	txs, err := r.getTransactionsByIDs(ctx, newTxIDFromTransfer, newTxIDToTransfer)
	if err != nil {
		return nil, err
	}

	return txs, nil
}

func validateCurrencyAndAccount(currency, accountID string) error {
	if currency == "" || len(currency) > 10 {
		return api.ErrInvalidCurrency
	}

	if accountID == "" || len(accountID) > 255 {
		return api.ErrInvalidAccountID
	}

	return nil
}

func formatUnknownError(err error) error {
	return fmt.Errorf("%w: %w", api.ErrUnhandledDatabaseError, err)
}

// Create new double entry transactions of from and to accounts, respectively.
func createDoubleEntry(ctx context.Context, tx *sql.Tx, request *api.TransferRequest, fromAccountDatabaseID, toAccountDatabaseID, idempotencyKey string) (string, string, error) {
	// Prepare the reusable statement for optimized performance of repeated queries.
	newTxStatement, err := tx.PrepareContext(ctx, insertStatement)
	if err != nil {
		return "", "", formatUnknownError(err)
	}

	defer newTxStatement.Close()

	var newIDFromAccount string

	err = newTxStatement.QueryRowContext(ctx, fromAccountDatabaseID, request.Amount, api.DEBIT, request.Remarks, idempotencyKey).Scan(&newIDFromAccount)
	if err != nil {
		return "", "", formatUnknownError(err)
	}

	var newIDToAccount string

	err = newTxStatement.QueryRowContext(ctx, toAccountDatabaseID, request.Amount, api.CREDIT, request.Remarks, idempotencyKey).Scan(&newIDToAccount)
	if err != nil {
		return "", "", formatUnknownError(err)
	}

	return newIDFromAccount, newIDToAccount, nil
}

// Updates balances for both sides of the account. if it results in negative balance, return api.ErrInsufficientBalance
func updateBalances(ctx context.Context, tx *sql.Tx, request *api.TransferRequest) error {
	// Prepare the reusable statement for optimized performance of repeated queries.
	updateBalanceStatement, err := tx.PrepareContext(ctx, updateAccountBalance)
	if err != nil {
		return formatUnknownError(err)
	}

	defer updateBalanceStatement.Close()

	var newSourceBalance decimal.Decimal
	if err = updateBalanceStatement.QueryRowContext(ctx, request.Amount.Neg(), request.FromAccountID, request.Currency).Scan(&newSourceBalance); err != nil {
		return formatUnknownError(err)
	}

	// do the check again to ensure the balance didn't go negative
	allowNegative := strings.EqualFold(request.FromAccountID, api.CompanyAccountID)
	if newSourceBalance.IsNegative() && !allowNegative {
		return api.ErrInsufficientBalance // and then rollback
	}

	_, err = updateBalanceStatement.ExecContext(ctx, request.Amount, request.ToAccountID, request.Currency)
	if err != nil {
		return formatUnknownError(err)
	}

	return nil
}

// Pessimistic lock of both accounts. Returns the accountPairBalance{}.
// also returns api.ErrInsufficientBalance if the source account can't cover requested amount.
func lockAccounts(ctx context.Context, tx *sql.Tx, request *api.TransferRequest) (*accountPairBalance, error) {
	// Prepare the reusable statement for optimized performance of repeated queries.
	lockStatement, err := tx.PrepareContext(ctx, selectLockAccount)
	if err != nil {
		return nil, formatUnknownError(err)
	}

	defer lockStatement.Close()

	from := account{}
	if err = lockStatement.QueryRowContext(ctx, request.FromAccountID, request.Currency).Scan(&from.id, &from.balance); err != nil {
		return nil, formatUnknownError(err)
	}

	// we're doing the checks here as it's unnecessary to return and process elsewhere
	allowNegative := strings.EqualFold(request.FromAccountID, api.CompanyAccountID)
	if from.balance.LessThan(request.Amount) && !allowNegative {
		return nil, api.ErrInsufficientBalance
	}

	to := account{}
	if err = lockStatement.QueryRowContext(ctx, request.ToAccountID, request.Currency).Scan(&to.id, &to.balance); err != nil {
		return nil, formatUnknownError(err)
	}

	accountBalances := &accountPairBalance{
		from: from,
		to:   to,
	}

	return accountBalances, nil
}

// Upsert ensures the account exists before we lock them.
func upsertAccounts(ctx context.Context, db *sql.DB, request *api.TransferRequest) error {
	// Prepare the reusable statement for optimized performance.
	upsertAccountStatement, err := db.PrepareContext(ctx, upsertAccount)
	if err != nil {
		return formatUnknownError(err)
	}

	defer upsertAccountStatement.Close()

	_, err = upsertAccountStatement.ExecContext(ctx, request.FromAccountID, request.Currency)
	if err != nil {
		return formatUnknownError(err)
	}

	_, err = upsertAccountStatement.ExecContext(ctx, request.ToAccountID, request.Currency)
	if err != nil {
		return formatUnknownError(err)
	}

	return nil
}

func (r *PostgresRepository) getTransactionsByIDs(ctx context.Context, txID1, txID2 string) ([]*api.Transaction, error) {
	transactions, err := r.db.QueryContext(ctx, selectTransactionPair, txID1, txID2)
	if err != nil {
		return nil, formatUnknownError(err)
	}

	defer transactions.Close()

	// there are exactly 2 entries produced by every transfer
	txs := make([]*api.Transaction, 0, transactionPairCapacity)

	for transactions.Next() {
		var tx api.Transaction

		err = transactions.Scan(&tx.TxID, &tx.AccountID, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
		if err != nil {
			return nil, formatUnknownError(err)
		}

		txs = append(txs, &tx)
	}

	if err = transactions.Err(); err != nil {
		return nil, formatUnknownError(err)
	}

	return txs, nil
}
