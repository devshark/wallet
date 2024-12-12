package repository

import (
	"context"
	"database/sql"
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
	row := r.db.QueryRowContext(ctx, `
		SELECT balance
		FROM transactions
		WHERE currency = $1 AND account_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, account.Currency, account.AccountId)

	err := row.Scan(&account.Balance)
	if err != nil {
		// if the account is the company account, the initial balance must be 0
		if err == sql.ErrNoRows && strings.EqualFold(account.AccountId, api.COMPANY_ACCOUNT_ID) {
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

	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, currency, amount, debit_credit, balance, description, created_at 
		FROM transactions WHERE id = $1
	`, txId)

	tx := &api.Transaction{}
	err := row.Scan(&tx.TxID, &tx.AccountId, &tx.Currency, &tx.Amount, &tx.Type, &tx.RunningBalance, &tx.Remarks, &tx.Time)
	if err != nil {
		if err == sql.ErrNoRows {
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

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, currency, amount, debit_credit, balance, description, created_at 
		FROM transactions WHERE currency = $1 AND account_id = $2
		ORDER BY created_at DESC
	`, currency, accountId)
	if err != nil {
		if err == sql.ErrNoRows {
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
			if err == sql.ErrNoRows {
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

	allowNegative := strings.EqualFold(request.FromAccountId, api.COMPANY_ACCOUNT_ID)

	existingTx := r.db.QueryRowContext(ctx, `SELECT count(1) FROM transactions WHERE group_id = $1`, idempotencyKey)
	var existingCount int
	err := existingTx.Scan(&existingCount)
	if err != nil {
		return nil, err
	}

	if existingCount > 0 {
		return nil, api.ErrDuplicateTransaction
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}

	// lock the row for update
	fromRow := tx.QueryRowContext(ctx, `
		SELECT balance
		FROM transactions
		WHERE currency = $1 AND account_id = $2
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, request.Currency, request.FromAccountId)

	var fromBalance decimal.Decimal
	err = fromRow.Scan(&fromBalance)
	if err != nil {
		// COMPANY account can have no beginning balance
		if !allowNegative {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to get account balance of %s: %v: %w", request.FromAccountId, err, api.ErrAccountNotFound)
		}
	}

	if fromBalance.LessThan(request.Amount) && !allowNegative {
		_ = tx.Rollback()
		return nil, api.ErrInsufficientBalance
	}

	// lock the row for update
	// ignore error if destination doesn't exist yet
	toRow := tx.QueryRowContext(ctx, `
		SELECT balance
		FROM transactions
		WHERE currency = $1 AND account_id = $2
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, request.Currency, request.ToAccountId)

	var toBalance decimal.Decimal
	err = toRow.Scan(&toBalance)
	if err != nil {
		r.logger.Printf("Account %s does not exist yet, initializing the account\n", request.ToAccountId)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO transactions (account_id, currency, amount, debit_credit, balance, description, group_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, request.FromAccountId, request.Currency, request.Amount, "DEBIT", fromBalance.Sub(request.Amount), request.Remarks, idempotencyKey)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO transactions (account_id, currency, amount, debit_credit, balance, description, group_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, request.ToAccountId, request.Currency, request.Amount, "CREDIT", toBalance.Add(request.Amount), request.Remarks, idempotencyKey)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	transactions, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, currency, amount, debit_credit, balance, description, created_at 
		FROM transactions WHERE group_id = $1
	`, idempotencyKey)
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
