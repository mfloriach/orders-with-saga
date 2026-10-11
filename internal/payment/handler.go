package payment

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"paymentgateway/internal/merchant"
	"paymentgateway/pkg/apperr"
	"paymentgateway/pkg/validate"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

type createBody struct {
	MerchantID     string `json:"merchant_id" validate:"required,uuid"`
	Amount         int64  `json:"amount" validate:"gt=0"`
	IdempotencyKey string `json:"idempotency_key" validate:"required,min=1,max=200"`
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.Write(c, 400, "validation_error", map[string]string{"body": "invalid JSON"})
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.Write(c, 400, "validation_error", errs)
	}
	p, err := h.svc.Create(c.Context(), b.MerchantID, b.Amount, b.IdempotencyKey)
	if errors.Is(err, merchant.ErrNotFound) {
		return apperr.Write(c, 404, "merchant_not_found", nil)
	}
	var ce *ConflictError
	if errors.As(err, &ce) {
		id := ""
		if ce.Existing != nil {
			id = ce.Existing.ID
		}
		c.Set("Conflict-Payment-ID", id)
		return c.Status(409).JSON(fiber.Map{"code": "idempotency_conflict", "payment_id": id})
	}
	if err != nil {
		return apperr.Write(c, 500, "internal_error", nil)
	}
	return c.Status(201).JSON(p)
}
