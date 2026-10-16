package merchant

import (
	"context"
	"errors"
	"net/http"

	"paymentgateway/internal/domain"
	"paymentgateway/pkg/apperr"
	"paymentgateway/pkg/validate"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ svc registerer }

// registerer is the subset of Service the handler needs (stubbed in tests).
type registerer interface {
	Register(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error)
}

func NewHandler(svc registerer) *Handler { return &Handler{svc} }

type createBody struct {
	Name       string  `json:"name" validate:"required,min=1,max=200"`
	Email      string  `json:"email" validate:"required,email,max=320"`
	WebhookURL *string `json:"webhook_url" validate:"omitempty,http_url,max=2000"`
}

// CreateMerchant godoc
// @Summary      Create a merchant
// @Tags         merchants
// @Accept       json
// @Produce      json
// @Param        body body createBody true "Merchant"
// @Success      201  {object}  domain.Merchant
// @Failure      400  {object}  apperr.AppError
// @Failure      409  {object}  apperr.AppError
// @Router       /merchants [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	var b createBody
	if err := c.BodyParser(&b); err != nil {
		return apperr.InvalidJSON()
	}
	if errs := validate.Struct(b); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	m, err := h.svc.Register(c.Context(), b.Name, b.Email, b.WebhookURL)
	switch {
	case errors.Is(err, ErrEmailTaken):
		return apperr.EmailTaken()
	case err != nil:
		return apperr.InternalError()
	}

	return c.Status(http.StatusCreated).JSON(m)
}
