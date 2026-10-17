// Package apiclient is a stdlib HTTP client with bounded retries.
//
// A Client is configured once at construction (base URL, default headers,
// retry policy, per-attempt timeout) and then issues requests through its
// verb methods. Backoff is exponential with jitter, capped, and honors
// context cancellation between attempts.
package apiclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

type HttpError struct {
	Path       string
	StatusCode int
	Body       string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("%s: %d", e.Path, e.StatusCode)
}

// Option configures a Client at construction, or a single request when passed
// to a verb. For a request, options layer over the client's defaults and later
// options win.
type Option func(*config)

type config struct {
	baseURL     string
	headers     http.Header
	maxAttempts int
	baseBackoff time.Duration
	maxBackoff  time.Duration
	timeout     time.Duration
	retryIf     RetryFunc
}

// RetryFunc decides whether an attempt is worth retrying. resp and err are the
// raw outcome of the send: resp is nil when the transport failed, and err is
// nil whenever a response came back. A nil RetryFunc retries transport errors
// and non-2xx responses.
type RetryFunc func(resp *http.Response, err error) bool

// retryNon2xx is the default RetryFunc: a transport error retries, and so does
// any response outside 2xx.
func retryNon2xx(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode < 200 || resp.StatusCode >= 300
}

// WithBaseURL sets the prefix for relative request paths. Absolute paths
// (http:// or https://) bypass it.
func WithBaseURL(u string) Option { return func(c *config) { c.baseURL = u } }

// WithHeader sets one default header, replacing any earlier value for key.
func WithHeader(key, value string) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set(key, value)
	}
}

// WithHeaders adds default headers.
func WithHeaders(h http.Header) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		for k, vs := range h {
			c.headers[k] = append([]string(nil), vs...)
		}
	}
}

// WithTimeout bounds each attempt's request build and send; 0 means none.
func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

// WithRetry sets the retry policy. maxAttempts counts the initial try plus
// retries; values < 1 mean one attempt. retryIf decides per attempt; nil
// retries transport errors and non-2xx responses.
func WithRetry(maxAttempts int, baseBackoff, maxBackoff time.Duration, retryIf RetryFunc) Option {
	return func(c *config) {
		c.maxAttempts = maxAttempts
		c.baseBackoff = baseBackoff
		c.maxBackoff = maxBackoff
		c.retryIf = retryIf
	}
}

// Client issues HTTP requests with a fixed policy. A nil inner client means
// http.DefaultClient.
type Client struct {
	http *http.Client
	cfg  config
}

// New builds a Client. A nil h means http.DefaultClient; options set the base
// URL, default headers, retry policy, and per-attempt timeout.
func New(h *http.Client, opts ...Option) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	c := &Client{http: h}
	for _, o := range opts {
		o(&c.cfg)
	}
	return c
}

func (c *Client) Get(ctx context.Context, path string, opts ...Option) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, path, nil, opts...)
}

func (c *Client) Post(ctx context.Context, path string, body []byte, opts ...Option) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, path, body, opts...)
}

func (c *Client) Put(ctx context.Context, path string, body []byte, opts ...Option) (*http.Response, error) {
	return c.do(ctx, http.MethodPut, path, body, opts...)
}

func (c *Client) Patch(ctx context.Context, path string, body []byte, opts ...Option) (*http.Response, error) {
	return c.do(ctx, http.MethodPatch, path, body, opts...)
}

func (c *Client) Delete(ctx context.Context, path string, opts ...Option) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, path, nil, opts...)
}

// Do builds and sends one request per attempt until success, a terminal
// result, the budget is spent, or ctx ends. A relative path joins the client's
// base URL; an absolute path is used as-is. Intermediate response bodies are
// drained and closed for connection reuse; the returned body (if any) belongs
// to the caller. A request-build error never retries. Each attempt's response
// status is checked: anything outside 2xx is retried until the budget runs out.
// The retry predicate sees the raw outcome of the send (see RetryFunc).
func (c *Client) do(ctx context.Context, method, path string, body []byte, opts ...Option) (*http.Response, error) {
	cfg := c.cfg.clone()
	for _, o := range opts {
		o(&cfg)
	}

	attempts := cfg.maxAttempts
	if attempts < 1 {
		attempts = 1
	}

	retryIf := cfg.retryIf
	if retryIf == nil {
		retryIf = retryNon2xx
	}

	endpoint := resolve(cfg.baseURL, path)

	var resp *http.Response
	var err error
	for a := 0; a < attempts; a++ {
		if a > 0 && !sleep(ctx, backoff(cfg.baseBackoff, cfg.maxBackoff, a)) {
			break
		}
		attemptCtx := ctx
		cancel := context.CancelFunc(func() {})
		if cfg.timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, cfg.timeout)
		}

		var reqBody io.Reader
		if body != nil {
			reqBody = bytes.NewReader(body)
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(attemptCtx, method, endpoint, reqBody)
		if err != nil {
			cancel()
			return nil, err
		}
		for k, vs := range cfg.headers {
			req.Header[k] = append([]string(nil), vs...)
		}

		resp, err = c.http.Do(req)
		cancel()
		if a == attempts-1 || !retryIf(resp, err) {
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

// clone deep-copies the header map so per-request options never mutate the
// client's defaults.
func (c config) clone() config {
	if c.headers != nil {
		h := make(http.Header, len(c.headers))
		for k, vs := range c.headers {
			h[k] = append([]string(nil), vs...)
		}
		c.headers = h
	}
	return c
}

// resolve joins a relative path onto baseURL, trimming the base's trailing and
// the path's leading slash. An absolute http(s) path is returned untouched, so
// one client can serve a fixed upstream and arbitrary absolute targets.
func resolve(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
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
