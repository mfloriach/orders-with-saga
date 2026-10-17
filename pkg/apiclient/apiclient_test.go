package apiclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func retry5xx(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode >= 500
}

type script struct {
	mu     sync.Mutex
	codes  []int
	calls  int
	method string
	path   string
	header http.Header
	body   []byte
}

func (s *script) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.calls++
	s.method = r.Method
	s.path = r.URL.Path
	s.header = r.Header.Clone()
	s.body, _ = io.ReadAll(r.Body)
	code := 200
	if len(s.codes) > 0 {
		code = s.codes[len(s.codes)-1]
		if s.calls <= len(s.codes) {
			code = s.codes[s.calls-1]
		}
	}
	s.mu.Unlock()
	w.WriteHeader(code)
}

func (s *script) snapshot() (calls int, method, path string, header http.Header, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, s.method, s.path, s.header, s.body
}

func testClient() *Client {
	return New(nil,
		WithRetry(3, time.Millisecond, 5*time.Millisecond, retry5xx),
		WithTimeout(2*time.Second),
	)
}

// TestOptionsWriteFields verifies every option writes its config field.
func TestOptionsWriteFields(t *testing.T) {
	var c config
	WithBaseURL("http://x")(&c)
	WithHeader("A", "1")(&c)
	WithHeaders(http.Header{"B": {"2"}})(&c)
	WithTimeout(3 * time.Second)(&c)
	WithRetry(4, time.Second, 2*time.Second, retry5xx)(&c)

	if c.baseURL != "http://x" || c.headers.Get("A") != "1" || c.headers.Get("B") != "2" {
		t.Fatalf("base/headers = %+v", c)
	}
	if c.timeout != 3*time.Second || c.maxAttempts != 4 || c.baseBackoff != time.Second || c.maxBackoff != 2*time.Second || c.retryIf == nil {
		t.Fatalf("retry/timeout = %+v", c)
	}
}

// TestCloneDeepCopiesHeaders verifies per-request mutation cannot leak back.
func TestCloneDeepCopiesHeaders(t *testing.T) {
	orig := config{headers: http.Header{"A": {"1"}}}
	cp := orig.clone()
	cp.headers.Set("A", "2")
	cp.headers.Set("B", "3")
	if orig.headers.Get("A") != "1" || orig.headers.Get("B") != "" {
		t.Fatalf("clone mutated original: %v", orig.headers)
	}
}

// TestNewAppliesOptions verifies a nil client defaults and options take effect.
func TestNewAppliesOptions(t *testing.T) {
	c := New(nil, WithBaseURL("http://h"), WithTimeout(time.Second), WithHeader("X", "y"))
	if c.http != http.DefaultClient {
		t.Fatal("nil client should default to http.DefaultClient")
	}
	if c.cfg.baseURL != "http://h" || c.cfg.timeout != time.Second || c.cfg.headers.Get("X") != "y" {
		t.Fatalf("options not applied: %+v", c.cfg)
	}
}

// TestRetryThenSuccess verifies a 5xx is retried and headers are sent.
func TestRetryThenSuccess(t *testing.T) {
	s := &script{codes: []int{500, 200}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	c := New(nil,
		WithRetry(3, time.Millisecond, 5*time.Millisecond, retry5xx),
		WithTimeout(2*time.Second),
		WithHeader("X-Custom", "v"),
	)
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	calls, _, _, header, _ := s.snapshot()
	if resp.StatusCode != 200 || calls != 2 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, calls)
	}
	if header.Get("X-Custom") != "v" {
		t.Fatalf("header not passed: %v", header)
	}
}

// TestNoRetryOn4xx verifies terminal statuses return after one attempt.
func TestNoRetryOn4xx(t *testing.T) {
	s := &script{codes: []int{400}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := testClient().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	calls, _, _, _, _ := s.snapshot()
	if resp.StatusCode != 400 || calls != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, calls)
	}
}

// TestFalsePredicateStops verifies a predicate that declines ends the loop
// after a single attempt.
func TestFalsePredicateStops(t *testing.T) {
	s := &script{codes: []int{500}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	c := New(nil, WithRetry(3, time.Millisecond, 5*time.Millisecond, func(*http.Response, error) bool { return false }))
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	calls, _, _, _, _ := s.snapshot()
	if resp.StatusCode != 500 || calls != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, calls)
	}
}

// TestExhaustedReturnsLast verifies the final retryable response surfaces.
func TestExhaustedReturnsLast(t *testing.T) {
	s := &script{codes: []int{500, 500, 500, 500}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := testClient().Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	calls, _, _, _, _ := s.snapshot()
	if resp.StatusCode != 500 || calls != 3 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, calls)
	}
}

// TestMaxAttemptsBelowOne verifies attempts < 1 means a single try.
func TestMaxAttemptsBelowOne(t *testing.T) {
	s := &script{codes: []int{500, 200}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	c := New(nil, WithRetry(0, time.Millisecond, 5*time.Millisecond, retry5xx))
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	calls, _, _, _, _ := s.snapshot()
	if resp.StatusCode != 500 || calls != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, calls)
	}
}

