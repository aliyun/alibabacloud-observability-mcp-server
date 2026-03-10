package stability

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_ContextCancellationPropagation verifies that for any operation,
// when the passed-in context is cancelled, the operation returns promptly with
// context.Canceled or context.DeadlineExceeded error.
//
// Feature: go-mcp-server-rewrite, Property 9: Context 取消传播
// **Validates: Requirements 11.4**
func TestProperty_ContextCancellationPropagation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: Retry returns a context error promptly when context is already cancelled
	properties.Property("Retry returns context error when context is pre-cancelled", prop.ForAll(
		func(maxAttempts int) bool {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately

			var callCount int64
			cfg := RetryConfig{
				MaxAttempts: maxAttempts,
				WaitTime:    time.Second, // large wait to prove we don't actually wait
				BackoffFunc: DefaultBackoff(time.Second),
			}

			fn := func(ctx context.Context) error {
				atomic.AddInt64(&callCount, 1)
				return errors.New("fail")
			}

			start := time.Now()
			err := Retry(ctx, cfg, fn)
			elapsed := time.Since(start)

			// Must return a context error
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				// The first call to fn may execute before the context check,
				// but the retry loop should stop after at most 1 call.
				// If maxAttempts == 1, fn runs once and returns its error (no wait).
				// For maxAttempts > 1, after the first fn call, the context check
				// should catch the cancellation.
				if maxAttempts > 1 {
					t.Logf("Expected context error for maxAttempts=%d, got: %v", maxAttempts, err)
					return false
				}
			}

			// Should complete quickly (well under 1 second, since we don't wait)
			if elapsed > 500*time.Millisecond {
				t.Logf("Retry took too long with cancelled context: %v", elapsed)
				return false
			}

			// Should not have retried many times
			count := atomic.LoadInt64(&callCount)
			if count > 1 {
				t.Logf("Expected at most 1 call with cancelled context, got %d", count)
				return false
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	// Property: Retry returns context error promptly when context is cancelled during wait
	properties.Property("Retry returns context error when cancelled during backoff wait", prop.ForAll(
		func(maxAttempts int) bool {
			if maxAttempts < 2 {
				return true // need at least 2 attempts to trigger a wait
			}

			ctx, cancel := context.WithCancel(context.Background())

			var callCount int64
			cfg := RetryConfig{
				MaxAttempts: maxAttempts,
				WaitTime:    10 * time.Second, // very long wait
				BackoffFunc: func(attempt int) time.Duration { return 10 * time.Second },
			}

			fn := func(ctx context.Context) error {
				n := atomic.AddInt64(&callCount, 1)
				if n == 1 {
					// Cancel context after first call, during the backoff wait
					go func() {
						time.Sleep(5 * time.Millisecond)
						cancel()
					}()
				}
				return errors.New("fail")
			}

			start := time.Now()
			err := Retry(ctx, cfg, fn)
			elapsed := time.Since(start)

			// Must return a context error
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Expected context error, got: %v", err)
				return false
			}

			// Should return quickly (not wait the full 10s backoff)
			if elapsed > 2*time.Second {
				t.Logf("Retry did not return promptly after context cancel: %v", elapsed)
				return false
			}

			// Should have called fn exactly once (cancelled during wait before 2nd call)
			count := atomic.LoadInt64(&callCount)
			if count != 1 {
				t.Logf("Expected 1 call, got %d", count)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	// Property: Retry with deadline returns context error when deadline expires
	properties.Property("Retry returns DeadlineExceeded when context deadline expires", prop.ForAll(
		func(maxAttempts int) bool {
			if maxAttempts < 2 {
				return true
			}

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()

			var callCount int64
			cfg := RetryConfig{
				MaxAttempts: maxAttempts,
				WaitTime:    10 * time.Second,
				BackoffFunc: func(attempt int) time.Duration { return 10 * time.Second },
			}

			fn := func(ctx context.Context) error {
				atomic.AddInt64(&callCount, 1)
				return errors.New("fail")
			}

			start := time.Now()
			err := Retry(ctx, cfg, fn)
			elapsed := time.Since(start)

			// Must return a context error
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Expected context error, got: %v", err)
				return false
			}

			// Should return promptly (not wait the full backoff)
			if elapsed > 2*time.Second {
				t.Logf("Retry did not return promptly after deadline: %v", elapsed)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	// Property: CircuitBreaker.Execute propagates context cancellation
	properties.Property("CircuitBreaker.Execute returns context error when context is cancelled", prop.ForAll(
		func(maxFailures int) bool {
			cb := NewCircuitBreaker("ctx-test", maxFailures, time.Hour)
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // pre-cancel

			fn := func(ctx context.Context) error {
				// Simulate an operation that checks context
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					return errors.New("should not reach here with cancelled context")
				}
			}

			err := cb.Execute(ctx, fn)

			// The fn should have received the cancelled context and returned its error
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Expected context error from CircuitBreaker, got: %v", err)
				return false
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	// Property: Full resilience stack (Retry + CircuitBreaker) propagates context cancellation
	properties.Property("Retry+CircuitBreaker stack returns context error when cancelled during backoff", prop.ForAll(
		func(maxAttempts, maxFailures int) bool {
			if maxAttempts < 2 {
				return true
			}

			cb := NewCircuitBreaker("stack-test", maxFailures, time.Hour)
			ctx, cancel := context.WithCancel(context.Background())

			var callCount int64
			cfg := RetryConfig{
				MaxAttempts: maxAttempts,
				WaitTime:    10 * time.Second,
				BackoffFunc: func(attempt int) time.Duration { return 10 * time.Second },
			}

			// Simulate the executeWithResilience pattern from client code
			wrappedFn := func(ctx context.Context) error {
				return cb.Execute(ctx, func(ctx context.Context) error {
					n := atomic.AddInt64(&callCount, 1)
					if n == 1 {
						go func() {
							time.Sleep(5 * time.Millisecond)
							cancel()
						}()
					}
					return errors.New("api error")
				})
			}

			start := time.Now()
			err := Retry(ctx, cfg, wrappedFn)
			elapsed := time.Since(start)

			// Must return a context error
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Logf("Expected context error from stack, got: %v", err)
				return false
			}

			// Should return promptly
			if elapsed > 2*time.Second {
				t.Logf("Stack did not return promptly: %v", elapsed)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
		gen.IntRange(5, 20), // high maxFailures so breaker stays closed
	))

	properties.TestingRun(t)
}
