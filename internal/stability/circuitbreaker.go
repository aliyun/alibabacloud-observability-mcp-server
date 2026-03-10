package stability

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is in the Open state
// and rejects calls immediately.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// State represents the state of a circuit breaker.
type State int

const (
	StateClosed   State = iota // Normal operation, counting failures
	StateOpen                  // Rejecting all calls
	StateHalfOpen              // Allowing one trial call
)

// String returns the string representation of a State.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// CircuitBreaker implements the circuit breaker pattern.
//
// Closed: normal operation; consecutive failures are counted.
// Open:   after maxFailures consecutive failures, all calls are rejected immediately.
// HalfOpen: after resetTimeout elapses, one trial call is allowed.
//   - If the trial succeeds → Closed
//   - If the trial fails   → Open
type CircuitBreaker struct {
	name         string
	maxFailures  int
	resetTimeout time.Duration

	mu          sync.Mutex
	state       State
	failures    int
	lastFailure time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker.
// name is used for identification in logs/errors.
// maxFailures is the number of consecutive failures before opening.
// resetTimeout is how long to wait in Open state before transitioning to HalfOpen.
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

// currentState returns the effective state, transitioning from Open to HalfOpen
// if the reset timeout has elapsed. Must be called with cb.mu held.
func (cb *CircuitBreaker) currentState() State {
	if cb.state == StateOpen && time.Since(cb.lastFailure) >= cb.resetTimeout {
		cb.state = StateHalfOpen
	}
	return cb.state
}

// Execute runs fn if the circuit breaker allows it.
//
// In Closed state, fn is executed. If it fails, the failure counter increments.
// When consecutive failures reach maxFailures, the breaker transitions to Open.
//
// In Open state, ErrCircuitOpen is returned immediately without calling fn.
//
// In HalfOpen state, fn is executed as a trial.
// If the trial succeeds, the breaker resets to Closed.
// If the trial fails, the breaker returns to Open.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	cb.mu.Lock()
	state := cb.currentState()

	switch state {
	case StateOpen:
		cb.mu.Unlock()
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow one trial call; stay in HalfOpen while it runs.
		cb.mu.Unlock()
		err := fn(ctx)
		cb.mu.Lock()
		if err == nil {
			cb.reset()
		} else {
			cb.tripOpen()
		}
		cb.mu.Unlock()
		return err

	default: // StateClosed
		cb.mu.Unlock()
		err := fn(ctx)
		cb.mu.Lock()
		if err == nil {
			cb.reset()
		} else {
			cb.failures++
			cb.lastFailure = time.Now()
			if cb.failures >= cb.maxFailures {
				cb.state = StateOpen
			}
		}
		cb.mu.Unlock()
		return err
	}
}

// reset moves the breaker back to Closed and clears the failure counter.
// Must be called with cb.mu held.
func (cb *CircuitBreaker) reset() {
	cb.state = StateClosed
	cb.failures = 0
}

// tripOpen moves the breaker to Open and records the failure time.
// Must be called with cb.mu held.
func (cb *CircuitBreaker) tripOpen() {
	cb.state = StateOpen
	cb.lastFailure = time.Now()
}
