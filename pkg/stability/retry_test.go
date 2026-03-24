package stability

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

var errTest = errors.New("test error")

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	var calls int32
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, WaitTime: time.Millisecond}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestRetry_SuccessOnSecondAttempt(t *testing.T) {
	var calls int32
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, WaitTime: time.Millisecond}, func(ctx context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			return errTest
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestRetry_AllAttemptsFail(t *testing.T) {
	var calls int32
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, WaitTime: time.Millisecond}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errTest
	})
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}
}

func TestRetry_ReturnsLastError(t *testing.T) {
	attempt := 0
	errs := []error{
		errors.New("error 1"),
		errors.New("error 2"),
		errors.New("error 3"),
	}
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, WaitTime: time.Millisecond}, func(ctx context.Context) error {
		e := errs[attempt]
		attempt++
		return e
	})
	if err == nil || err.Error() != "error 3" {
		t.Fatalf("expected 'error 3', got %v", err)
	}
}

func TestRetry_ContextCancelledDuringWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32

	go func() {
		// Cancel after a short delay to allow the first attempt to fail
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := Retry(ctx, RetryConfig{MaxAttempts: 5, WaitTime: 5 * time.Second}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errTest
	})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Should return quickly, not wait the full 5 seconds
	if elapsed > 2*time.Second {
		t.Fatalf("retry did not respect context cancellation, took %v", elapsed)
	}
	if got := atomic.LoadInt32(&calls); got < 1 {
		t.Fatalf("expected at least 1 call, got %d", got)
	}
}

func TestRetry_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var calls int32
	err := Retry(ctx, RetryConfig{MaxAttempts: 3, WaitTime: time.Second}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errTest
	})

	// The first call still happens (fn is called), but after it fails,
	// context is already cancelled so we return ctx.Err()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRetry_ContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	var calls int32
	err := Retry(ctx, RetryConfig{MaxAttempts: 100, WaitTime: time.Second}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errTest
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestRetry_MaxAttemptsZeroDefaultsToOne(t *testing.T) {
	var calls int32
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 0, WaitTime: time.Millisecond}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errTest
	})
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestRetry_SingleAttemptSuccess(t *testing.T) {
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 1}, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRetry_SingleAttemptFailure(t *testing.T) {
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 1}, func(ctx context.Context) error {
		return errTest
	})
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
}

func TestRetry_CustomBackoffFunc(t *testing.T) {
	var waits []int
	customBackoff := func(attempt int) time.Duration {
		waits = append(waits, attempt)
		return time.Millisecond // fast for testing
	}

	var calls int32
	err := Retry(context.Background(), RetryConfig{
		MaxAttempts: 4,
		BackoffFunc: customBackoff,
	}, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errTest
	})

	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("expected 4 calls, got %d", got)
	}
	// BackoffFunc is called for attempts 1, 2, 3 (not after the last attempt)
	if len(waits) != 3 {
		t.Fatalf("expected 3 backoff calls, got %d: %v", len(waits), waits)
	}
	for i, w := range waits {
		if w != i+1 {
			t.Fatalf("expected backoff attempt %d, got %d", i+1, w)
		}
	}
}

func TestDefaultBackoff_ExponentialGrowth(t *testing.T) {
	backoff := DefaultBackoff(100 * time.Millisecond)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},  // 100ms * 2^0
		{2, 200 * time.Millisecond},  // 100ms * 2^1
		{3, 400 * time.Millisecond},  // 100ms * 2^2
		{4, 800 * time.Millisecond},  // 100ms * 2^3
		{5, 1600 * time.Millisecond}, // 100ms * 2^4
	}

	for _, tt := range tests {
		got := backoff(tt.attempt)
		if got != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, got)
		}
	}
}

func TestDefaultBackoff_LargeAttemptDoesNotOverflow(t *testing.T) {
	backoff := DefaultBackoff(time.Second)
	// Should not panic or produce negative values
	result := backoff(100)
	if result < 0 {
		t.Fatalf("backoff produced negative duration for large attempt: %v", result)
	}
}
