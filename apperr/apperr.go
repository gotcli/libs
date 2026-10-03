// Package apperr defines the errors services return and how they map to HTTP.
package apperr

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// InputError carries a message that is safe to return to the client.
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

func Invalid(message string) error { return &InputError{Message: message} }

// NotFoundError names the missing resource, e.g. "contact not found".
type NotFoundError struct{ Resource string }

func (e *NotFoundError) Error() string { return e.Resource + " not found" }

func NotFound(resource string) error { return &NotFoundError{Resource: resource} }

// ConflictError reports a uniqueness or state conflict with a client-safe message.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

func Conflict(message string) error { return &ConflictError{Message: message} }

// ToHTTP converts service and database errors to client-facing fiber errors.
// Anything unrecognised is returned unchanged so the shared error handler
// answers 500 without leaking internal details.
func ToHTTP(err error) error {
	var input *InputError
	var notFound *NotFoundError
	var conflict *ConflictError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &input):
		return fiber.NewError(fiber.StatusBadRequest, input.Message)
	case errors.As(err, &notFound):
		return fiber.NewError(fiber.StatusNotFound, notFound.Error())
	case errors.As(err, &conflict):
		return fiber.NewError(fiber.StatusConflict, conflict.Message)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return fiber.NewError(fiber.StatusNotFound, "resource not found")
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return fiber.NewError(fiber.StatusConflict, "resource already exists")
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return fiber.NewError(fiber.StatusConflict, "resource is referenced by other records")
	case errors.Is(err, gorm.ErrCheckConstraintViolated):
		return fiber.NewError(fiber.StatusBadRequest, "invalid value")
	default:
		return err
	}
}
