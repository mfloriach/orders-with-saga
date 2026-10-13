package payment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paymentgateway/pkg/apiclient"
)

type mcScript struct {
	codes  []int
	bodies []string
	calls  int
	method string
	path   string
	header http.Header
	body   []byte
}

func (s *mcScript) handler(w http.ResponseWriter, r *http.Request) {
	s.calls++
	s.method = r.Method
	s.path = r.URL.Path
	s.header = r.Header.Clone()
	s.body, _ = io.ReadAll(r.Body)
	i := s.calls - 1
	if i >= len(s.codes) {
		i = len(s.codes) - 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.codes[i])
	w.Write([]byte(s.bodies[i]))
}

func mcTestRepo(url string) *MasterCardRepository {
	return NewMasterCardRepository(apiclient.New(nil), MasterCardConfig{
		BaseURL: url, APIKey: "k", Timeout: 2 * time.Second,
		MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond,
	})
}

func mcStatusBody(status string) string {
	return `{"id":"p-1","status":"` + status + `"}`
}

// TestCallPut verifies method, path, auth, body, and key header.
func TestCallPut(t *testing.T) {
	s := &mcScript{codes: []int{200}, bodies: []string{mcStatusBody("accepted")}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	if err := mcTestRepo(srv.URL).Call(context.Background(), "p-1", 100, "k-1"); err != nil {
		t.Fatal(err)
	}
	if s.method != "PUT" || s.path != "/payment/p-1" {
		t.Fatalf("method=%s path=%s", s.method, s.path)
	}
	if got := s.header.Get("Authorization"); got != "Bearer k" {
		t.Fatalf("auth=%q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(s.body, &body); err != nil {
		t.Fatal(err)
	}
	if body["amount"] != float64(100) || body["idempotency_key"] != "k-1" {
		t.Fatalf("body=%s", s.body)
	}
}

// TestCallRetries5xx verifies transient failures retry then succeed.
func TestCallRetries5xx(t *testing.T) {
	s := &mcScript{codes: []int{500, 500, 200}, bodies: []string{"x", "x", mcStatusBody("accepted")}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	if err := mcTestRepo(srv.URL).Call(context.Background(), "p-1", 100, "k-1"); err != nil {
		t.Fatal(err)
	}
	if s.calls != 3 {
		t.Fatalf("calls=%d, want 3", s.calls)
	}
}

// TestCallRefused verifies 4xx fails without retry.
func TestCallRefused(t *testing.T) {
	s := &mcScript{codes: []int{400}, bodies: []string{"nope"}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	r := mcTestRepo(srv.URL)
	err := r.Call(context.Background(), "p-1", 100, "k-1")
	if err == nil || r.Retryable(err) || s.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, s.calls)
	}
}

// TestStatusMapping verifies accepted/rejected/pending mapping.
func TestStatusMapping(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		accepted bool
		wantErr  bool
	}{
		{"accepted", "accepted", true, false},
		{"rejected", "rejected", false, false},
		{"pending is unconfirmed", "pending", false, true},
		{"unknown is unconfirmed", "weird", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &mcScript{codes: []int{200}, bodies: []string{mcStatusBody(tc.status)}}
			srv := httptest.NewServer(http.HandlerFunc(s.handler))
			defer srv.Close()

			accepted, err := mcTestRepo(srv.URL).Status(context.Background(), "p-1")
			if tc.wantErr != (err != nil) || accepted != tc.accepted {
				t.Fatalf("accepted=%v err=%v", accepted, err)
			}
			if s.method != "GET" || s.path != "/payment/p-1" {
				t.Fatalf("method=%s path=%s", s.method, s.path)
			}
		})
	}
}

// TestStatusTransportError verifies a down simulator surfaces an error.
func TestStatusTransportError(t *testing.T) {
	s := &mcScript{codes: []int{500}, bodies: []string{"x"}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	if _, err := mcTestRepo(srv.URL).Status(context.Background(), "p-1"); err == nil {
		t.Fatal("expected error")
	}
}

// TestCreatePayment verifies POST creation and key header.
func TestCreatePayment(t *testing.T) {
	s := &mcScript{codes: []int{201}, bodies: []string{`{"id":"net-9","status":"pending"}`}}
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	defer srv.Close()

	id, err := mcTestRepo(srv.URL).CreatePayment(context.Background(), 100, "k-1")
	if err != nil || id != "net-9" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if s.method != "POST" || s.path != "/payment" || s.header.Get("Idempotency-Key") != "k-1" {
		t.Fatalf("method=%s path=%s key=%s", s.method, s.path, s.header.Get("Idempotency-Key"))
	}
}

// TestCallOverallTimeout verifies one repo call never exceeds its budget.
func TestCallOverallTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	r := NewMasterCardRepository(apiclient.New(nil), MasterCardConfig{
		BaseURL: srv.URL, OverallTimeout: 50 * time.Millisecond,
		MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond,
	})
	start := time.Now()
	if err := r.Call(context.Background(), "p-1", 100, "k-1"); err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("call exceeded budget: %v", elapsed)
	}
}

// TestRetryableMatrix verifies the classifier.
func TestRetryableMatrix(t *testing.T) {
	r := mcTestRepo("http://invalid")
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout", context.DeadlineExceeded, true},
		{"canceled reconciles", context.Canceled, true},
		{"plain error", errors.New("x"), false},
		{"http 500", &httpError{StatusCode: 500}, true},
		{"http 400", &httpError{StatusCode: 400}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.Retryable(tc.err); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
