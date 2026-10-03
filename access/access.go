// Package access enforces permissions carried in the access token. Each
// service checks codes from the shared catalogue seeded by
// database/init/006_permissions_roles.sql, e.g. "contact.view".
package access

import (
	"github.com/gofiber/fiber/v2"
	sharedauth "github.com/gotcli/libs/auth"
)

// OwnerRole bypasses permission checks within its own workspace.
const OwnerRole = "owner"

// Require allows the request when the caller has a workspace and either holds
// the permission or is the workspace owner. It must run after auth.RequireJWT.
func Require(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := sharedauth.WorkspaceClaimsFrom(c)
		if err != nil {
			return fiber.NewError(fiber.StatusForbidden, "workspace context is required")
		}
		if !claims.HasRole(OwnerRole) && !claims.HasPermission(permission) {
			return fiber.NewError(fiber.StatusForbidden, "missing permission "+permission)
		}
		return c.Next()
	}
}
