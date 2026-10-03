// Package httpx is the legacy handler helper used by booking-service and
// tenant-service. New code should use request for parsing and return apperr
// errors mapped with apperr.ToHTTP, which never exposes unknown errors.
package httpx

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/apperr"
	"github.com/gotcli/libs/request"
	"gorm.io/gorm"
)

// ID reads a positive int64 path parameter.
//
// Deprecated: use request.ID.
func ID(c *fiber.Ctx, name string) (int64, error) {
	return request.ID(c, name)
}

// Fail maps a service error to a fiber error: fiber errors pass through,
// conflictErrors become 409, apperr and gorm errors follow apperr.ToHTTP, and
// database errors become a 500 without details, and anything else is treated
// as a validation failure (400 with its message), which is what existing
// callers rely on for plain errors.New validation.
//
// Deprecated: return apperr errors from services and use apperr.ToHTTP.
func Fail(err error, conflictErrors ...error) error {
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		return fiberError
	}
	for _, conflict := range conflictErrors {
		if errors.Is(err, conflict) {
			return fiber.NewError(fiber.StatusConflict, err.Error())
		}
	}
	if mapped := apperr.ToHTTP(err); mapped != err {
		return mapped
	}
	if internal(err) {
		return err
	}
	return fiber.NewError(fiber.StatusBadRequest, err.Error())
}

// internal reports errors whose text describes the database, not the request:
// raw driver errors (anything with a SQLSTATE, such as *pgconn.PgError) and a
// query naming a missing column. They go to the shared handler as a plain 500.
func internal(err error) bool {
	var sqlError interface{ SQLState() string }
	return errors.As(err, &sqlError) || errors.Is(err, gorm.ErrInvalidField)
}
