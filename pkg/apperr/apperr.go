package apperr

import (
	"errors"
	"fmt"
	"net/http"

	"paymentgateway/pkg/logging"

	"github.com/gofiber/fiber/v2"
)

type appError struct {
	Status  int               `json:"status"`
	Code    string            `json:"code"`
	Title   string            `json:"title"`
	Details string            `json:"details,omitempty"`
	Fields  map[string]any    `json:"fields,omitempty"`
	Headers map[string]string `json:"-"`
}

func (e *appError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Title)
}

func (e *appError) withField(key string, value any) *appError {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	e.Fields[key] = value
	return e
}

func (e *appError) withHeader(key, value string) *appError {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	e.Headers[key] = value
	return e
}

func Handler(c *fiber.Ctx, err error) error {
	var appErr *appError
	if !errors.As(err, &appErr) {
		status := http.StatusInternalServerError
		var fe *fiber.Error
		if errors.As(err, &fe) {
			status = fe.Code
		}
		logging.LogErrorResponse(c, status)
		return fiber.DefaultErrorHandler(c, err)
	}

	status := appErr.Status
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	logging.LogErrorResponse(c, status)

	for key, value := range appErr.Headers {
		c.Set(key, value)
	}

	body := fiber.Map{
		"code":    appErr.Code,
		"title":   appErr.Title,
		"details": appErr.Details,
	}

	if id := logging.RequestIDFrom(c.UserContext()); id != "" {
		body["request_id"] = id
	}

	for key, value := range appErr.Fields {
		body[key] = value
	}

	return c.Status(status).JSON(body)
}
