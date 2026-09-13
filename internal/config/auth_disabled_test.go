package config

import (
	"strings"
	"testing"
)

func TestExplicitAuthDisabled(t *testing.T) {
	const text = "listen = \"127.0.0.1:3000\"\n[telemt]\nurl = \"http://127.0.0.1:9091\"\n[auth]\ndisabled = true\n"
	s, err := DecodeSource([]byte(text), "panel.toml", "current")
	if err != nil || !s.Config.Auth.Disabled {
		t.Fatal("explicit opt-out rejected", err)
	}
	for _, value := range []string{"false", `"true"`} {
		if _, err := DecodeSource([]byte(strings.Replace(text, "disabled = true", "disabled = "+value, 1)), "panel.toml", "current"); err == nil {
			t.Fatal("accepted missing credentials or invalid boolean", value)
		}
	}
	cfg, err := load(t, strings.Replace(minimal, "[auth]", "[auth]\ndisabled = true", 1))
	if err != nil || cfg.Auth.PasswordHash != "$2a$10$x" || cfg.Auth.Username != "admin" {
		t.Fatal("stored configuration credentials changed", err)
	}
}
