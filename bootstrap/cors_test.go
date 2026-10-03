package bootstrap

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestCORSMiddleware(t *testing.T) {
	handler, err := corsMiddleware([]string{"http://localhost:4010"})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(handler)
	// Stands in for RequireJWT: preflight must be answered before it.
	app.Use(func(c *fiber.Ctx) error { return fiber.ErrUnauthorized })

	for _, test := range []struct {
		origin, allowed string
		status          int
	}{
		{"http://localhost:4010", "http://localhost:4010", fiber.StatusNoContent},
		{"http://evil.localhost:4010", "", fiber.StatusNoContent},
	} {
		request := httptest.NewRequest(fiber.MethodOptions, "/api/v1/auth/me", nil)
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Access-Control-Request-Method", "GET")
		request.Header.Set("Access-Control-Request-Headers", "authorization")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		if got := response.Header.Get("Access-Control-Allow-Origin"); response.StatusCode != test.status || got != test.allowed {
			t.Errorf("origin %s: status %d, allow-origin %q; want %d, %q", test.origin, response.StatusCode, got, test.status, test.allowed)
		}
	}
}

func TestCORSMiddlewareRejectsBadOrigins(t *testing.T) {
	for _, origin := range []string{"*", "localhost:4010", "http://*.localhost", "http://localhost:4010/app", "ftp://localhost"} {
		if _, err := corsMiddleware([]string{origin}); err == nil {
			t.Errorf("corsMiddleware(%q) accepted a bad origin", origin)
		}
	}
}
