package repository

import (
	"context"

	"github.com/devshark/wallet/api"
)

type Repository interface {
	// Deposit(ctx context.Context, request api.DepositRequest) (api.Transaction, error)
	// Withdraw(ctx context.Context, request api.WithdrawRequest) (api.Transaction, error)
	Transfer(ctx context.Context, request *api.TransferRequest, idempotencyKey string) ([]*api.Transaction, error)
	GetAccountBalance(ctx context.Context, currency, accountId string) (*api.Account, error)
	GetTransaction(ctx context.Context, txId string) (*api.Transaction, error)
	GetTransactions(ctx context.Context, currency, accountId string) ([]*api.Transaction, error)
}
