package httptransport

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"example.com/rolescout/api/internal/domain"
)

// errorBody is the JSON shape of every error response:
//
//	{"error": {"code": "invalid_input", "message": "...", "fields": [...]}}
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Fields  []domain.FieldError `json:"fields,omitempty"`
}

// badRequestError marks malformed requests, such as unparsable JSON. Its
// message is shown to the client.
type badRequestError struct{ message string }

func (e *badRequestError) Error() string { return e.message }

// conflictError is a 409 with a specific message for the client.
type conflictError struct{ message string }

func (e *conflictError) Error() string { return e.message }

func badRequest(format string, args ...any) error {
	return &badRequestError{message: fmt.Sprintf(format, args...)}
}

// ErrorHandler renders handler and framework errors in the API error format.
// Unexpected errors are logged and returned as a generic 500.
func ErrorHandler(c fiber.Ctx, err error) error {
	status, detail := classify(err)
	if status == fiber.StatusInternalServerError {
		slog.ErrorContext(c.Context(), "unexpected error",
			"request_id", requestid.FromContext(c), "method", c.Method(), "path", c.Path(), "error", err)
	}
	return c.Status(status).JSON(errorBody{Error: detail})
}

func classify(err error) (int, errorDetail) {
	var validation *domain.ValidationError
	var transition domain.ErrTransitionNotAllowed
	var fiberErr *fiber.Error
	var badReq *badRequestError
	var conflict *conflictError
	switch {
	case errors.As(err, &conflict):
		return fiber.StatusConflict, errorDetail{Code: "conflict", Message: conflict.message}
	case errors.As(err, &validation):
		return fiber.StatusUnprocessableEntity, errorDetail{
			Code: "invalid_input", Message: "The request contains invalid fields.", Fields: validation.Fields,
		}
	case errors.As(err, &transition):
		return fiber.StatusConflict, errorDetail{Code: "invalid_transition", Message: transition.Error()}
	case errors.Is(err, errUnauthenticated):
		return fiber.StatusUnauthorized, errorDetail{Code: "unauthenticated", Message: "Sign in to continue."}
	case errors.Is(err, errInvalidCredentials):
		return fiber.StatusUnauthorized, errorDetail{Code: "invalid_credentials", Message: "The email or password is incorrect."}
	case errors.Is(err, domain.ErrRegistrationClosed):
		return fiber.StatusForbidden, errorDetail{Code: "registration_closed", Message: "Registration is disabled on this server."}
	case errors.Is(err, domain.ErrForbidden):
		return fiber.StatusForbidden, errorDetail{Code: "forbidden", Message: "You are not allowed to perform this action."}
	case errors.Is(err, domain.ErrNotFound):
		return fiber.StatusNotFound, errorDetail{Code: "not_found", Message: "The resource was not found."}
	case errors.Is(err, domain.ErrConflict):
		return fiber.StatusConflict, errorDetail{Code: "conflict", Message: "The resource already exists."}
	case errors.Is(err, domain.ErrInvalid):
		// A database constraint rejected the input without field details.
		return fiber.StatusUnprocessableEntity, errorDetail{Code: "invalid_input", Message: "The request contains invalid data."}
	case errors.As(err, &badReq):
		return fiber.StatusBadRequest, errorDetail{Code: "bad_request", Message: badReq.message}
	case errors.As(err, &fiberErr):
		return fiberErr.Code, errorDetail{Code: fiberCode(fiberErr.Code), Message: fiberErr.Message}
	default:
		return fiber.StatusInternalServerError, errorDetail{Code: "internal", Message: "An unexpected error occurred."}
	}
}

func fiberCode(status int) string {
	switch status {
	case fiber.StatusNotFound:
		return "not_found"
	case fiber.StatusMethodNotAllowed:
		return "method_not_allowed"
	case fiber.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case fiber.StatusBadRequest:
		return "bad_request"
	case fiber.StatusTooManyRequests:
		return "rate_limited"
	case fiber.StatusPreconditionFailed:
		return "ai_disabled"
	case fiber.StatusServiceUnavailable:
		return "ai_unavailable"
	case fiber.StatusBadGateway:
		return "ai_failed"
	case fiber.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	default:
		if status >= 500 {
			return "internal"
		}
		return "error"
	}
}
