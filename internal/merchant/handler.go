package merchant

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"paymentgateway/pkg/apperr"
	"paymentgateway/pkg/validate"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

type createBody struct {
	Name       string  `json:"name" validate:"required,min=1,max=200"`
	Email      string  `json:"email" validate:"required,email,max=320"`
	WebhookURL *string `json:"webhook_url" validate:"omitempty,http_url,max=2000"`
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.Write(c, 400, "validation_error", map[string]string{"body": "invalid JSON"})
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.Write(c, 400, "validation_error", errs)
	}
	m, err := h.svc.Register(c.Context(), b.Name, b.Email, b.WebhookURL)
	if errors.Is(err, ErrEmailTaken) {
		return apperr.Write(c, 409, "email_taken", nil)
	}
	if err != nil {
		return apperr.Write(c, 500, "internal_error", nil)
	}
	return c.Status(201).JSON(m)
}
