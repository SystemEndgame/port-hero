// Package config loads the optional user configuration file
// (~/.port-hero/config.yaml). Every setting has a safe default, so Port Hero
// works with zero configuration while remaining fully customizable.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultGracePeriod is the SIGTERM→SIGKILL grace window used when the user
// does not override it.
const DefaultGracePeriod = 1500 * time.Millisecond

// DefaultLogLevel and DefaultLogFormat are used when no config or flag is set.
const (
	DefaultLogLevel  = "info"
	DefaultLogFormat = "text"
)

// Config is the parsed user configuration.
type Config struct {
	// GracePeriod overrides the SIGTERM→SIGKILL grace window.
	GracePeriod time.Duration `yaml:"grace_period"`
	// LogLevel is one of debug|info|warn|error.
	LogLevel string `yaml:"log_level"`
	// LogFormat is one of text|json.
	LogFormat string `yaml:"log_format"`
	// Whitelist removes warning-level confirmations for specific ports and
	// processes. Critical protections can never be bypassed.
	Whitelist Whitelist `yaml:"whitelist"`
	// Protection lets users add extra protected ports and daemons.
	Protection Protection `yaml:"protection"`
}

// Whitelist lists things the Safety Shield should never warn about.
type Whitelist struct {
	Ports     []int    `yaml:"ports"`
	Processes []string `yaml:"processes"`
}

// Protection adds user-level entries to the Safety Shield tables.
type Protection struct {
	Ports   map[int]string `yaml:"extra_protected_ports"`
	Daemons []string       `yaml:"extra_protected_daemons"`
}

// Default returns a fully-populated configuration with safe defaults.
func Default() *Config {
	return &Config{
		GracePeriod: DefaultGracePeriod,
		LogLevel:    DefaultLogLevel,
		LogFormat:   DefaultLogFormat,
		Whitelist:   Whitelist{},
		Protection:  Protection{Ports: map[int]string{}},
	}
}

// Load reads the config file at the given path. A missing file is not an
// error — it yields Default(). An unreadable or malformed file is an error so
// the user notices their configuration is being ignored.
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// LoadFromHome loads the default user config file, falling back to defaults
// when it does not exist.
func LoadFromHome() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Default(), nil
	}
	return Load(filepath.Join(home, ".port-hero", "config.yaml"))
}

// applyDefaults fills zero-valued fields with safe defaults.
func (c *Config) applyDefaults() {
	if c.GracePeriod <= 0 {
		c.GracePeriod = DefaultGracePeriod
	}
	if c.LogLevel == "" {
		c.LogLevel = DefaultLogLevel
	}
	if c.LogFormat == "" {
		c.LogFormat = DefaultLogFormat
	}
	if c.Protection.Ports == nil {
		c.Protection.Ports = map[int]string{}
	}
}

// Validate checks semantic constraints and returns a descriptive error.
func (c *Config) Validate() error {
	if c.GracePeriod < 100*time.Millisecond {
		return errors.New("grace_period must be at least 100ms")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log_level %q must be one of debug|info|warn|error", c.LogLevel)
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("log_format %q must be one of text|json", c.LogFormat)
	}
	for _, p := range c.Whitelist.Ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("whitelist port %d out of range", p)
		}
	}
	for port := range c.Protection.Ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("protected port %d out of range", port)
		}
	}
	return nil
}
