// Package config loads service configuration into the global viper instance.
//
// Values come from built-in defaults, then an optional config.yml, then
// environment variables prefixed with the service name (e.g. CRM_SERVICE_DB_HOST).
// A missing config.yml is not an error because the file is git-ignored.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Defaults are the settings every service shares. Services add or override
// keys through Load's extra defaults. Connection settings such as DB_* are
// deliberately absent: each service supplies them from its own config.
var Defaults = map[string]any{
	"APP_ENV":            "development",
	"APP_PORT":           3000,
	"LOG_DIR":            "logs",
	"HTTP_READ_TIMEOUT":  "15s",
	"HTTP_WRITE_TIMEOUT": "15s",
	"HTTP_IDLE_TIMEOUT":  "60s",
	"HTTP_BODY_LIMIT":    1048576,
	"SHUTDOWN_TIMEOUT":   "10s",
	"JWT_ISSUER":         "github.com/lineoa-platform/identity-service",
	"JWT_AUDIENCE":       "github.com/lineoa-platform/identity-service",
	"JWT_ACCESS_SECRET":  "",
}

// EnvPrefix derives the environment prefix from a Go module path:
// "github.com/gotcli/crm-service" becomes "CRM_SERVICE".
func EnvPrefix(module string) string {
	parts := strings.Split(module, "/")
	prefix := strings.NewReplacer("-", "_", ".", "_").Replace(parts[len(parts)-1])
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix == "" {
		return "APP"
	}
	return prefix
}

// Load applies defaults, reads config.yml if present, binds the environment
// and validates the result.
func Load(module string, extra map[string]any) error {
	ApplyDefaults(module, extra)
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	for _, path := range []string{".", "./configs", "../", "../configs"} {
		viper.AddConfigPath(path)
	}
	viper.SetEnvPrefix(EnvPrefix(module))
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("cannot read config.yml: %w", err)
		}
	}
	return Validate()
}

// ApplyDefaults registers the shared defaults, APP_NAME and extra defaults.
func ApplyDefaults(module string, extra map[string]any) {
	for key, value := range Defaults {
		viper.SetDefault(key, value)
	}
	viper.SetDefault("APP_NAME", module)
	for key, value := range extra {
		viper.SetDefault(key, value)
	}
}

// Validate rejects configuration that would otherwise fail at request time.
func Validate() error {
	var problems []string
	if port := viper.GetInt("APP_PORT"); port <= 0 || port > 65535 {
		problems = append(problems, "APP_PORT must be between 1 and 65535")
	}
	if viper.GetInt("HTTP_BODY_LIMIT") <= 0 {
		problems = append(problems, "HTTP_BODY_LIMIT must be positive")
	}
	for _, key := range []string{"HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT"} {
		if viper.GetDuration(key) <= 0 {
			problems = append(problems, key+" must be a positive duration")
		}
	}
	if len(problems) > 0 {
		return errors.New("invalid configuration: " + strings.Join(problems, "; "))
	}
	return nil
}

// List splits a comma-separated setting such as "http://a, http://b" into its
// trimmed, non-empty items.
func List(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
