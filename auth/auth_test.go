package auth

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func signedAccessToken(t *testing.T, secret string, workspaceID int64, roles []string) string {
	t.Helper()
	now := time.Now().UTC()
	claims := Claims{
		UserID: 42, WorkspaceID: workspaceID, Roles: roles, TokenType: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "identity", Audience: jwt.ClaimStrings{"lineoa"}, Subject: "user:42",
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestRequireJWTProvidesWorkspaceContext(t *testing.T) {
	secret := strings.Repeat("a", 32)
	verifier, _ := NewVerifier("identity", "lineoa", secret)
	app := fiber.New()
	app.Get("/protected", RequireJWT(verifier), func(c *fiber.Ctx) error {
		claims, ok := ClaimsFromContext(c.UserContext())
		if !ok || claims.WorkspaceID != 7 {
			return fiber.ErrInternalServerError
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	request := httptest.NewRequest("GET", "/protected", nil)
	request.Header.Set(fiber.HeaderAuthorization, "Bearer "+signedAccessToken(t, secret, 7, []string{"owner"}))
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
	}
}

func TestRequireJWTRejectsMissingWorkspace(t *testing.T) {
	secret := strings.Repeat("a", 32)
	verifier, _ := NewVerifier("identity", "lineoa", secret)
	app := fiber.New()
	app.Get("/protected", RequireJWT(verifier), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	request := httptest.NewRequest("GET", "/protected", nil)
	request.Header.Set(fiber.HeaderAuthorization, "Bearer "+signedAccessToken(t, secret, 0, []string{"user"}))
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusForbidden)
	}
}

func TestVerifierParsesWorkspaceClaims(t *testing.T) {
	secret := strings.Repeat("a", 32)
	verifier, err := NewVerifier("identity", "lineoa", secret)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	claims, err := verifier.ParseAccess(signedAccessToken(t, secret, 7, []string{"owner"}))
	if err != nil {
		t.Fatalf("ParseAccess() error = %v", err)
	}
	if claims.UserID != 42 || claims.WorkspaceID != 7 || !claims.HasRole("owner") {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestVerifierRejectsWrongSecret(t *testing.T) {
	verifier, _ := NewVerifier("identity", "lineoa", strings.Repeat("b", 32))
	if _, err := verifier.ParseAccess(signedAccessToken(t, strings.Repeat("a", 32), 7, nil)); err == nil {
		t.Fatal("ParseAccess() accepted a token signed with another secret")
	}
}
