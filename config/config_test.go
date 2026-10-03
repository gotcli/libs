package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func withDefaults(t *testing.T, overrides map[string]any) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	ApplyDefaults("github.com/gotcli/test-service", nil)
	for key, value := range overrides {
		viper.Set(key, value)
	}
}

func TestValidateAcceptsDefaults(t *testing.T) {
	withDefaults(t, nil)
	if err := Validate(); err != nil {
		t.Fatalf("defaults should be valid: %v", err)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	tests := map[string]map[string]any{
		"port":       {"APP_PORT": 0},
		"timeout":    {"HTTP_READ_TIMEOUT": "not-a-duration"},
		"body limit": {"HTTP_BODY_LIMIT": -1},
	}
	for name, overrides := range tests {
		t.Run(name, func(t *testing.T) {
			withDefaults(t, overrides)
			if err := Validate(); err == nil || !strings.Contains(err.Error(), "invalid configuration") {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestLoadUsesEnvironmentAndServiceDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Chdir(t.TempDir()) // no config.yml: defaults and env only
	t.Setenv("TEST_SERVICE_APP_PORT", "4100")
	if err := Load("github.com/gotcli/test-service", map[string]any{"FEATURE_X": true, "HTTP_BODY_LIMIT": 2048}); err != nil {
		t.Fatal(err)
	}
	if viper.GetInt("APP_PORT") != 4100 || !viper.GetBool("FEATURE_X") || viper.GetInt("HTTP_BODY_LIMIT") != 2048 {
		t.Fatalf("port=%d feature=%v body=%d", viper.GetInt("APP_PORT"), viper.GetBool("FEATURE_X"), viper.GetInt("HTTP_BODY_LIMIT"))
	}
	if viper.GetString("APP_NAME") != "github.com/gotcli/test-service" {
		t.Fatalf("APP_NAME = %q", viper.GetString("APP_NAME"))
	}
}

func TestEnvPrefix(t *testing.T) {
	for module, want := range map[string]string{
		"github.com/gotcli/crm-service": "CRM_SERVICE",
		"line.gateway":                  "LINE_GATEWAY",
		"":                              "APP",
	} {
		if got := EnvPrefix(module); got != want {
			t.Errorf("EnvPrefix(%q) = %q, want %q", module, got, want)
		}
	}
}

func TestList(t *testing.T) {
	got := List(" http://localhost:4010, ,http://app.localhost:4010 ")
	if len(got) != 2 || got[0] != "http://localhost:4010" || got[1] != "http://app.localhost:4010" {
		t.Fatalf("List = %q", got)
	}
	if got := List(""); len(got) != 0 {
		t.Fatalf("List(\"\") = %q, want empty", got)
	}
}
