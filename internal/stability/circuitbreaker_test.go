package stability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewCircuitBreaker_Defaults(t *testing.T) {
	cb := NewCircuitBreaker("test", 0, 0)
	if cb.maxFailures != 5 {
		t.Errorf("expected default maxFailures=5, got %d", cb.maxFailures)
	}
	if cb.resetTimeout != 30*time.Second {
		t.Errorf("expected default resetTimeout=30s, got %v", cb.resetTimeout)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected initial state Closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_ClosedState_Success(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, time.Second)
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected Closed state after success, got %s", cb.State())
	}
}

func TestCircuitBreaker_ClosedState_FailuresBelowThreshold(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, time.Second)
	// 2 failures (below threshold of 3) should keep it Closed
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errTest
		})
	}
	if cb.State() != StateClosed {
		t.Errorf("expected Closed state with %d failures, got %s", 2, cb.State())
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, time.Second)
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error {
			return errTest
		})
		if !errors.Is(err, errTest) {
			t.Fatalf("attempt %d: expected errTest, got %v", i+1, err)
		}
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected Open state after 3 failures, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpenState_RejectsImmediately(t *testing.T) {
	cb := NewCircuitBreaker("test", 1, time.Hour)
	// Trip the breaker
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return errTest
	})

	called := false
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
	if called {
		t.Error("fn should not have been called when circuit is open")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, time.Second)
	// 2 failures
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errTest
		})
	}
	// 1 success should reset the counter
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	// 2 more failures should NOT open the breaker (counter was reset)
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func(ctx context.Context) error {
			return errTest
		})
	}
	if cb.State() != StateClosed {
		t.Errorf("expected Closed (counter should have reset), got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_TrialSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)
	// Trip the breaker
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return errTest
	})
	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %s", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(20 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen after timeout, got %s", cb.State())
	}

	// Trial call succeeds → should go back to Closed
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error on trial success, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected Closed after successful trial, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_TrialFailure(t *testing.T) {
	cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)
	// Trip the breaker
	_ = cb.Execute(context.Background(), func(ctx context.Context) error {
		return errTest
	})

	// Wait for reset timeout
	time.Sleep(20 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen, got %s", cb.State())
	}

	// Trial call fails → should go back to Open
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return errTest
	})
	if !errors.Is(err, errTest) {
		t.Fatalf("expected errTest, got %v", err)
	}
	if cb.State() != StateOpen {
		t.Errorf("expected Open after failed trial, got %s", cb.State())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown(99)"},
	}
	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", int(tt.state), got, tt.want)
		}
	}
}
