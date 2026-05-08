package retry

import (
	"context"
	"errors"
	"time"
)

type Config struct {
	MaxRetries   int
	BaseDelay    time.Duration
	BeforeRetry func(attempt int, delay time.Duration)
}

var ErrTransient = errors.New("transient error")

func isTransient(err error) bool {
	return errors.Is(err, ErrTransient)
}

func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := cfg.BaseDelay * time.Duration(1<<(attempt-1))
			if cfg.BeforeRetry != nil {
				cfg.BeforeRetry(attempt, delay)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := fn(); err != nil {
			lastErr = err
			if !isTransient(err) {
				return err
			}
			continue
		}
		return nil
	}
	return lastErr
}