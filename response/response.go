package response

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

type Envelope struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func Send(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(Envelope{Status: status, Message: http.StatusText(status), Data: data})
}

func SendMessage(c *fiber.Ctx, status int, message string, data any) error {
	if message == "" {
		message = http.StatusText(status)
	}
	return c.Status(status).JSON(Envelope{Status: status, Message: message, Data: data})
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := http.StatusText(status)
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		status = fiberError.Code
		message = fiberError.Message
	}
	return c.Status(status).JSON(Envelope{Status: status, Message: message, Data: nil})
}
