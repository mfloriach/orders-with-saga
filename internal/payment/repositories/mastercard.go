package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apiclient"
)

// payment is the part of the simulator's payment representation we decode.
type MasterCardResponse struct {
	ID     string               `json:"id"`
	Status domain.NetworkStatus `json:"status"`
}

// MasterCardRepository implements domain.CardNetwork against the card-network
// simulator. The injected client owns the base URL, auth header, per-attempt
// timeout, and retry loop; this type knows only the simulator's paths and
// payloads.
type MasterCardRepository struct {
	client *apiclient.Client
}

func NewMasterCardRepository(client *apiclient.Client) *MasterCardRepository {
	return &MasterCardRepository{client: client}
}

// Authorize holds funds via an idempotent PUT and returns the simulator's
// authorize key, which Capture needs.
func (r *MasterCardRepository) Authorize(ctx context.Context, paymentID string, amount int64, idemKey string) (string, error) {
	path := "/payment/" + paymentID
	body, err := json.Marshal(map[string]any{"amount": amount, "idempotency_key": idemKey})
	if err != nil {
		return "", &domain.ErrorCardNetwork{Path: path, Err: err}
	}

	resp, err := r.client.Put(ctx, path, body)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	var p MasterCardResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&p); err != nil {
		return "", &domain.ErrorCardNetwork{Path: path, Err: err}
	}

	return p.ID, nil
}

// GetPayment fetches the remote (id, status) without interpreting it; callers
// map the simulator vocabulary themselves.
func (r *MasterCardRepository) GetPayment(ctx context.Context, paymentID string) (domain.NetworkStatus, error) {
	path := "/payment/" + paymentID
	resp, err := r.client.Get(ctx, path)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	var p MasterCardResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&p); err != nil {
		return domain.NetworkRejected, &domain.ErrorCardNetwork{Path: path, Err: err}
	}

	return p.Status, nil
}

// Capture settles a previously authenticated payment via an idempotent POST.
func (r *MasterCardRepository) Capture(ctx context.Context, authorizeID string) error {
	path := "/payment/" + authorizeID + "/capture"
	resp, err := r.client.Post(ctx, path, nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var p MasterCardResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&p); err != nil {
		return &domain.ErrorCardNetwork{Path: path, Err: err}
	}

	if p.Status != domain.NetworkRejected {
		return &domain.ErrorCardNetwork{Path: path, Err: fmt.Errorf("payment was not captured")}
	}

	return nil
}
