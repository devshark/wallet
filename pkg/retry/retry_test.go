package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devshark/wallet/pkg/retry"
)

func TestRetry(t *testing.T) {
	t.Run("Successful on first attempt", func(t *testing.T) {
		attempts := 0
		err := retry.Retry(context.Background(), func() error {
			attempts++
			return nil
		}, retry.DefaultMaxAttempts)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Successful after multiple attempts", func(t *testing.T) {
		attempts := 0
		err := retry.Retry(context.Background(), func() error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		}, retry.DefaultMaxAttempts)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Failure after max attempts", func(t *testing.T) {
		attempts := 0
		err := retry.Retry(context.Background(), func() error {
			attempts++
			return errors.New("persistent error")
		}, retry.DefaultMaxAttempts)

		if err == nil {
			t.Error("Expected an error, got nil")
		}
		if attempts != retry.DefaultMaxAttempts {
			t.Errorf("Expected %d attempts, got %d", retry.DefaultMaxAttempts, attempts)
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := retry.Retry(ctx, func() error {
			attempts++
			time.Sleep(20 * time.Millisecond)
			return errors.New("temporary error")
		}, retry.DefaultMaxAttempts)

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
		if attempts == 0 {
			t.Error("Expected at least one attempt")
		}
		if attempts >= retry.DefaultMaxAttempts {
			t.Errorf("Expected less than %d attempts, got %d", retry.DefaultMaxAttempts, attempts)
		}
	})

	t.Run("Custom max attempts", func(t *testing.T) {
		customMaxAttempts := 3
		attempts := 0
		err := retry.Retry(context.Background(), func() error {
			attempts++
			return errors.New("persistent error")
		}, customMaxAttempts)

		if err == nil {
			t.Error("Expected an error, got nil")
		}
		if attempts != customMaxAttempts {
			t.Errorf("Expected %d attempts, got %d", customMaxAttempts, attempts)
		}
	})

	t.Run("Backoff increases", func(t *testing.T) {
		// t.Skip("slow test")
		attempts := 0
		start := time.Now()
		err := retry.Retry(context.Background(), func() error {
			attempts++
			return errors.New("persistent error")
		}, retry.DefaultMaxAttempts)

		duration := time.Since(start)

		if err == nil {
			t.Error("Expected an error, got nil")
		}
		if attempts != retry.DefaultMaxAttempts {
			t.Errorf("Expected %d attempts, got %d", retry.DefaultMaxAttempts, attempts)
		}

		// Check if the total duration is at least the sum of the backoff times
		// Initial attempt is immediate, then an initial backoff is 100ms, doubling each time
		expectedMinDuration := 100 + 200 + 400 + 800
		if duration < time.Duration(expectedMinDuration)*time.Millisecond {
			t.Errorf("Expected duration of at least %dms, got %v", expectedMinDuration, duration)
		}
	})
}
