package steps

import (
	"context"
	"fmt"
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/saga"
)

type Authorize struct {
	Payments    models.PaymentRepository
	CardNetwork models.CardNetwork
}

func (s Authorize) Run(p *models.Payment) saga.Step {
	return saga.Step{
		Name: "authorize",
		Do: func(ctx context.Context) error {
			if err := s.CardNetwork.Authenticate(ctx, p.ID, p.Amount, p.IdempotencyKey); err != nil {
				return fmt.Errorf("payment: card network did not accept %s: %w", p.ID, err)
			}

			if err := s.Payments.MarkAuthorized(ctx, p); err != nil {
				return fmt.Errorf("payment: failed to mark authorized: %w", err)
			}

			return nil
		},
		Compensate: func(ctx context.Context) error {
			_, status, err := s.CardNetwork.GetPayment(ctx, p.ID)
			if err != nil {
				return &saga.InDoubtError{Step: "authorize", Err: err}
			}

			switch models.NetworkStatus(status) {
			case models.NetworkAccepted:
				if err := s.Payments.MarkAuthorized(ctx, p); err != nil {
					return fmt.Errorf("payment: failed to mark authorized: %w", err)
				}

				if err := s.CardNetwork.Capture(ctx, p.ID, p.Amount, p.IdempotencyKey); err != nil {
					return &saga.InDoubtError{Step: "capture", Err: err}
				}

				if err := s.Payments.MarkCaptured(ctx, p); err != nil {
					return fmt.Errorf("payment: failed to mark captured: %w", err)
				}

				return nil
			case models.NetworkRejected:
				return fmt.Errorf("payment: card network did not accept %s: %w", p.ID, err)
			default:
				return &saga.InDoubtError{Step: "authorize", Err: err}
			}
		},
	}
}
