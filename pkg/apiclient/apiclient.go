// Package apiclient is a stdlib HTTP client with bounded retries.
//
// Retry policy lives here and only here: callers pass what to build and
// when to retry, and get back the final attempt. Backoff is exponential
// with jitter, capped, and honors context cancellation between attempts.
package apiclient

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// Policy bounds a Do call. MaxAttempts counts the initial try plus retries;
// values < 1 mean one attempt. RetryIf decides per attempt; nil retries
// transport errors only and returns any HTTP response as-is.
type Policy struct {
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Timeout     time.Duration // per-attempt timeout; 0 = none
	RetryIf     func(resp *http.Response, err error) bool
}

// Client wraps an *http.Client. A nil inner client means http.DefaultClient.
type Client struct{ http *http.Client }

func New(h *http.Client) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	return &Client{http: h}
}

func (c *Client) httpClient() *http.Client {
	if c == nil || c.http == nil {
		return http.DefaultClient
	}
	return c.http
}

// Do builds and sends one request per attempt until success, a terminal
// result, the budget is spent, or ctx ends. Intermediate response bodies
// are drained and closed for connection reuse; the returned body (if any)
// belongs to the caller. A builder error never retries.
func (c *Client) Do(ctx context.Context, build func(ctx context.Context) (*http.Request, error), p Policy) (*http.Response, error) {
	attempts := p.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	retryIf := p.RetryIf
	if retryIf == nil {
		retryIf = func(_ *http.Response, err error) bool { return err != nil }
	}

	var resp *http.Response
	var err error
	for a := 0; a < attempts; a++ {
		if a > 0 && !sleep(ctx, backoff(p.BaseBackoff, p.MaxBackoff, a)) {
			break
		}
		attemptCtx := ctx
		cancel := context.CancelFunc(func() {})
		if p.Timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, p.Timeout)
		}
		var req *http.Request
		req, err = build(attemptCtx)
		if err != nil {
			cancel()
			return nil, err
		}
		resp, err = c.httpClient().Do(req)
		cancel()
		if !retryIf(resp, err) || a == attempts-1 {
			return resp, err
		}
		drainClose(resp)
		resp = nil
	}
	if err == nil {
		err = ctx.Err()
	}
	return nil, err
}

// backoff returns the capped exponential delay for retry number a (1-based)
// with half-to-full jitter.
func backoff(base, max time.Duration, a int) time.Duration {
	if base <= 0 {
		return 0
	}
	d := base << (a - 1)
	if d > max || d <= 0 {
		d = max
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func drainClose(r *http.Response) {
	if r == nil || r.Body == nil {
		return
	}
	io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	r.Body.Close()
}
