package bootstrap

import (
	"context"
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/response"
	"gorm.io/gorm"
)

// Pinger is satisfied by *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// RegisterHealthRoutes mounts /health/live (process is up) and /health/ready
// (database answers a ping within two seconds).
func RegisterHealthRoutes(app fiber.Router, db *gorm.DB) {
	registerHealthRoutes(app, func() (Pinger, error) {
		sqlDB, err := db.DB()
		return sqlDB, err
	})
}

func registerHealthRoutes(app fiber.Router, pool func() (Pinger, error)) {
	app.Get("/health/live", func(c *fiber.Ctx) error {
		return response.Send(c, fiber.StatusOK, map[string]string{"state": "alive"})
	})
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		pinger, err := pool()
		if err != nil || pinger == nil {
			return fiber.NewError(fiber.StatusServiceUnavailable, "database unavailable")
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()
		if err := pinger.PingContext(ctx); err != nil {
			return fiber.NewError(fiber.StatusServiceUnavailable, "database unavailable")
		}
		return response.Send(c, fiber.StatusOK, map[string]string{"state": "ready"})
	})
}

var _ Pinger = (*sql.DB)(nil)
