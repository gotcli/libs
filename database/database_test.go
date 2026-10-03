package database

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDSNEscapesCredentials(t *testing.T) {
	cfg := Config{Host: "db.internal", Port: 6543, Name: "lineOA_db", Username: "app user", Password: `p@ss word/"'`, SSLMode: "disable"}
	parsed, err := url.Parse(cfg.DSN())
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if parsed.User.Username() != "app user" || password != `p@ss word/"'` || parsed.Host != "db.internal:6543" ||
		parsed.Path != "/lineOA_db" || parsed.Query().Get("sslmode") != "disable" {
		t.Fatalf("unexpected DSN %s", cfg.DSN())
	}
}

func TestDSNDefaultsToRequiredTLS(t *testing.T) {
	cfg := Config{Host: "db", Port: 5432, Name: "app", Username: "app"}
	if !strings.Contains(cfg.DSN(), "sslmode=require") {
		t.Fatalf("empty SSLMode must mean require, got %s", cfg.DSN())
	}
}

func TestValidate(t *testing.T) {
	valid := Config{Host: "db", Port: 5432, Name: "app", Username: "app"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	tests := map[string]func(*Config){
		"host":     func(c *Config) { c.Host = " " },
		"port":     func(c *Config) { c.Port = 0 },
		"big port": func(c *Config) { c.Port = 70000 },
		"name":     func(c *Config) { c.Name = "" },
		"username": func(c *Config) { c.Username = "" },
		"pool":     func(c *Config) { c.MaxOpenConns = -1 },
		"backoff":  func(c *Config) { c.ConnectBackoff = -time.Second },
	}
	for name, mutate := range tests {
		cfg := valid
		mutate(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestConnectRejectsInvalidConfig(t *testing.T) {
	if _, err := Connect(context.Background(), Config{}); err == nil || !strings.Contains(err.Error(), "host is required") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestConnectGivesUpAfterAttempts(t *testing.T) {
	cfg := Config{Host: "127.0.0.1", Port: 1, Name: "x", Username: "x", SSLMode: "disable", // nothing listens on port 1
		ConnectAttempts: 2, ConnectBackoff: 10 * time.Millisecond}
	_, err := Connect(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "after 2 attempts") {
		t.Fatalf("expected retry exhaustion, got %v", err)
	}
}

func TestCloseNil(t *testing.T) {
	Close(nil)
}
