package version

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/response"
)

func TestHandler(t *testing.T) {
	app := fiber.New()
	Group(app, V1)

	result, err := app.Test(httptest.NewRequest("GET", "/api/v1", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()

	if result.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d", result.StatusCode)
	}
	var body response.Envelope
	if err := json.NewDecoder(result.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Data != V1.String() {
		t.Fatalf("version = %v", body.Data)
	}
}

func TestVersionPath(t *testing.T) {
	if got := V1.Path(); got != "/api/v1" {
		t.Fatalf("path = %q", got)
	}
}
