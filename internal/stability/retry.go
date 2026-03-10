package stability

import (
	"context"
	"time"
)

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

// Retry 执行带重试的操作。
// fn 最多被调用 cfg.MaxAttempts 次。如果所有尝试均失败，返回最后一次的错误。
// 在重试等待期间会响应 context 取消。
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
