package retry

import (
	"context"
	"fmt"
	"time"
)

const DefaultMaxAttempts = 5
const BackoffCoefficient = 2
const initialBackoff = 100 * time.Millisecond

// retry retries a function with exponential backoff up to a maximum number of attempts
func Retry(ctx context.Context, f func() error, maxAttempts int) error {
	backoff := initialBackoff
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
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context already expired: %w", ctx.Err())
			}

			return nil
		case <-time.After(backoff):
			backoff *= BackoffCoefficient
		}
	}
}
