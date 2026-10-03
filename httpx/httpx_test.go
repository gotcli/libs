package httpx

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/apperr"
	"gorm.io/gorm"
)

func TestFail(t *testing.T) {
	conflict := errors.New("conflict")
	tests := []struct {
		name    string
		err     error
		want    int
		message string
	}{
		{name: "not found", err: gorm.ErrRecordNotFound, want: fiber.StatusNotFound},
		{name: "conflict", err: conflict, want: fiber.StatusConflict, message: "conflict"},
		{name: "legacy validation", err: errors.New("invalid"), want: fiber.StatusBadRequest, message: "invalid"},
		{name: "apperr input", err: apperr.Invalid("name is required"), want: fiber.StatusBadRequest, message: "name is required"},
		{name: "apperr not found", err: apperr.NotFound("booking"), want: fiber.StatusNotFound, message: "booking not found"},
		{name: "fiber error", err: fiber.ErrForbidden, want: fiber.StatusForbidden},
	}
	for _, err := range []error{
		&sqlStateError{"42P01", `relation "invitations" does not exist`},
		fmt.Errorf("list invitations: %w", &sqlStateError{"42703", `column "status" does not exist`}),
		gorm.ErrInvalidField,
	} {
		if got := Fail(err); got != err {
			t.Errorf("Fail(%v) = %#v, want the error unchanged so the handler answers 500", err, got)
		}
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := Fail(test.err, conflict).(*fiber.Error)
			if !ok || got.Code != test.want || (test.message != "" && got.Message != test.message) {
				t.Fatalf("error = %#v", got)
			}
		})
	}
}

// sqlStateError stands in for *pgconn.PgError without importing the driver.
type sqlStateError struct{ code, message string }

func (e *sqlStateError) Error() string    { return "ERROR: " + e.message + " (SQLSTATE " + e.code + ")" }
func (e *sqlStateError) SQLState() string { return e.code }
