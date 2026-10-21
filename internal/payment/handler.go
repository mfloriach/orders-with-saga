package payment

import (
	"context"
	"errors"
	"net/http"

	"paymentgateway/internal/domain"
	"paymentgateway/internal/merchant"
	"paymentgateway/pkg/apperr"
	saga "paymentgateway/pkg/saga/v2"
	"paymentgateway/pkg/validate"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ svc creator }

// creator is the subset of Service the handler needs (stubbed in tests).
// The raw Idempotency-Key header is passed through; key resolution lives
// in the service.
type creator interface {
	Create(ctx context.Context, merchantID string, price domain.Price, idemHeader string) (*domain.PaymentCaptured, error)
}

func NewHandler(svc creator) *Handler { return &Handler{svc} }

type createBody struct {
	MerchantID string       `json:"merchant_id" validate:"required,uuid"`
	Price      domain.Price `json:"price"`
}

// CreatePayment godoc
// @Summary      Create a payment
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key header string false "Idempotency key"
// @Param        body body createBody true "Payment"
// @Success      201  {object}  domain.PaymentCaptured
// @Failure      400  {object}  apperr.AppError
// @Failure      404  {object}  apperr.AppError
// @Failure      409  {object}  apperr.AppError
// @Router       /payments [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.InvalidJSON()
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	p, err := h.svc.Create(c.Context(), b.MerchantID, b.Price, c.Get("Idempotency-Key"))
	var (
		ce  *domain.ConflictError
		ecn *domain.ErrorCardNetwork
		se  *saga.SagaError
	)
	switch {
	case errors.Is(err, merchant.ErrNotFound):
		return apperr.MerchantNotFound()
	case errors.Is(err, domain.ErrConflict):
		return apperr.IdempotencyConflict("")
	case errors.As(err, &ce):
		id := ""
		if ce.Existing != nil {
			id = ce.Existing.ID
			c.Set("Idempotency-Key", ce.Existing.IdempotencyKey)
		}
		return apperr.IdempotencyConflict(id)
	case errors.As(err, &ecn):
		return apperr.BadGateway()
	case errors.As(err, &se):
		return apperr.BadGateway()
	case err != nil:
		return apperr.InternalError()
	}

	c.Set("Idempotency-Key", p.IdempotencyKey)
	return c.Status(http.StatusCreated).JSON(p)
}
