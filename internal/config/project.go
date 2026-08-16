package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/SystemEndgame/port-hero/internal/cache"
)

// projectFileName is the optional per-repository config file.
const projectFileName = ".port-hero.yaml"

// ProjectConfig is the per-repository configuration. It makes the tool
// team-aware: a cloned repo carries its display name and start command.
type ProjectConfig struct {
	// Name overrides the project name shown in the UI/JSON output.
	Name string `yaml:"name,omitempty"`
	// Start is the command used by --restart. It preserves package-manager
	// context (npm run dev, go run, docker compose) that raw argv
	// reconstruction cannot.
	Start string `yaml:"start,omitempty"`
}

// projectConfigs caches discovery results per starting directory so list
// refreshes never repeat the filesystem walk.
var projectConfigs = cache.New[*ProjectConfig](30 * time.Second)

// FindProjectConfig locates .port-hero.yaml by walking up from dir to the
// enclosing repository root, then parses it. A missing file yields (nil, nil).
// A malformed file yields (nil, error); the result is still cached so the
// hot path does not re-parse it on every call.
func FindProjectConfig(dir string) (*ProjectConfig, error) {
	if dir == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if cfg, ok := projectConfigs.Get(abs); ok {
		return cfg, nil
	}
	cfg, err := locateProjectConfig(abs)
	projectConfigs.Set(abs, cfg)
	return cfg, err
}

// locateProjectConfig walks up from dir looking for projectFileName, stopping
// at the repository root so configs from unrelated parents are never read.
func locateProjectConfig(dir string) (*ProjectConfig, error) {
	for {
		path := filepath.Join(dir, projectFileName)
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var cfg ProjectConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, err
			}
			return &cfg, nil
		}
		// The repository root is the config boundary.
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return nil, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}
