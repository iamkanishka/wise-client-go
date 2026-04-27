package wise

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for well-known Wise API conditions.
// Use errors.Is or the helper predicates (IsNotFound, etc.) to match them.
var (
	// ErrUnauthorized is returned when the API responds with HTTP 401.
	ErrUnauthorized = errors.New("wise: unauthorized — invalid or expired token")

	// ErrForbidden is returned on HTTP 403.
	// When code is SCA_REQUIRED, IsSCARequired also returns true.
	ErrForbidden = errors.New("wise: forbidden — check permissions or SCA requirements")

	// ErrNotFound is returned when the requested resource does not exist (HTTP 404).
	ErrNotFound = errors.New("wise: resource not found")

	// ErrConflict is returned on HTTP 409 (e.g. idempotency key reuse with different body).
	ErrConflict = errors.New("wise: conflict")

	// ErrUnprocessable is returned on HTTP 422 — typically a validation failure.
	ErrUnprocessable = errors.New("wise: unprocessable entity")

	// ErrRateLimited is returned on HTTP 429 — too many requests.
	ErrRateLimited = errors.New("wise: rate limited")

	// ErrServerError is returned for HTTP 5xx responses.
	ErrServerError = errors.New("wise: server error")

	// ErrSCARequired signals that Strong Customer Authentication is needed.
	// The endpoint returned HTTP 403 with code "SCA_REQUIRED".
	ErrSCARequired = errors.New("wise: strong customer authentication required")

	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("wise: circuit breaker open — too many recent failures")

	// ErrInvalidWebhookSignature is returned when a webhook HMAC signature does not match.
	ErrInvalidWebhookSignature = errors.New("wise: invalid webhook signature")
)

// APIError is the structured error returned when the Wise API responds with
// a non-2xx HTTP status. It implements error and can be unwrapped to match
// the sentinel errors above via errors.Is / errors.As.
type APIError struct {
	// StatusCode is the HTTP response status code.
	StatusCode int `json:"-"`

	// Status is the HTTP status text (e.g. "Bad Request").
	Status string `json:"-"`

	// Code is the Wise machine-readable error code.
	Code string `json:"code,omitempty"`

	// Message is the human-readable error description.
	Message string `json:"message,omitempty"`

	// Errors contains per-field validation errors (HTTP 422 responses).
	Errors []FieldError `json:"errors,omitempty"`

	// RequestID is the X-Request-Id from the response, useful for Wise support escalation.
	RequestID string `json:"-"`
}

// FieldError represents a validation failure on a specific request field.
type FieldError struct {
	// Field is the JSON path of the invalid field.
	Field string `json:"field"`

	// Code is the machine-readable error code for this field.
	Code string `json:"code"`

	// Message is the human-readable description.
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	msg := fmt.Sprintf("wise: HTTP %d %s", e.StatusCode, e.Status)
	if e.Code != "" {
		msg += " — " + e.Code
	}

	if e.Message != "" {
		msg += ": " + e.Message
	}

	if e.RequestID != "" {
		msg += " (request-id: " + e.RequestID + ")"
	}

	return msg
}

// Unwrap enables errors.Is / errors.As to match sentinel errors.
func (e *APIError) Unwrap() error {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized

	case http.StatusForbidden:
		if e.Code == "SCA_REQUIRED" {
			return ErrSCARequired
		}

		return ErrForbidden

	case http.StatusNotFound:
		return ErrNotFound

	case http.StatusConflict:
		return ErrConflict

	case http.StatusUnprocessableEntity:
		return ErrUnprocessable

	case http.StatusTooManyRequests:
		return ErrRateLimited
	}

	if e.StatusCode >= http.StatusInternalServerError {
		return ErrServerError
	}

	return nil
}

// IsNotFound reports whether err is a 404 Not Found API error.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsUnauthorized reports whether err is a 401 Unauthorized API error.
func IsUnauthorized(err error) bool { return errors.Is(err, ErrUnauthorized) }

// IsRateLimited reports whether err is a 429 Too Many Requests error.
func IsRateLimited(err error) bool { return errors.Is(err, ErrRateLimited) }

// IsSCARequired reports whether the operation requires Strong Customer Authentication.
func IsSCARequired(err error) bool { return errors.Is(err, ErrSCARequired) }

// IsServerError reports whether err is a 5xx server-side error.
func IsServerError(err error) bool { return errors.Is(err, ErrServerError) }

// FieldErrors extracts per-field validation errors from an APIError.
// Returns nil if err is not an *APIError or carries no field errors.
func FieldErrors(err error) []FieldError {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Errors
	}

	return nil
}

// parseAPIError deserialises the response body into an *APIError.
func parseAPIError(resp *http.Response, body []byte) *APIError {
	ae := &APIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Code:       "",
		Message:    "",
		Errors:     nil,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	// Wise returns several error envelope shapes — try to decode all of them.
	var envelope struct {
		Code    string       `json:"code"`
		Message string       `json:"message"`
		Error   string       `json:"error"`
		Errors  []FieldError `json:"errors"`
	}

	if jsonErr := json.Unmarshal(body, &envelope); jsonErr == nil {
		ae.Code = envelope.Code
		ae.Message = envelope.Message

		if ae.Message == "" {
			ae.Message = envelope.Error
		}

		ae.Errors = envelope.Errors
	} else {
		ae.Message = string(body)
	}

	return ae
}

// MockAPIError builds an *APIError suitable for returning from mock handlers.
//
//	mock.Transfers.OnFund = func(...) (*FundResponse, error) {
//	    return nil, wise.MockAPIError(403, "SCA_REQUIRED", "Authentication required")
//	}
//
// Use MockNotFoundError and MockValidationError for common scenarios.
func MockAPIError(statusCode int, code, message string) error {
	return &APIError{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d", statusCode),
		Code:       code,
		Message:    message,
		Errors:     nil,
		RequestID:  "",
	}
}

// MockNotFoundError returns a 404 Not Found APIError for use in tests.
func MockNotFoundError(message string) error {
	return MockAPIError(http.StatusNotFound, "NOT_FOUND", message)
}

// MockValidationError returns a 422 Unprocessable Entity APIError with field errors.
func MockValidationError(fields ...FieldError) error {
	return &APIError{
		StatusCode: http.StatusUnprocessableEntity,
		Status:     "422",
		Code:       "VALIDATION_ERROR",
		Message:    "Validation failed",
		Errors:     fields,
		RequestID:  "",
	}
}

// MockFieldError builds a single FieldError for use with MockValidationError.
func MockFieldError(field, code, message string) FieldError {
	return FieldError{Field: field, Code: code, Message: message}
}
