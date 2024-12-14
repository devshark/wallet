package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/devshark/wallet/api"
)

// AccountReaderClient implements the AccountReader interface.
type AccountReaderClient struct {
	baseURL    string
	httpClient *http.Client
	clientName string
}

// NewAccountReaderClient creates a new AccountReaderClient.
func NewAccountReaderClient(baseURL string) *AccountReaderClient {
	return &AccountReaderClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		clientName: "AccountOperatorClient",
	}
}

// GetAccountBalance retrieves the account balance for a given currency and account ID.
func (c *AccountReaderClient) GetAccountBalance(ctx context.Context, currency, accountID string) (*api.Account, error) {
	url := fmt.Sprintf("%s/account/%s/%s", c.baseURL, accountID, currency)
	account := &api.Account{}
	err := c.getAndDecode(ctx, url, account)

	return account, err
}

// GetTransaction retrieves a transaction by its ID.
func (c *AccountReaderClient) GetTransaction(ctx context.Context, txID string) (*api.Transaction, error) {
	url := fmt.Sprintf("%s/transactions/%s", c.baseURL, txID)
	transaction := &api.Transaction{}
	err := c.getAndDecode(ctx, url, transaction)

	return transaction, err
}

// GetTransactions retrieves all transactions for a given currency and account ID.
func (c *AccountReaderClient) GetTransactions(ctx context.Context, currency, accountID string) ([]*api.Transaction, error) {
	url := fmt.Sprintf("%s/transactions/%s/%s", c.baseURL, accountID, currency)

	var transactions []*api.Transaction

	err := c.getAndDecodeSlice(ctx, url, &transactions)

	return transactions, err
}

// getAndDecode performs a GET request and decodes the response into the provided interface.
func (c *AccountReaderClient) getAndDecode(ctx context.Context, url string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-Name", c.clientName)
	req.Header.Set("User-Agent", c.clientName)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", api.ErrUnexpected, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// getAndDecodeSlice performs a GET request and decodes the response into the provided slice.
func (c *AccountReaderClient) getAndDecodeSlice(ctx context.Context, url string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-Name", c.clientName)
	req.Header.Set("User-Agent", c.clientName)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", api.ErrUnexpected, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
