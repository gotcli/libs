package access

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	sharedauth "github.com/gotcli/libs/auth"
)

func status(t *testing.T, claims *sharedauth.Claims) int {
	t.Helper()
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if claims != nil {
			c.Locals(sharedauth.ClaimsLocal, claims)
		}
		return c.Next()
	}, Require("contacts.write"), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func TestRequire(t *testing.T) {
	tests := []struct {
		name   string
		claims *sharedauth.Claims
		want   int
	}{
		{"no claims", nil, fiber.StatusForbidden},
		{"no workspace", &sharedauth.Claims{UserID: 1, Roles: []string{OwnerRole}}, fiber.StatusForbidden},
		{"missing permission", &sharedauth.Claims{UserID: 1, WorkspaceID: 2, Permissions: []string{"contacts.read"}}, fiber.StatusForbidden},
		{"has permission", &sharedauth.Claims{UserID: 1, WorkspaceID: 2, Permissions: []string{"contacts.write"}}, fiber.StatusNoContent},
		{"owner", &sharedauth.Claims{UserID: 1, WorkspaceID: 2, Roles: []string{OwnerRole}}, fiber.StatusNoContent},
	}
	for _, test := range tests {
		if got := status(t, test.claims); got != test.want {
			t.Errorf("%s: status %d, want %d", test.name, got, test.want)
		}
	}
}
