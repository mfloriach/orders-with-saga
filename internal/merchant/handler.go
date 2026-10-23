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

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 50
)

// merchantService is the subset of Service the handler needs (stubbed in tests).
type merchantService interface {
	Register(ctx context.Context, name, email string, webhook *string) (*domain.Merchant, error)
	List(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error)
	Get(ctx context.Context, id string) (*domain.Merchant, error)
	Update(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error)
	Delete(ctx context.Context, id string) error
}

type createBody struct {
	Name       string  `json:"name" validate:"required,min=1,max=200"`
	Email      string  `json:"email" validate:"required,email,max=320"`
	WebhookURL *string `json:"webhook_url" validate:"omitempty,http_url,max=2000"`
}

// updateBody uses pointers so an absent field is distinguishable from an empty
// value. Sending webhook_url as an empty string clears it.
type updateBody struct {
	Name       *string `json:"name" validate:"omitempty,min=1,max=200"`
	Email      *string `json:"email" validate:"omitempty,email,max=320"`
	WebhookURL *string `json:"webhook_url" validate:"omitempty,http_url,max=2000"`
}

type idParam struct {
	ID string `param:"id" validate:"required"`
}

type listQuery struct {
	Page  int `query:"page" validate:"gte=1,lte=100"`
	Limit int `query:"limit" validate:"gte=1,lte=50"`
}

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type ListResponse struct {
	Data       []domain.Merchant `json:"data"`
	Pagination Pagination        `json:"pagination"`
}

// Handler serves merchant CRUD endpoints.
type Handler struct{ svc merchantService }

func NewHandler(svc merchantService) *Handler { return &Handler{svc} }

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

// ListMerchants godoc
// @Summary      List merchants
// @Tags         merchants
// @Produce      json
// @Param        page  query int false "Page number" default(1) minimum(1) maximum(100)
// @Param        limit query int false "Page size" default(20) minimum(1) maximum(50)
// @Success      200  {object}  ListResponse
// @Failure      400  {object}  apperr.AppError
// @Router       /merchants [get]
func (h *Handler) List(c *fiber.Ctx) error {
	var q listQuery
	if err := c.QueryParser(&q); err != nil {
		return apperr.ValidationFailed(map[string]string{"query": "malformed query string"})
	}
	if q.Page == 0 {
		q.Page = defaultPage
	}
	if q.Limit == 0 {
		q.Limit = defaultLimit
	}
	if errs := validate.Struct(q); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	merchants, total, err := h.svc.List(c.Context(), q.Limit, (q.Page-1)*q.Limit)
	if err != nil {
		return apperr.InternalError()
	}
	if merchants == nil {
		merchants = []domain.Merchant{}
	}

	return c.JSON(ListResponse{
		Data: merchants,
		Pagination: Pagination{
			Page:       q.Page,
			Limit:      q.Limit,
			TotalItems: total,
			TotalPages: totalPages(total, q.Limit),
		},
	})
}

// GetMerchant godoc
// @Summary      Get a merchant
// @Tags         merchants
// @Produce      json
// @Param        id path string true "Merchant ID"
// @Success      200  {object}  domain.Merchant
// @Failure      400  {object}  apperr.AppError
// @Failure      404  {object}  apperr.AppError
// @Router       /merchants/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := merchantID(c)
	if err != nil {
		return err
	}

	m, err := h.svc.Get(c.Context(), id)
	switch {
	case errors.Is(err, ErrNotFound):
		return apperr.MerchantNotFound()
	case err != nil:
		return apperr.InternalError()
	}

	return c.JSON(m)
}

// UpdateMerchant godoc
// @Summary      Update a merchant
// @Tags         merchants
// @Accept       json
// @Produce      json
// @Param        id   path string     true "Merchant ID"
// @Param        body body updateBody true "Fields to update"
// @Success      200  {object}  domain.Merchant
// @Failure      400  {object}  apperr.AppError
// @Failure      404  {object}  apperr.AppError
// @Failure      409  {object}  apperr.AppError
// @Router       /merchants/{id} [patch]
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := merchantID(c)
	if err != nil {
		return err
	}

	var b updateBody
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&b); err != nil {
			return apperr.InvalidJSON()
		}
	}
	// An empty webhook clears it; validate a view without the empty value.
	toValidate := b
	if toValidate.WebhookURL != nil && *toValidate.WebhookURL == "" {
		toValidate.WebhookURL = nil
	}
	if errs := validate.Struct(toValidate); errs != nil {
		return apperr.ValidationFailed(errs)
	}

	m, err := h.svc.Update(c.Context(), id, b.Name, b.Email, b.WebhookURL)
	switch {
	case errors.Is(err, ErrNotFound):
		return apperr.MerchantNotFound()
	case errors.Is(err, ErrEmailTaken):
		return apperr.EmailTaken()
	case err != nil:
		return apperr.InternalError()
	}

	return c.JSON(m)
}

// DeleteMerchant godoc
// @Summary      Delete a merchant
// @Tags         merchants
// @Param        id path string true "Merchant ID"
// @Success      204  "Merchant deleted"
// @Failure      400  {object}  apperr.AppError
// @Router       /merchants/{id} [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := merchantID(c)
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil && !errors.Is(err, ErrNotFound) {
		return apperr.InternalError()
	}

	return c.SendStatus(http.StatusNoContent)
}

func merchantID(c *fiber.Ctx) (string, error) {
	var p idParam
	if err := c.ParamsParser(&p); err != nil {
		return "", apperr.ValidationFailed(map[string]string{"id": "is required"})
	}
	if errs := validate.Struct(p); errs != nil {
		return "", apperr.ValidationFailed(errs)
	}
	return p.ID, nil
}

func totalPages(total, limit int) int {
	if total == 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
