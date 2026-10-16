package steps

import (
	"context"
	"fmt"
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/saga"
)

type Capture struct {
	Payments    domain.PaymentRepository
	CardNetwork domain.CardNetwork
}

func (s Capture) Run(p *domain.Payment) saga.Step {
	return saga.Step{
		Name: "call-api-capture",
		Do: func(ctx context.Context) error {
			if err := s.CardNetwork.Capture(ctx, p.ID, p.Amount, p.IdempotencyKey); err != nil {
				return &saga.InDoubtError{Step: "capture", Err: err}
			}

			if err := s.Payments.MarkCaptured(ctx, p); err != nil {
				return fmt.Errorf("payment: failed to mark captured: %w", err)
			}

			return nil
		},
		Compensate: func(ctx context.Context) error {
			_, status, err := s.CardNetwork.GetPayment(ctx, p.ID)
			if err != nil {
				return &saga.InDoubtError{Step: "capture", Err: fmt.Errorf("payment: could not get payment %s: %w", p.ID, err)}
			}

			switch domain.NetworkStatus(status) {
			case domain.NetworkCaptured:
				if err := s.Payments.MarkCaptured(ctx, p); err != nil {
					return fmt.Errorf("payment: failed to mark captured: %w", err)
				}

				return nil
			case domain.NetworkRejected:
				return fmt.Errorf("payment: card network did not accept %s: %w", p.ID, err)
			default:
				return &saga.InDoubtError{Step: "capture", Err: err}
			}
		},
	}
}
