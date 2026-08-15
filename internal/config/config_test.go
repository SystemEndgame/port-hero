package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GracePeriod != DefaultGracePeriod {
		t.Errorf("GracePeriod = %v, want default %v", cfg.GracePeriod, DefaultGracePeriod)
	}
	if cfg.LogLevel != DefaultLogLevel || cfg.LogFormat != DefaultLogFormat {
		t.Errorf("log defaults wrong: %q %q", cfg.LogLevel, cfg.LogFormat)
	}
	if cfg.Protection.Ports == nil {
		t.Error("Protection.Ports should default to non-nil")
	}
}

func TestLoadValidConfig(t *testing.T) {
	p := writeConfig(t, `
grace_period: 2s
log_level: debug
log_format: json
whitelist:
  ports: [3000, 5173]
  processes: ["npm start"]
protection:
  extra_protected_ports:
    9000: "Admin dashboard"
  extra_protected_daemons: ["myagent"]
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GracePeriod != 2*time.Second {
		t.Errorf("GracePeriod = %v, want 2s", cfg.GracePeriod)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Errorf("log settings wrong: %q %q", cfg.LogLevel, cfg.LogFormat)
	}
	if len(cfg.Whitelist.Ports) != 2 || cfg.Whitelist.Ports[0] != 3000 {
		t.Errorf("whitelist ports wrong: %v", cfg.Whitelist.Ports)
	}
	if reason := cfg.Protection.Ports[9000]; reason != "Admin dashboard" {
		t.Errorf("protected port 9000 reason = %q", reason)
	}
	if len(cfg.Protection.Daemons) != 1 || cfg.Protection.Daemons[0] != "myagent" {
		t.Errorf("protected daemons wrong: %v", cfg.Protection.Daemons)
	}
}

func TestLoadMalformedConfigFails(t *testing.T) {
	p := writeConfig(t, "grace_period: [not-a-duration")
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestLoadInvalidValuesFails(t *testing.T) {
	cases := []string{
		"grace_period: 10ms\n", // below 100ms floor
		"log_level: verbose\n",
		"log_format: xml\n",
		"whitelist:\n  ports: [70000]\n",
	}
	for _, body := range cases {
		p := writeConfig(t, body)
		if _, err := Load(p); err == nil {
			t.Errorf("expected validation error for config: %s", body)
		}
	}
}

func TestValidateGoodConfig(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}
