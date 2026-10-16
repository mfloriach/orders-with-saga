package steps

import (
	"context"
	"fmt"
	"log/slog"
	"paymentgateway/internal/domain"
	"paymentgateway/pkg/saga"
)

type Capture struct {
	Payments    domain.PaymentRepository
	CardNetwork domain.CardNetwork
}

func (s Capture) Name() string { return "capture" }

func (s Capture) Do(ctx context.Context, p *domain.Payment) error {
	if err := s.CardNetwork.Capture(ctx, p.ID, p.Amount, p.IdempotencyKey); err != nil {
		return &saga.InDoubtError{Step: "capture", Err: err}
	}

	if err := s.Payments.MarkCaptured(ctx, p); err != nil {
		return fmt.Errorf("payment: failed to mark captured: %w", err)
	}

	return nil
}

func (s Capture) Compensate(ctx context.Context, p *domain.Payment) error {
	_, status, err := s.CardNetwork.GetPayment(ctx, p.ID)
	if err != nil {
		return &saga.InDoubtError{Step: "capture", Err: fmt.Errorf("payment: could not get payment %s: %w", p.ID, err)}
	}

	switch domain.NetworkStatus(status) {
	case domain.NetworkCaptured:
		// return markApplied(ctx, s.Payments, p, domain.StatusCaptured, s.Payments.MarkCaptured)
		return nil
	case domain.NetworkRejected:
		// Terminal refusal: nothing to reconcile. Log it and let
		// the unwind converge — downstream steps (mark failed)
		// still run this attempt.
		slog.InfoContext(ctx, "payment: capture nothing to reconcile, remote refused",
			slog.String("payment_id", p.ID))
		return nil
	default:
		return &saga.InDoubtError{Step: "capture", Err: err}
	}
}
