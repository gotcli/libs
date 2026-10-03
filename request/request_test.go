package request

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/apperr"
)

// run executes fn inside a real fiber request so path and query parsing match
// production behaviour.
func run(t *testing.T, route, target, body string, fn func(*fiber.Ctx) error) error {
	t.Helper()
	var result error
	app := fiber.New()
	app.All(route, func(c *fiber.Ctx) error {
		result = fn(c)
		return nil
	})
	req := httptest.NewRequest(fiber.MethodPost, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	if _, err := app.Test(req, -1); err != nil {
		t.Fatal(err)
	}
	return result
}

func isInvalid(err error) bool {
	var input *apperr.InputError
	return errors.As(err, &input)
}

func TestID(t *testing.T) {
	var got int64
	if err := run(t, "/x/:id", "/x/42", "", func(c *fiber.Ctx) (err error) { got, err = ID(c, "id"); return }); err != nil || got != 42 {
		t.Fatalf("got %d err %v", got, err)
	}
	for _, target := range []string{"/x/0", "/x/-3", "/x/abc", "/x/99999999999999999999"} {
		if err := run(t, "/x/:id", target, "", func(c *fiber.Ctx) error { _, err := ID(c, "id"); return err }); !isInvalid(err) {
			t.Errorf("%s: err %v", target, err)
		}
	}
}

func TestOptionalQueries(t *testing.T) {
	var id *int64
	var flag *bool
	err := run(t, "/", "/?tag_id=7&following=true", "", func(c *fiber.Ctx) (err error) {
		if id, err = OptionalID(c, "tag_id"); err != nil {
			return err
		}
		flag, err = OptionalBool(c, "following")
		return err
	})
	if err != nil || id == nil || *id != 7 || flag == nil || !*flag {
		t.Fatalf("id=%v flag=%v err=%v", id, flag, err)
	}
	err = run(t, "/", "/", "", func(c *fiber.Ctx) (err error) {
		if id, err = OptionalID(c, "tag_id"); err != nil {
			return err
		}
		flag, err = OptionalBool(c, "following")
		return err
	})
	if err != nil || id != nil || flag != nil {
		t.Fatalf("absent values: id=%v flag=%v err=%v", id, flag, err)
	}
	if err := run(t, "/", "/?tag_id=0", "", func(c *fiber.Ctx) error { _, err := OptionalID(c, "tag_id"); return err }); !isInvalid(err) {
		t.Errorf("zero id: %v", err)
	}
	if err := run(t, "/", "/?following=maybe", "", func(c *fiber.Ctx) error { _, err := OptionalBool(c, "following"); return err }); !isInvalid(err) {
		t.Errorf("bad bool: %v", err)
	}
}

func TestPaging(t *testing.T) {
	var page Page
	if err := run(t, "/", "/", "", func(c *fiber.Ctx) (err error) { page, err = Paging(c); return }); err != nil || page != (Page{1, DefaultPageSize}) {
		t.Fatalf("defaults: %+v %v", page, err)
	}
	if err := run(t, "/", "/?page=3&page_size=10", "", func(c *fiber.Ctx) (err error) { page, err = Paging(c); return }); err != nil || page.Offset() != 20 {
		t.Fatalf("page 3: %+v %v", page, err)
	}
	for _, target := range []string{"/?page=0", "/?page_size=101", "/?page=x"} {
		if err := run(t, "/", target, "", func(c *fiber.Ctx) error { _, err := Paging(c); return err }); !isInvalid(err) {
			t.Errorf("%s: %v", target, err)
		}
	}
}

func TestBody(t *testing.T) {
	var input struct {
		Name string `json:"name"`
	}
	if err := run(t, "/", "/", `{"name":"VIP"}`, func(c *fiber.Ctx) error { return Body(c, &input) }); err != nil || input.Name != "VIP" {
		t.Fatalf("name=%q err=%v", input.Name, err)
	}
	for _, body := range []string{"", `{"name":`} {
		if err := run(t, "/", "/", body, func(c *fiber.Ctx) error { return Body(c, &input) }); !isInvalid(err) {
			t.Errorf("body %q: %v", body, err)
		}
	}
}

func TestNewPagedNeverReturnsNullItems(t *testing.T) {
	paged := NewPaged[int](nil, Page{Page: 1, PageSize: 20}, 0)
	if paged.Items == nil {
		t.Fatal("items must be an empty slice")
	}
}
