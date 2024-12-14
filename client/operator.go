package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/devshark/wallet/api"
)

// AccountOperatorClient implements the AccountOperator interface.
type AccountOperatorClient struct {
	baseURL    string
	httpClient *http.Client
	clientName string
}

// NewAccountOperatorClient creates a new AccountOperatorClient.
func NewAccountOperatorClient(baseURL string) *AccountOperatorClient {
	return &AccountOperatorClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		clientName: "AccountOperatorClient",
	}
}

func (c *AccountOperatorClient) WithName(name string) *AccountOperatorClient {
	c.clientName = name

	return c
}

// Deposit performs a deposit operation.
func (c *AccountOperatorClient) Deposit(ctx context.Context, request *api.DepositRequest, idempotencyKey string) (*api.Transaction, error) {
	url := fmt.Sprintf("%s/deposit", c.baseURL)

	return c.postAndDecode(ctx, url, request, idempotencyKey)
}

// Withdraw performs a withdrawal operation.
func (c *AccountOperatorClient) Withdraw(ctx context.Context, request *api.WithdrawRequest, idempotencyKey string) (*api.Transaction, error) {
	url := fmt.Sprintf("%s/withdraw", c.baseURL)

	return c.postAndDecode(ctx, url, request, idempotencyKey)
}

// Transfer performs a transfer operation.
func (c *AccountOperatorClient) Transfer(ctx context.Context, request *api.TransferRequest, idempotencyKey string) (*api.Transaction, error) {
	url := fmt.Sprintf("%s/transfer", c.baseURL)

	return c.postAndDecode(ctx, url, request, idempotencyKey)
}

// postAndDecode performs a POST request and decodes the response into a Transaction.
func (c *AccountOperatorClient) postAndDecode(ctx context.Context, url string, payload interface{}, idempotencyKey string) (*api.Transaction, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", idempotencyKey)
	req.Header.Set("Client-Name", c.clientName)
	req.Header.Set("User-Agent", c.clientName)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", api.ErrUnexpected, resp.StatusCode)
	}

	var transaction api.Transaction
	if err := json.NewDecoder(resp.Body).Decode(&transaction); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &transaction, nil
}
