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
	ErrInvalidAccountId = errors.New("invalid account id")

	ErrNegativeAmount = errors.New("negative amount")
	ErrSameAccountIds = errors.New("same account ids")

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
)

type DebitOrCreditType string

const (
	DEBIT  DebitOrCreditType = "DEBIT"
	CREDIT DebitOrCreditType = "CREDIT"
)

const (
	COMPANY_ACCOUNT_ID = "company"
)

type Account struct {
	AccountId string          `json:"account"`
	Currency  string          `json:"currency"`
	Balance   decimal.Decimal `json:"balance"`
}

type Transaction struct {
	TxID           string            `json:"tx_id"`
	AccountId      string            `json:"account_id"`
	Type           DebitOrCreditType `json:"type"`
	Amount         decimal.Decimal   `json:"amount"`
	Currency       string            `json:"currency"`
	RunningBalance decimal.Decimal   `json:"running_balance"`
	Remarks        string            `json:"remarks"`
	Time           string            `json:"time"`
}

type TransferRequest struct {
	FromAccountId string          `json:"from_account_id"`
	ToAccountId   string          `json:"to_account_id"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Remarks       string          `json:"remarks,omitempty"`
}

type DepositRequest struct {
	Currency    string          `json:"currency"`
	ToAccountId string          `json:"account_id"`
	Amount      decimal.Decimal `json:"amount"`
	Remarks     string          `json:"remarks,omitempty"`
}

type WithdrawRequest struct {
	Currency      string          `json:"currency"`
	FromAccountId string          `json:"account_id"`
	Amount        decimal.Decimal `json:"amount"`
	Remarks       string          `json:"remarks,omitempty"`
}

type ErrorResponse struct {
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
}

type AccountReader interface {
	GetAccountBalance(ctx context.Context, currency, accountId string) (Account, error)
	GetTransaction(ctx context.Context, txId string) (Transaction, error)
	GetTransactions(ctx context.Context, currency, accountId string) ([]Transaction, error)
}

type AccountOperator interface {
	Deposit(ctx context.Context, request DepositRequest) (Transaction, error)
	Withdraw(ctx context.Context, request WithdrawRequest) (Transaction, error)
	Transfer(ctx context.Context, request TransferRequest) (Transaction, error)
}
