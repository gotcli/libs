package internalauth

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const Header = "X-Internal-Token"

func ValidateSecret(secret string) error {
	if len(secret) < 32 {
		return errors.New("internal API secret must contain at least 32 bytes")
	}
	return nil
}

func Require(secret string) fiber.Handler {
	if err := ValidateSecret(secret); err != nil {
		panic(err)
	}
	expected := []byte(secret)
	return func(c *fiber.Ctx) error {
		provided := []byte(strings.TrimSpace(c.Get(Header)))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			return fiber.ErrUnauthorized
		}
		return c.Next()
	}
}
