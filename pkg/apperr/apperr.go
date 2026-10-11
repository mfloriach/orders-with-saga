package apperr

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type AppError struct {
	Status  int
	Code    string
	Title   string
	Details string
	Fields  map[string]any
	Headers map[string]string
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Title)
}

func New(status int, code, title, details string) *AppError {
	return &AppError{
		Status:  status,
		Code:    code,
		Title:   title,
		Details: details,
	}
}

func (e *AppError) WithField(key string, value any) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	e.Fields[key] = value
	return e
}

func (e *AppError) WithHeader(key, value string) *AppError {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	e.Headers[key] = value
	return e
}

func Handler(c *fiber.Ctx, err error) error {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		return fiber.DefaultErrorHandler(c, err)
	}

	status := appErr.Status
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}

	for key, value := range appErr.Headers {
		c.Set(key, value)
	}

	body := fiber.Map{
		"code":    appErr.Code,
		"title":   appErr.Title,
		"details": appErr.Details,
	}

	for key, value := range appErr.Fields {
		body[key] = value
	}

	return c.Status(status).JSON(body)
}
