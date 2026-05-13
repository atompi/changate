package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	ctx := context.Background()
	err := Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestDo_TransientErrors_RetriesSuccess(t *testing.T) {
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return ErrTransient
		}
		return nil
	}

	ctx := context.Background()
	err := Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestDo_NonTransientError_ReturnsImmediately(t *testing.T) {
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("non-transient error")
	}

	ctx := context.Background()
	err := Do(ctx, cfg, fn)
	if err == nil {
		t.Fatal("Do() want error, got nil")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return ErrTransient
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, cfg, fn)
	if err != context.Canceled {
		t.Fatalf("Do() error = %v, want %v", err, context.Canceled)
	}
}

func TestDo_AllRetriesFail(t *testing.T) {
	cfg := Config{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return ErrTransient
	}

	err := Do(context.Background(), cfg, fn)
	if err == nil {
		t.Fatal("Do() want error, got nil")
	}
	if callCount != cfg.MaxRetries+1 {
		t.Errorf("callCount = %d, want %d (initial + %d retries)", callCount, cfg.MaxRetries+1, cfg.MaxRetries)
	}
}

func TestDo_WithBeforeRetryHook(t *testing.T) {
	cfg := Config{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		BeforeRetry: func(attempt int, delay time.Duration) {
			t.Logf("before retry: attempt=%d delay=%v", attempt, delay)
		},
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return ErrTransient
		}
		return nil
	}

	ctx := context.Background()
	err := Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
}

func TestIsTransient(t *testing.T) {
	if !isTransient(ErrTransient) {
		t.Error("isTransient(ErrTransient) = false, want true")
	}

	if isTransient(errors.New("other error")) {
		t.Error("isTransient(other error) = true, want false")
	}

	if isTransient(nil) {
		t.Error("isTransient(nil) = true, want false")
	}
}
