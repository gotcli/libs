// Package request parses path, query and body values strictly: malformed input
// is a 400 rather than silently falling back to a default.
package request

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/apperr"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Page struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func (p Page) Offset() int { return (p.Page - 1) * p.PageSize }

// Paged is the list response body for page-numbered endpoints.
type Paged[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

func NewPaged[T any](items []T, page Page, total int64) Paged[T] {
	if items == nil {
		items = []T{}
	}
	return Paged[T]{Items: items, Page: page.Page, PageSize: page.PageSize, Total: total}
}

// ID reads a positive int64 path parameter.
func ID(c *fiber.Ctx, name string) (int64, error) {
	value, err := strconv.ParseInt(c.Params(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, apperr.Invalid("invalid " + name)
	}
	return value, nil
}

// OptionalID reads a positive int64 query parameter; absent means nil.
func OptionalID(c *fiber.Ctx, name string) (*int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, apperr.Invalid("invalid " + name)
	}
	return &value, nil
}

func OptionalBool(c *fiber.Ctx, name string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, apperr.Invalid("invalid " + name)
	}
	return &value, nil
}

func intQuery(c *fiber.Ctx, name string, fallback, min, max int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, apperr.Invalid(name + " must be between " + strconv.Itoa(min) + " and " + strconv.Itoa(max))
	}
	return value, nil
}

func Paging(c *fiber.Ctx) (Page, error) {
	page, err := intQuery(c, "page", 1, 1, 1_000_000)
	if err != nil {
		return Page{}, err
	}
	size, err := intQuery(c, "page_size", DefaultPageSize, 1, MaxPageSize)
	if err != nil {
		return Page{}, err
	}
	return Page{Page: page, PageSize: size}, nil
}

// Limit reads a bounded "limit" query parameter for cursor-paginated lists.
func Limit(c *fiber.Ctx) (int, error) {
	return intQuery(c, "limit", DefaultPageSize, 1, MaxPageSize)
}

func Body(c *fiber.Ctx, out any) error {
	if len(c.Body()) == 0 {
		return apperr.Invalid("request body is required")
	}
	if err := c.BodyParser(out); err != nil {
		return apperr.Invalid("invalid request body")
	}
	return nil
}
