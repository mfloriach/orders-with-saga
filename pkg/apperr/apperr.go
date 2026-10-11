package apperr

import "github.com/gofiber/fiber/v2"

type APIError struct {
	Code   string            `json:"code"`
	Errors map[string]string `json:"errors,omitempty"`
}

func Write(c *fiber.Ctx, status int, code string, errs map[string]string) error {
	return c.Status(status).JSON(APIError{Code: code, Errors: errs})
}
