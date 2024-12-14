package api

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
)

var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrAccountNotFound  = errors.New("account not found")
	ErrInvalidAmount    = errors.New("invalid amount")
	ErrInvalidCurrency  = errors.New("invalid currency")
	ErrInvalidAccountID = errors.New("invalid account id")

	ErrNegativeAmount = errors.New("negative amount")
	ErrSameAccountIDs = errors.New("same account ids")

	ErrInvalidTxID          = errors.New("invalid tx id")
	ErrInvalidAccount       = errors.New("invalid account")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrDuplicateTransaction = errors.New("duplicate transaction")

	ErrCompanyAccount = errors.New("cannot use company account")

	ErrMissingIdempotencyKey = errors.New("missing idempotency key")

	ErrTransferFailed         = errors.New("transfer failed")
	ErrFailedToGetTransaction = errors.New("failed to get transaction")
	ErrIncompleteTransaction  = errors.New("transaction did not complete")

	ErrUnexpected = errors.New("unexpected error")

	ErrUnhandledDatabaseError = errors.New("unhandled database error")
)

type DebitOrCreditType string

const (
	DEBIT  DebitOrCreditType = "DEBIT"
	CREDIT DebitOrCreditType = "CREDIT"
)

const (
	CompanyAccountID = "company"
)

type Account struct {
	AccountID string          `json:"account"`
	Currency  string          `json:"currency"`
	Balance   decimal.Decimal `json:"balance"`
}

type Transaction struct {
	TxID           string            `json:"tx_id"`
	AccountID      string            `json:"account_id"`
	Type           DebitOrCreditType `json:"type"`
	Amount         decimal.Decimal   `json:"amount"`
	Currency       string            `json:"currency"`
	RunningBalance decimal.Decimal   `json:"running_balance"`
	Remarks        string            `json:"remarks"`
	Time           string            `json:"time"`
}

type TransferRequest struct {
	FromAccountID string          `json:"from_account_id"`
	ToAccountID   string          `json:"to_account_id"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Remarks       string          `json:"remarks,omitempty"`
}

type DepositRequest struct {
	Currency    string          `json:"currency"`
	ToAccountID string          `json:"account_id"`
	Amount      decimal.Decimal `json:"amount"`
	Remarks     string          `json:"remarks,omitempty"`
}

type WithdrawRequest struct {
	Currency      string          `json:"currency"`
	FromAccountID string          `json:"account_id"`
	Amount        decimal.Decimal `json:"amount"`
	Remarks       string          `json:"remarks,omitempty"`
}

type ErrorResponse struct {
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
}

type AccountReader interface {
	GetAccountBalance(ctx context.Context, currency, accountID string) (Account, error)
	GetTransaction(ctx context.Context, txID string) (Transaction, error)
	GetTransactions(ctx context.Context, currency, accountID string) ([]Transaction, error)
}

type AccountOperator interface {
	Deposit(ctx context.Context, request DepositRequest) (Transaction, error)
	Withdraw(ctx context.Context, request WithdrawRequest) (Transaction, error)
	Transfer(ctx context.Context, request TransferRequest) (Transaction, error)
}

func OppositeType(t DebitOrCreditType) DebitOrCreditType {
	if t == CREDIT {
		return DEBIT
	}

	return CREDIT
}
