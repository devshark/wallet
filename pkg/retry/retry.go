package retry

import (
	"context"
	"time"
)

const DefaultMaxAttempts = 5
const BackoffCoefficient = 2

// retry retries a function with exponential backoff up to a maximum number of attempts
func Retry(ctx context.Context, f func() error, maxAttempts int) error {
	backoff := 100 * time.Millisecond

	attempt := 0
	for {
		err := f()
		if err == nil {
			return nil
		}

		attempt++
		if attempt >= maxAttempts {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= BackoffCoefficient
		}
	}
}
