package apperr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func TestToHTTP(t *testing.T) {
	tests := []struct {
		err     error
		status  int
		message string
	}{
		{Invalid("name is required"), fiber.StatusBadRequest, "name is required"},
		{fmt.Errorf("wrapped: %w", NotFound("contact")), fiber.StatusNotFound, "contact not found"},
		{Conflict("tag name already exists"), fiber.StatusConflict, "tag name already exists"},
		{gorm.ErrRecordNotFound, fiber.StatusNotFound, "resource not found"},
		{gorm.ErrDuplicatedKey, fiber.StatusConflict, "resource already exists"},
		{gorm.ErrForeignKeyViolated, fiber.StatusConflict, "resource is referenced by other records"},
		{gorm.ErrCheckConstraintViolated, fiber.StatusBadRequest, "invalid value"},
	}
	for _, test := range tests {
		var fiberErr *fiber.Error
		if !errors.As(ToHTTP(test.err), &fiberErr) || fiberErr.Code != test.status || fiberErr.Message != test.message {
			t.Errorf("ToHTTP(%v) = %v, want %d %q", test.err, fiberErr, test.status, test.message)
		}
	}
}

func TestToHTTPKeepsInternalErrorsOpaque(t *testing.T) {
	internal := errors.New("pq: connection reset")
	if got := ToHTTP(internal); got != internal {
		t.Fatalf("internal errors must pass through for the 500 handler, got %v", got)
	}
	if ToHTTP(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
