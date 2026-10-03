package bootstrap

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// corsMiddleware lets the listed browser origins (e.g. the admin app at
// http://localhost:4010) call the service directly. Each origin must be an
// exact scheme://host[:port]; wildcards are rejected because tokens travel in
// the Authorization header.
func corsMiddleware(origins []string) (fiber.Handler, error) {
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.Contains(u.Host, "*") ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("invalid CORS origin %q: want scheme://host[:port]", origin)
		}
	}
	return cors.New(cors.Config{
		AllowOrigins: strings.Join(origins, ","),
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Accept,Authorization,Content-Type",
		MaxAge:       600,
	}), nil
}
