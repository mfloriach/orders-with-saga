package apperr

import (
	"net/http"
	"strconv"
)

func InternalError() *appError {
	return &appError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Title:   "Internal server error",
		Details: "Something went wrong.",
	}
}

func ValidationFailed(errs map[string]string) *appError {
	err := appError{
		Status:  http.StatusBadRequest,
		Code:    "validation_error",
		Title:   "Validation failed",
		Details: "One or more fields are invalid.",
	}

	return err.withField("errors", errs)
}

func InvalidJSON() *appError {
	err := appError{
		Status:  http.StatusBadRequest,
		Code:    "validation_error",
		Title:   "Invalid request body",
		Details: "Request body is not valid JSON.",
	}

	return err.withField("errors", map[string]string{
		"body": "invalid JSON",
	})
}

func EmailTaken() *appError {
	return &appError{
		Status:  http.StatusConflict,
		Code:    "email_taken",
		Title:   "Email already registered",
		Details: "A merchant with this email already exists.",
	}
}

func IdempotencyConflict(paymentID string) *appError {
	err := appError{
		Status:  http.StatusConflict,
		Code:    "idempotency_conflict",
		Title:   "Duplicate idempotency key",
		Details: "A payment with this idempotency key already exists.",
	}

	return err.
		withField("payment_id", paymentID).
		withHeader("Conflict-Payment-ID", paymentID)
}

func RateLimitedError(retryAfter int) *appError {
	err := appError{
		Status:  http.StatusTooManyRequests,
		Code:    "rate_limited",
		Title:   "Rate limit exceeded",
		Details: "Too many requests.",
	}

	return err.withField("retry_after", retryAfter).
		withHeader("Retry-After", strconv.Itoa(retryAfter))
}

func MerchantNotFound() *appError {
	return &appError{
		Status:  http.StatusNotFound,
		Code:    "merchant_not_found",
		Title:   "Merchant not found",
		Details: "No merchant exists with the given ID.",
	}
}

func BadGateway() *appError {
	return &appError{
		Status:  http.StatusBadGateway,
		Code:    "bad_gateway",
		Title:   "Upstream error",
		Details: "The card network or another upstream service is unavailable.",
	}
}

func NotFound() *appError {
	return &appError{
		Status:  http.StatusNotFound,
		Code:    "not_found",
		Title:   "Resource not found",
		Details: "The requested resource was not found.",
	}
}
