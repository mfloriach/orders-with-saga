package workflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"paymentgateway/internal/config"
	"paymentgateway/internal/domain"
)

type capture struct {
	Payments    domain.PaymentRepository
	CardNetwork domain.CardNetwork
	Outbox      domain.OutBoxPaymentRepository
	Db          *sql.DB
}

func (a capture) Name() string { return "capture" }

func (a capture) Do(ctx context.Context, w *PaymentWorkflow) error {
	p := w.Authenticated
	if p == nil {
		return errors.New("payment is not pending")
	}

	if err := a.CardNetwork.Capture(ctx, p.ID); err != nil {
		a.Payments.Failed(ctx, p.ID, fmt.Errorf("payment: failed to capture: %w", err))
		return err
	}

	return config.WithinTx(ctx, a.Db, func(txCtx context.Context) error {
		pCaptured, err := a.Payments.Captured(txCtx, *p)
		if err != nil {
			return err
		}

		if err := a.Outbox.Store(txCtx, *p); err != nil {
			return err
		}

		w.Captured = &pCaptured

		return nil
	})
}

func (a capture) Compensate(ctx context.Context, w *PaymentWorkflow) error {
	p := w.Authenticated
	if p == nil {
		return errors.New("payment is not pending")
	}

	status, err := a.CardNetwork.GetPayment(ctx, p.AuthenticationKey)
	if err != nil {
		return err
	}

	switch status {
	case domain.NetworkRejected:
		_, err := a.Payments.Failed(ctx, p.ID, fmt.Errorf("rejected by card network"))
		if err != nil {
			return err
		}
		return nil
	case domain.NetworkAccepted:
	default:
		return fmt.Errorf("payment required investigation paymentID: %s, status: %s", p.ID, status)
	}

	return nil
}
