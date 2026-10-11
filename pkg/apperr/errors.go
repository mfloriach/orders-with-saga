package apperr

import (
	"net/http"
	"strconv"
)

func InternalError() *AppError {
	return New(
		http.StatusInternalServerError,
		"internal_error",
		"Internal server error",
		"Something went wrong.",
	)
}

func ValidationFailed(errs map[string]string) *AppError {
	return New(
		http.StatusBadRequest,
		"validation_error",
		"Validation failed",
		"One or more fields are invalid.",
	).WithField("errors", errs)
}

func InvalidJSON() *AppError {
	return New(
		http.StatusBadRequest,
		"validation_error",
		"Invalid request body",
		"Request body is not valid JSON.",
	).WithField("errors", map[string]string{
		"body": "invalid JSON",
	})
}

func EmailTaken() *AppError {
	return New(
		http.StatusConflict,
		"email_taken",
		"Email already registered",
		"A merchant with this email already exists.",
	)
}

func IdempotencyConflict(paymentID string) *AppError {
	return New(
		http.StatusConflict,
		"idempotency_conflict",
		"Duplicate idempotency key",
		"A payment with this idempotency key already exists.",
	).
		WithField("payment_id", paymentID).
		WithHeader("Conflict-Payment-ID", paymentID)
}

func RateLimitedError(retryAfter int) *AppError {
	return New(
		http.StatusTooManyRequests,
		"rate_limited",
		"Rate limit exceeded",
		"Too many requests.",
	).
		WithField("retry_after", retryAfter).
		WithHeader("Retry-After", strconv.Itoa(retryAfter))
}

func MerchantNotFound() *AppError {
	return New(
		http.StatusNotFound,
		"merchant_not_found",
		"Merchant not found",
		"No merchant exists with the given ID.",
	)
}

func NotFound() *AppError {
	return New(
		http.StatusNotFound,
		"not_found",
		"Resource not found",
		"The requested resource was not found.",
	)
}
