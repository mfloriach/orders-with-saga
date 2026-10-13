package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"paymentgateway/pkg/apiclient"
)

// MasterCardConfig carries the card-network simulator settings. Zero
// attempts/backoff fall back to apiclient single-shot behavior; prefer
// explicit values from validated env.
type MasterCardConfig struct {
	BaseURL        string
	APIKey         string
	Timeout        time.Duration
	OverallTimeout time.Duration
	MaxAttempts    int
	BaseBackoff    time.Duration
	MaxBackoff     time.Duration
}

// defaultOverallTimeout bounds one repo call (including its internal
// retries) when OverallTimeout is unset.
const defaultOverallTimeout = 10 * time.Second

// Assumed simulator statuses (confirm in review).
const (
	mcStatusAccepted = "accepted"
	mcStatusRejected = "rejected"
)

// httpError carries a non-2xx network status for Retryable classification.
type httpError struct {
	StatusCode int
	Body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("mastercard: unexpected status %d: %s", e.StatusCode, e.Body)
}

// MasterCardRepository implements Processor against the card-network
// simulator. It satisfies Processor at compile time.
type MasterCardRepository struct {
	client *apiclient.Client
	cfg    MasterCardConfig
}

var _ Processor = (*MasterCardRepository)(nil)

func NewMasterCardRepository(client *apiclient.Client, cfg MasterCardConfig) *MasterCardRepository {
	if cfg.OverallTimeout <= 0 {
		cfg.OverallTimeout = defaultOverallTimeout
	}
	return &MasterCardRepository{client: client, cfg: cfg}
}

// withOverallTimeout bounds one call including its internal retries.
func (r *MasterCardRepository) withOverallTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.cfg.OverallTimeout)
}

func (r *MasterCardRepository) policy() apiclient.Policy {
	return apiclient.Policy{
		MaxAttempts: r.cfg.MaxAttempts,
		BaseBackoff: r.cfg.BaseBackoff,
		MaxBackoff:  r.cfg.MaxBackoff,
		Timeout:     r.cfg.Timeout,
		RetryIf: func(resp *http.Response, err error) bool {
			if err != nil {
				return isTransient(err)
			}
			return resp.StatusCode >= 500
		},
	}
}

func (r *MasterCardRepository) authed(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	}
	return req, nil
}

func decodePayment(resp *http.Response) (id, status string, err error) {
	defer resp.Body.Close()
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", err
	}
	return out.ID, out.Status, nil
}

func statusError(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return &httpError{StatusCode: resp.StatusCode, Body: string(body)}
}

// Call authorizes via idempotent PUT; the client retries transient
// failures internally, so one Call covers the whole attempt budget.
func (r *MasterCardRepository) Call(ctx context.Context, paymentID string, amount int64, idemKey string) error {
	body, err := json.Marshal(map[string]any{"amount": amount, "idempotency_key": idemKey})
	if err != nil {
		return err
	}

	ctx, cancel := r.withOverallTimeout(ctx)
	defer cancel()
	resp, err := r.client.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		return r.authed(ctx, http.MethodPut, r.cfg.BaseURL+"/payment/"+paymentID, body)
	}, r.policy())
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp)
	}
	_, _, err = decodePayment(resp)

	return err
}

// Status maps the network state: accepted→true, rejected→false, anything
// else (including transport errors) is an error so the caller reconciles
// via the in-doubt path instead of compensating a live payment.
func (r *MasterCardRepository) Status(ctx context.Context, paymentID string) (bool, error) {
	ctx, cancel := r.withOverallTimeout(ctx)
	defer cancel()
	resp, err := r.client.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		return r.authed(ctx, http.MethodGet, r.cfg.BaseURL+"/payment/"+paymentID, nil)
	}, r.policy())

	if err != nil {
		return false, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, statusError(resp)
	}

	_, status, err := decodePayment(resp)
	if err != nil {
		return false, err
	}

	switch status {
	case mcStatusAccepted:
		return true, nil
	case mcStatusRejected:
		return false, nil
	default:
		return false, fmt.Errorf("mastercard: unconfirmed status %q", status)
	}
}

// CreatePayment creates by idempotency key (key-based flow; unused by
// Create today, which authorizes via idempotent PUT).
func (r *MasterCardRepository) CreatePayment(ctx context.Context, amount int64, idemKey string) (string, error) {
	body, err := json.Marshal(map[string]any{"amount": amount})
	if err != nil {
		return "", err
	}

	ctx, cancel := r.withOverallTimeout(ctx)
	defer cancel()
	resp, err := r.client.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := r.authed(ctx, http.MethodPost, r.cfg.BaseURL+"/payment", body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Idempotency-Key", idemKey)
		return req, nil
	}, r.policy())

	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", statusError(resp)
	}

	id, _, err := decodePayment(resp)

	return id, err
}

// Retryable classifies errors for the seam: transport transients and 5xx
// (via httpError) retry; 4xx, decode failures, and anything else do not.
func (r *MasterCardRepository) Retryable(err error) bool {
	var he *httpError
	if errors.As(err, &he) {
		return he.StatusCode >= 500
	}
	return isTransient(err)
}

func isTransient(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