// TestCancelledCtxAborts verifies a dead context sends nothing and stops.
func TestCancelledCtxAborts(t *testing.T) {
	s := &script{codes: []int{500}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testClient().Get(ctx, srv.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	calls, _, _, _, _ := s.snapshot()
	if calls != 0 {
		t.Fatalf("calls=%d, want 0", calls)
	}
}

// TestDefaultRetryIfReturnsResponse verifies the default retries transport
// errors only: an HTTP error status returns after one attempt.
func TestDefaultRetryIfReturnsResponse(t *testing.T) {
	s := &script{codes: []int{503}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	c := New(nil, WithRetry(3, time.Millisecond, 5*time.Millisecond, nil))
	resp, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	calls, _, _, _, _ := s.snapshot()
	if resp.StatusCode != 503 || calls != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, calls)
	}
}

// TestPostBodyRoundTrip verifies method and body survive a send.
func TestPostBodyRoundTrip(t *testing.T) {
	s := &script{codes: []int{201}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := testClient().Post(context.Background(), srv.URL, []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, method, _, _, body := s.snapshot()
	if method != "POST" || string(body) != `{"a":1}` {
		t.Fatalf("method=%s body=%s", method, body)
	}
}

// TestVerbs verifies each verb reaches the server with its method.
func TestVerbs(t *testing.T) {
	s := &script{}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()
	c := New(nil)
	ctx := context.Background()

	closeBody := func(r *http.Response, err error) error {
		if err == nil {
			r.Body.Close()
		}
		return err
	}
	steps := []struct {
		want string
		do   func() error
	}{
		{http.MethodGet, func() error { return closeBody(c.Get(ctx, srv.URL)) }},
		{http.MethodPost, func() error { return closeBody(c.Post(ctx, srv.URL, []byte("x"))) }},
		{http.MethodPut, func() error { return closeBody(c.Put(ctx, srv.URL, []byte("x"))) }},
		{http.MethodPatch, func() error { return closeBody(c.Patch(ctx, srv.URL, []byte("x"))) }},
		{http.MethodDelete, func() error { return closeBody(c.Delete(ctx, srv.URL)) }},
	}
	for _, step := range steps {
		if err := step.do(); err != nil {
			t.Fatal(err)
		}
		if _, method, _, _, _ := s.snapshot(); method != step.want {
			t.Fatalf("method=%s want %s", method, step.want)
		}
	}
}

// TestPerRequestHeaderOverridesGlobal verifies per-request headers win and the
// client default is not mutated.
func TestPerRequestHeaderOverridesGlobal(t *testing.T) {
	s := &script{}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	c := New(nil, WithHeader("X-G", "global"))
	ctx := context.Background()

	resp, err := c.Get(ctx, srv.URL, WithHeader("X-G", "local"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, _, _, header, _ := s.snapshot(); header.Get("X-G") != "local" {
		t.Fatalf("per-request header = %q, want local", header.Get("X-G"))
	}

	resp, err = c.Get(ctx, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, _, _, header, _ := s.snapshot(); header.Get("X-G") != "global" {
		t.Fatalf("global header mutated: %q", header.Get("X-G"))
	}
}

// TestAbsolutePathBypassesBaseURL verifies an absolute path ignores baseURL.
func TestAbsolutePathBypassesBaseURL(t *testing.T) {
	base := &script{codes: []int{418}}
	baseSrv := httptest.NewServer(http.HandlerFunc(base.handler))
	defer baseSrv.Close()

	target := &script{codes: []int{200}}
	targetSrv := httptest.NewServer(http.HandlerFunc(target.handler))
	defer targetSrv.Close()

	c := New(nil, WithBaseURL(baseSrv.URL))
	resp, err := c.Get(context.Background(), targetSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d, want 200 (absolute path ignored base URL)", resp.StatusCode)
	}
	if calls, _, _, _, _ := base.snapshot(); calls != 0 {
		t.Fatalf("base server called %d times", calls)
	}
}

// TestRelativePathJoinsBaseURL verifies relative paths are prefixed, trimming
// the base's trailing slash and the path's leading slash.
func TestRelativePathJoinsBaseURL(t *testing.T) {
	s := &script{}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	c := New(nil, WithBaseURL(srv.URL+"/v1/"))
	resp, err := c.Get(context.Background(), "payment/x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, _, path, _, _ := s.snapshot(); path != "/v1/payment/x" {
		t.Fatalf("path=%q, want /v1/payment/x", path)
	}
}

// TestTimeoutAborts verifies WithTimeout bounds a slow response.
func TestTimeoutAborts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(nil, WithTimeout(5*time.Millisecond))
	if _, err := c.Get(context.Background(), srv.URL); err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestBackoffCapped verifies the exponential delay never exceeds max.
func TestBackoffCapped(t *testing.T) {
	for a := 1; a < 10; a++ {
		if d := backoff(time.Second, 2*time.Second, a); d > 2*time.Second {
			t.Fatalf("backoff(%d)=%v exceeds cap", a, d)
		}
	}
}
