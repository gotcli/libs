package version

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/response"
)

type API string

const (
	V1 API = "v1"

	Current = V1
)

var Supported = []API{V1}

func (v API) String() string {
	return string(v)
}

func (v API) Path() string {
	return fmt.Sprintf("/api/%s", v)
}

// Group creates a versioned API group and registers its version endpoint.
func Group(router fiber.Router, apiVersion API) fiber.Router {
	group := router.Group(apiVersion.Path())
	group.Get("", Handler(apiVersion))
	return group
}

// Handler returns an API-version endpoint using the shared response envelope.
func Handler(apiVersion API) fiber.Handler {
	value := strings.TrimSpace(apiVersion.String())
	return func(c *fiber.Ctx) error {
		return response.Send(c, fiber.StatusOK, value)
	}
}
