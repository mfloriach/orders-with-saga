package payment

import (
	"context"
	"errors"
	"net/http"

	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment/models"
	"paymentgateway/pkg/apperr"
	"paymentgateway/pkg/idempotency"
	"paymentgateway/pkg/validate"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ svc creator }

// creator is the subset of Service the handler needs (stubbed in tests).
// The raw Idempotency-Key header is passed through; key resolution lives
// in the service.
type creator interface {
	Create(ctx context.Context, merchantID string, amount int64, idemHeader string) (*models.Payment, error)
}

func NewHandler(svc creator) *Handler { return &Handler{svc} }

type createBody struct {
	MerchantID string `json:"merchant_id" validate:"required,uuid"`
	Amount     int64  `json:"amount" validate:"gt=0"`
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.InvalidJSON()
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	idemKey := c.Get("Idempotency-Key", idempotency.GenerateKey(c.Request().RequestURI()))

	p, err := h.svc.Create(c.Context(), b.MerchantID, b.Amount, idemKey)
	var ce *ConflictError
	switch {
	case errors.Is(err, merchant.ErrNotFound):
		return apperr.MerchantNotFound()
	case errors.As(err, &ce):
		id := ""
		if ce.Existing != nil {
			id = ce.Existing.ID
			c.Set("Idempotency-Key", ce.Existing.IdempotencyKey)
		}
		return apperr.IdempotencyConflict(id)
	case err != nil:
		return apperr.InternalError()
	}

	c.Set("Idempotency-Key", p.IdempotencyKey)
	return c.Status(http.StatusCreated).JSON(p)
}
