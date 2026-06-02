package stability

import (
	"context"
	"io"
	"net"
	"strings"
	"time"
)

// isRetryableError determines whether an error is transient and should be retried.
// Returns true for network-related errors (EOF, timeout, connection reset, etc.)
// and false for permanent errors (auth, not found, invalid request, etc.).
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// EOF is a common transient error when connections are closed unexpectedly
	if err == io.EOF {
		return true
	}

	errMsg := err.Error()

	// Check for common network error patterns
	retryablePatterns := []string{
		"EOF",
		"connection reset",
		"connection refused",
		"broken pipe",
		"timeout",
		"i/o timeout",
		"context deadline exceeded",
		"connection timed out",
		"network is unreachable",
		"temporary failure",
	}

	lowerMsg := strings.ToLower(errMsg)
	for _, pattern := range retryablePatterns {
		if strings.Contains(lowerMsg, pattern) {
			return true
		}
	}

	// Check for net.Error interface (timeout/temporary errors)
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts int
	WaitTime    time.Duration
	BackoffFunc func(attempt int) time.Duration
}

// DefaultBackoff 返回默认的指数退避函数: waitTime * 2^(attempt-1)
func DefaultBackoff(waitTime time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		shift := uint(attempt - 1)
		if shift > 62 {
			shift = 62
		}
		return waitTime * (1 << shift)
	}
}

// Retry executes an operation with retry logic.
// fn is called up to cfg.MaxAttempts times.
// Only retryable errors (network/timeout issues) trigger a retry.
// Permanent errors (auth, not found, invalid request) are returned immediately.
// During retry waits, context cancellation is respected.
func Retry(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) error) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}

	backoff := cfg.BackoffFunc
	if backoff == nil {
		backoff = DefaultBackoff(cfg.WaitTime)
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Don't retry for permanent errors (auth, not found, etc.)
		if !isRetryableError(lastErr) {
			return lastErr
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Check context before sleeping
		if ctx.Err() != nil {
			return ctx.Err()
		}

		wait := backoff(attempt)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return lastErr
}
