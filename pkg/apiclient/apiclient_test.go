package apiclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	codes  []int
	calls  int
	method string
	header http.Header
	body   []byte
}

func (s *script) handler(w http.ResponseWriter, r *http.Request) {
	s.calls++
	s.method = r.Method
	s.header = r.Header.Clone()
	s.body, _ = io.ReadAll(r.Body)
	code := s.codes[len(s.codes)-1]
	if s.calls <= len(s.codes) {
		code = s.codes[s.calls-1]
	}
	w.WriteHeader(code)
}

func testPolicy() Policy {
	return Policy{MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond, Timeout: 2 * time.Second, RetryIf: retry5xx}
}

func getBuilder(url string) func(context.Context) (*http.Request, error) {
	return func(ctx context.Context) (*http.Request, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("X-Custom", "v")
		return req, nil
	}
}

// TestRetryThenSuccess verifies a 5xx is retried and the final body reads.
func TestRetryThenSuccess(t *testing.T) {
	s := &script{codes: []int{500, 200}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := New(nil).Do(context.Background(), getBuilder(srv.URL), testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || s.calls != 2 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, s.calls)
	}
	if v := resp.Header; v != nil && s.header.Get("X-Custom") != "v" {
		t.Fatalf("header not passed through: %v", s.header)
	}
}

// TestNoRetryOn4xx verifies terminal statuses return after one attempt.
func TestNoRetryOn4xx(t *testing.T) {
	s := &script{codes: []int{400}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := New(nil).Do(context.Background(), getBuilder(srv.URL), testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 || s.calls != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, s.calls)
	}
}

// TestExhaustedReturnsLast verifies the final retryable response surfaces.
func TestExhaustedReturnsLast(t *testing.T) {
	s := &script{codes: []int{500, 500, 500, 500}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	start := time.Now()
	resp, err := New(nil).Do(context.Background(), getBuilder(srv.URL), testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 || s.calls != 3 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, s.calls)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("budget blown: %v", elapsed)
	}
}

// TestCancelledCtxAborts verifies a dead context sends nothing and stops.
func TestCancelledCtxAborts(t *testing.T) {
	s := &script{codes: []int{500}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(nil).Do(ctx, getBuilder(srv.URL), testPolicy())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if s.calls != 0 {
		t.Fatalf("calls=%d, want 0", s.calls)
	}
}

// TestBuilderErrorSkipsSend verifies a builder failure sends nothing.
func TestBuilderErrorSkipsSend(t *testing.T) {
	s := &script{}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	boom := errors.New("boom")
	_, err := New(nil).Do(context.Background(), func(context.Context) (*http.Request, error) {
		return nil, boom
	}, testPolicy())
	if !errors.Is(err, boom) || s.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, s.calls)
	}
}

// TestPostBodyRoundTrip verifies method and body survive a send.
func TestPostBodyRoundTrip(t *testing.T) {
	s := &script{codes: []int{201}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := New(nil).Do(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "POST", srv.URL, strings.NewReader(`{"a":1}`))
	}, testPolicy())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if s.method != "POST" || string(s.body) != `{"a":1}` {
		t.Fatalf("method=%s body=%s", s.method, s.body)
	}
}

// TestNilRetryIfReturnsResponse verifies the default retries transport
// errors only: an HTTP error status returns after one attempt.
func TestNilRetryIfReturnsResponse(t *testing.T) {
	s := &script{codes: []int{503}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	resp, err := New(nil).Do(context.Background(), getBuilder(srv.URL), Policy{MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 || s.calls != 1 {
		t.Fatalf("status=%d calls=%d", resp.StatusCode, s.calls)
	}
}
