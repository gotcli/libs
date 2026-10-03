package internalauth

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequire(t *testing.T) {
	secret := "01234567890123456789012345678901"
	app := fiber.New()
	app.Get("/", Require(secret), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	request := httptest.NewRequest("GET", "/", nil)
	response, _ := app.Test(request)
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("without token status = %d", response.StatusCode)
	}

	request = httptest.NewRequest("GET", "/", nil)
	request.Header.Set(Header, secret)
	response, _ = app.Test(request)
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("valid token status = %d", response.StatusCode)
	}
}

func TestValidateSecret(t *testing.T) {
	if ValidateSecret("short") == nil {
		t.Fatal("expected short secret rejection")
	}
}
