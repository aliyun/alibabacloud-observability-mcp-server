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

// TestProperty_RetryExecutionCount verifies that for any always-failing operation
// and retry configuration MaxAttempts=N, the Retry function executes the operation
// exactly N times and returns the last error.
//
// Feature: go-mcp-server-rewrite, Property 13: 重试执行次数
// Validates: Requirements 12.3
func TestProperty_RetryExecutionCount(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator: MaxAttempts in [1, 20]
	genMaxAttempts := gen.IntRange(1, 20)

	properties.Property("always-failing operation is executed exactly MaxAttempts times", prop.ForAll(
		func(maxAttempts int) bool {
			var callCount int64
			expectedErr := errors.New("always fails")

			cfg := RetryConfig{
				MaxAttempts: maxAttempts,
				BackoffFunc: func(attempt int) time.Duration { return 0 }, // no wait for fast tests
			}

			fn := func(ctx context.Context) error {
				atomic.AddInt64(&callCount, 1)
				return expectedErr
			}

			err := Retry(context.Background(), cfg, fn)

			count := atomic.LoadInt64(&callCount)
			if count != int64(maxAttempts) {
				t.Logf("Expected %d calls, got %d", maxAttempts, count)
				return false
			}

			// The returned error should be the last error from fn
			if !errors.Is(err, expectedErr) {
				t.Logf("Expected error %v, got %v", expectedErr, err)
				return false
			}

			return true
		},
		genMaxAttempts,
	))

	properties.Property("operation that succeeds on attempt K is called exactly K times", prop.ForAll(
		func(maxAttempts, succeedOn int) bool {
			var callCount int64

			cfg := RetryConfig{
				MaxAttempts: maxAttempts,
				BackoffFunc: func(attempt int) time.Duration { return 0 },
			}

			fn := func(ctx context.Context) error {
				n := atomic.AddInt64(&callCount, 1)
				if int(n) >= succeedOn {
					return nil
				}
				return errors.New("not yet")
			}

			err := Retry(context.Background(), cfg, fn)

			count := atomic.LoadInt64(&callCount)

			if err != nil {
				// Should only fail if succeedOn > maxAttempts
				if succeedOn <= maxAttempts {
					t.Logf("Expected success at attempt %d (max=%d), but got error: %v", succeedOn, maxAttempts, err)
					return false
				}
				// All attempts exhausted, should have called maxAttempts times
				return count == int64(maxAttempts)
			}

			// Succeeded: should have called exactly succeedOn times
			if count != int64(succeedOn) {
				t.Logf("Expected %d calls to succeed, got %d", succeedOn, count)
				return false
			}
			return true
		},
		genMaxAttempts,
		gen.IntRange(1, 20).WithLabel("succeedOnAttempt"),
	))

	properties.TestingRun(t)
}

// TestProperty_CircuitBreakerStateTransition verifies that for any circuit breaker
// configuration maxFailures=N, after N consecutive failures the breaker enters
// the Open state, and subsequent calls return ErrCircuitOpen immediately without
// executing the actual operation.
//
// Feature: go-mcp-server-rewrite, Property 14: 熔断器状态转换
// Validates: Requirements 12.4
func TestProperty_CircuitBreakerStateTransition(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator: maxFailures in [1, 20]
	genMaxFailures := gen.IntRange(1, 20)

	properties.Property("after N consecutive failures, circuit breaker enters Open state and rejects calls", prop.ForAll(
		func(maxFailures int) bool {
			cb := NewCircuitBreaker("test", maxFailures, time.Hour) // long reset timeout so it stays open
			ctx := context.Background()
			failErr := errors.New("operation failed")

			var actualCalls int64

			failFn := func(ctx context.Context) error {
				atomic.AddInt64(&actualCalls, 1)
				return failErr
			}

			// Execute N failing calls to trip the breaker
			for i := 0; i < maxFailures; i++ {
				err := cb.Execute(ctx, failFn)
				if err == nil {
					t.Logf("Expected failure on call %d, got nil", i+1)
					return false
				}
				if errors.Is(err, ErrCircuitOpen) {
					t.Logf("Circuit opened too early on call %d (maxFailures=%d)", i+1, maxFailures)
					return false
				}
			}

			// All N calls should have actually executed the function
			if atomic.LoadInt64(&actualCalls) != int64(maxFailures) {
				t.Logf("Expected %d actual calls, got %d", maxFailures, atomic.LoadInt64(&actualCalls))
				return false
			}

			// Verify the breaker is now Open
			if cb.State() != StateOpen {
				t.Logf("Expected Open state after %d failures, got %s", maxFailures, cb.State())
				return false
			}

			// Subsequent calls should be rejected without executing fn
			callsBefore := atomic.LoadInt64(&actualCalls)
			for i := 0; i < 3; i++ {
				err := cb.Execute(ctx, failFn)
				if !errors.Is(err, ErrCircuitOpen) {
					t.Logf("Expected ErrCircuitOpen after breaker opened, got %v", err)
					return false
				}
			}

			// fn should NOT have been called during the rejected calls
			if atomic.LoadInt64(&actualCalls) != callsBefore {
				t.Logf("fn was called after circuit opened: before=%d, after=%d", callsBefore, atomic.LoadInt64(&actualCalls))
				return false
			}

			return true
		},
		genMaxFailures,
	))

	properties.Property("a success resets the failure counter", prop.ForAll(
		func(maxFailures int) bool {
			cb := NewCircuitBreaker("test-reset", maxFailures, time.Hour)
			ctx := context.Background()
			failErr := errors.New("fail")

			failFn := func(ctx context.Context) error { return failErr }
			successFn := func(ctx context.Context) error { return nil }

			// Accumulate N-1 failures (one short of tripping)
			for i := 0; i < maxFailures-1; i++ {
				_ = cb.Execute(ctx, failFn)
			}

			// A success should reset the counter
			if err := cb.Execute(ctx, successFn); err != nil {
				t.Logf("Expected success, got %v", err)
				return false
			}

			// State should be Closed
			if cb.State() != StateClosed {
				t.Logf("Expected Closed state after success, got %s", cb.State())
				return false
			}

			// Now N-1 more failures should NOT trip the breaker
			for i := 0; i < maxFailures-1; i++ {
				_ = cb.Execute(ctx, failFn)
			}

			if cb.State() != StateClosed {
				t.Logf("Expected Closed state after N-1 failures post-reset, got %s", cb.State())
				return false
			}

			return true
		},
		genMaxFailures,
	))

	properties.TestingRun(t)
}
