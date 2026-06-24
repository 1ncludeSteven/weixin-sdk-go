package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

// RetryPolicy defines exponential backoff retry configuration.
type RetryPolicy struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// DefaultRetryPolicy returns a conservative default retry policy.
// 5 retries with exponential backoff starting at 500ms, capped at 30s.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:     5,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
	}
}

// isRetryableError determines if an error should be retried.
// Retryable: connection refused, DNS errors, timeout, reset, 5xx, 429.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return true
	case strings.Contains(msg, "connection reset"):
		return true
	case strings.Contains(msg, "no such host"):
		return true
	case strings.Contains(msg, "i/o timeout"):
		return true
	default:
		return false
	}
}

// isRetryableStatus returns true for HTTP status codes that should be retried.
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// calculateBackoff computes backoff duration for the given attempt.
// Uses exponential backoff: initial * multiplier^attempt, capped at maxBackoff.
func (p RetryPolicy) calculateBackoff(attempt int) time.Duration {
	backoff := float64(p.InitialBackoff) * math.Pow(p.Multiplier, float64(attempt))
	if backoff > float64(p.MaxBackoff) {
		backoff = float64(p.MaxBackoff)
	}
	return time.Duration(backoff)
}

// waitForRetry waits for the backoff duration, respecting context cancellation.
func waitForRetry(ctx context.Context, backoff time.Duration) error {
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// doRequestWithRetry executes a request with retry logic.
// Retries on network errors and 5xx/429 status codes using exponential backoff.
func (c *Client) doRequestWithRetry(
	ctx context.Context,
	endpoint string,
	reqBody interface{},
	timeout time.Duration,
	policy RetryPolicy,
) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := policy.calculateBackoff(attempt - 1)
			if err := waitForRetry(ctx, backoff); err != nil {
				return nil, fmt.Errorf("retry cancelled: %w", err)
			}
		}

		respBody, err := c.doRequest(ctx, endpoint, reqBody, timeout)
		if err == nil {
			return respBody, nil
		}

		lastErr = err

		// Determine if retryable
		retryable := false
		if isRetryableError(err) {
			retryable = true
		} else if isRetryableStatusFromErr(err) {
			retryable = true
		}

		if !retryable || attempt >= policy.MaxRetries {
			break
		}
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", policy.MaxRetries, lastErr)
}

// isRetryableStatusFromErr extracts HTTP status code from error message
// and checks if it's retryable.
// The original doRequest formats HTTP errors as "API error: STATUS: body".
func isRetryableStatusFromErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Extract 3-digit status code from "STATUS_CODE Message" pattern
	for i := 0; i < len(msg)-3; i++ {
		if msg[i] >= '0' && msg[i] <= '9' &&
			msg[i+1] >= '0' && msg[i+1] <= '9' &&
			msg[i+2] >= '0' && msg[i+2] <= '9' &&
			(i == 0 || msg[i-1] == ' ') {
			// Parse the 3-digit code
			var code int
			for j := 0; j < 3; j++ {
				code = code*10 + int(msg[i+j]-'0')
			}
			return isRetryableStatus(code)
		}
	}
	return false
}
