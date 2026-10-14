package steps

import (
	"context"
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/saga"
)

type StepInsertAudit struct {
	Audit models.AuditStore
}

func (s StepInsertAudit) Run(p *models.Payment) saga.Step {
	return saga.Step{
		Name: "insert-audit",
		Do: func(ctx context.Context) error {
			return s.Audit.Insert(ctx, p.ID, p.Amount)
		},
		Compensate: func(c context.Context) error {
			return s.Audit.MarkFailed(c, p.ID)
		},
	}
}
