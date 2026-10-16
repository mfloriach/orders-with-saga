package steps

import (
	"context"
	"fmt"
	"log/slog"
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/saga"
)

type Authorize struct {
	Payments    domain.PaymentRepository
	CardNetwork domain.CardNetwork
}

func (s Authorize) Name() string { return "authorize" }

func (s Authorize) Do(ctx context.Context, p *domain.Payment) error {
	if err := s.CardNetwork.Authenticate(ctx, p.ID, p.Amount, p.IdempotencyKey); err != nil {
		return fmt.Errorf("payment: card network did not accept %s: %w", p.ID, err)
	}

	if err := s.Payments.MarkAuthorized(ctx, p); err != nil {
		return fmt.Errorf("payment: failed to mark authorized: %w", err)
	}

	return nil
}

func (s Authorize) Compensate(ctx context.Context, p *domain.Payment) error {
	_, status, err := s.CardNetwork.GetPayment(ctx, p.ID)
	if err != nil {
		return &saga.InDoubtError{Step: "authorize", Err: err}
	}

	switch domain.NetworkStatus(status) {
	case domain.NetworkAccepted:
		// if err := markApplied(ctx, s.Payments, p, domain.StatusAuthorized, s.Payments.MarkAuthorized); err != nil {
		// 	return fmt.Errorf("payment: failed to mark authorized: %w", err)
		// }

		if err := s.CardNetwork.Capture(ctx, p.ID, p.Amount, p.IdempotencyKey); err != nil {
			return &saga.InDoubtError{Step: "capture", Err: err}
		}

		// return markApplied(ctx, s.Payments, p, domain.StatusCaptured, s.Payments.MarkCaptured)
		return nil
	case domain.NetworkRejected:
		// Terminal refusal: nothing to reconcile. Log it and let
		// the unwind converge — downstream steps (mark failed)
		// still run this attempt.
		slog.InfoContext(ctx, "payment: authorize nothing to reconcile, remote refused",
			slog.String("payment_id", p.ID))
		return nil
	default:
		return &saga.InDoubtError{Step: "authorize", Err: err}
	}
}
