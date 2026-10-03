package bootstrap

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gotcli/libs/database"
	"github.com/spf13/viper"
)

type fakePinger struct{ err error }

func (f fakePinger) PingContext(context.Context) error { return f.err }

func healthStatus(t *testing.T, pool func() (Pinger, error), path string) int {
	t.Helper()
	app := fiber.New()
	registerHealthRoutes(app, pool)
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func TestHealthRoutes(t *testing.T) {
	ok := func() (Pinger, error) { return fakePinger{}, nil }
	down := func() (Pinger, error) { return fakePinger{err: errors.New("refused")}, nil }
	missing := func() (Pinger, error) { return nil, errors.New("no pool") }
	if got := healthStatus(t, down, "/health/live"); got != fiber.StatusOK {
		t.Errorf("live = %d", got)
	}
	if got := healthStatus(t, ok, "/health/ready"); got != fiber.StatusOK {
		t.Errorf("ready = %d", got)
	}
	if got := healthStatus(t, down, "/health/ready"); got != fiber.StatusServiceUnavailable {
		t.Errorf("ready with failing ping = %d", got)
	}
	if got := healthStatus(t, missing, "/health/ready"); got != fiber.StatusServiceUnavailable {
		t.Errorf("ready without pool = %d", got)
	}
}

func TestNewAppRecoversPanics(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("HTTP_BODY_LIMIT", 1024)
	app := NewApp(nil)
	app.Get("/boom", func(*fiber.Ctx) error { panic("boom") })
	res, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/boom", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

// Startup must fail before touching the database when configuration is bad.
func TestRunFailsFast(t *testing.T) {
	register := func(*fiber.App, Deps) error { return nil }
	db := func() database.Config {
		return database.Config{Host: "localhost", Port: 5432, Name: "app", Username: "app"}
	}
	mod := "github.com/gotcli/test-service"
	tests := map[string]struct {
		options Options
		env     map[string]string
		want    string
	}{
		"missing register": {Options{Module: mod, Database: db}, nil, "Register are required"},
		"missing database": {Options{Module: mod, Register: register}, nil, "Database and Register are required"},
		"empty database config": {Options{Module: mod, Register: register,
			Database: func() database.Config { return database.Config{} }}, nil, "host is required"},
		"bad config": {Options{Module: mod, Register: register, Database: db},
			map[string]string{"TEST_SERVICE_APP_PORT": "0"}, "APP_PORT"},
		"service validation": {Options{Module: mod, Register: register, Database: db,
			Validate: func() error { return errors.New("INTERNAL_API_SECRET is required") }}, nil, "INTERNAL_API_SECRET"},
		"weak JWT secret": {Options{Module: mod, Register: register, Database: db, RequireJWT: true},
			map[string]string{"TEST_SERVICE_JWT_ACCESS_SECRET": "short"}, "JWT verifier"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			t.Chdir(t.TempDir())
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			err := run(test.options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
			if Run(test.options) != 1 {
				t.Fatal("Run must return exit code 1")
			}
		})
	}
}
