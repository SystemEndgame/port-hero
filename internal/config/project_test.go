package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProjectConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, projectFileName), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", projectFileName, err)
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatalf("git init: %v", err)
	}
}

func TestFindProjectConfigFromSubdir(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	writeProjectConfig(t, root, "name: golively-api\nstart: npm run dev\n")

	sub := filepath.Join(root, "apps", "dist")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg, err := FindProjectConfig(sub)
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config to be found by walking up to the repo root")
	}
	if cfg.Name != "golively-api" || cfg.Start != "npm run dev" {
		t.Errorf("cfg = %+v, want name=golively-api start=npm run dev", cfg)
	}
}

func TestFindProjectConfigStopsAtRepoRoot(t *testing.T) {
	outer := t.TempDir()
	// A config OUTSIDE the repo must not be picked up.
	writeProjectConfig(t, outer, "name: outer-project\n")

	repo := filepath.Join(outer, "repo")
	gitInit(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o700); err != nil {
		t.Fatal(err)
	}

	cfg, err := FindProjectConfig(filepath.Join(repo, "src"))
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config (repo root boundary), got %+v", cfg)
	}
}

func TestFindProjectConfigMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := FindProjectConfig(dir)
	if err != nil {
		t.Fatalf("FindProjectConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config, got %+v", cfg)
	}
}

func TestFindProjectConfigMalformed(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	writeProjectConfig(t, dir, "name: [not-a-string")

	if _, err := FindProjectConfig(dir); err == nil {
		t.Fatal("expected error for malformed yaml")
	}
}

func TestFindProjectConfigEmptyDir(t *testing.T) {
	if cfg, err := FindProjectConfig(""); err != nil || cfg != nil {
		t.Errorf("empty dir: cfg=%v err=%v, want nil,nil", cfg, err)
	}
}
