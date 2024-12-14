package repository

import (
	"context"

	"github.com/devshark/wallet/api"
)

type Repository interface {
	Transfer(ctx context.Context, request *api.TransferRequest, idempotencyKey string) ([]*api.Transaction, error)
	GetAccountBalance(ctx context.Context, currency, accountID string) (*api.Account, error)
	GetTransaction(ctx context.Context, txID string) (*api.Transaction, error)
	GetTransactions(ctx context.Context, currency, accountID string) ([]*api.Transaction, error)
}
