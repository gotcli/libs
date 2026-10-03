package middlewares

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/logger"
)

func RequestLogger(loggers *logger.Loggers) fiber.Handler {
	return func(c *fiber.Ctx) error {
		started := time.Now()
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			var value [16]byte
			_, _ = rand.Read(value[:])
			requestID = hex.EncodeToString(value[:])
		}
		c.Set("X-Request-ID", requestID)
		c.SetUserContext(logger.WithRequestID(c.UserContext(), requestID))
		loggers.Inbound.Info("request received", "event", "http.inbound", "request_id", requestID, "method", c.Method(), "path", c.Path(), "client_ip", c.IP())
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fiberError *fiber.Error
			if errors.As(err, &fiberError) {
				status = fiberError.Code
			}
		}
		attrs := []any{"event", "http.outbound", "request_id", requestID, "method", c.Method(), "path", c.Path(), "status", status, "duration_ms", time.Since(started).Milliseconds()}
		if err != nil {
			loggers.Outbound.Error("request failed", append(attrs, "error", err.Error())...)
			return err
		}
		loggers.Outbound.Info("response sent", attrs...)
		return nil
	}
}
