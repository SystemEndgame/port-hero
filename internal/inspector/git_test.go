package inspector

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitInfoBranch(t *testing.T) {
	dir := t.TempDir()
	must(t, exec.Command("git", "init", "-q", dir).Run())
	must(t, exec.Command("git", "-C", dir, "checkout", "-q", "-b", "feature/payment-fix").Run())
	must(t, exec.Command("git", "-C", dir, "-c", "user.email=t@t", "-c", "user.name=t",
		"commit", "-q", "--allow-empty", "-m", "init").Run())

	branch, dirty := gitInfo(dir)
	if branch != "feature/payment-fix" {
		t.Errorf("branch = %q, want feature/payment-fix", branch)
	}
	if dirty {
		t.Error("expected clean tree")
	}
}

func TestGitInfoDirty(t *testing.T) {
	dir := t.TempDir()
	must(t, exec.Command("git", "init", "-q", dir).Run())
	must(t, exec.Command("git", "-C", dir, "checkout", "-q", "-b", "main").Run())
	must(t, exec.Command("git", "-C", dir, "-c", "user.email=t@t", "-c", "user.name=t",
		"commit", "-q", "--allow-empty", "-m", "init").Run())
	must(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("x"), 0o600))

	_, dirty := gitInfo(dir)
	if !dirty {
		t.Error("expected dirty tree (untracked file)")
	}
}

func TestGitInfoNoRepo(t *testing.T) {
	dir := t.TempDir()
	branch, dirty := gitInfo(dir)
	if branch != "" {
		t.Errorf("branch = %q, want empty", branch)
	}
	if dirty {
		t.Error("dirty on non-repo")
	}
}

func TestProjectNameRepo(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "apps", "golively-app")
	must(t, os.MkdirAll(sub, 0o700))
	must(t, exec.Command("git", "init", "-q", root).Run())

	// Repo root is "root" (temp dir name), subdir inherits it via walk-up.
	if got := ProjectName(sub); got != filepath.Base(root) {
		t.Errorf("ProjectName(subdir) = %q, want %q (repo root)", got, filepath.Base(root))
	}
}

func TestProjectNameNoRepo(t *testing.T) {
	dir := t.TempDir()
	if got := ProjectName(dir); got != filepath.Base(dir) {
		t.Errorf("ProjectName(no repo) = %q, want %q", got, filepath.Base(dir))
	}
}

func TestGitInfoDetached(t *testing.T) {
	dir := t.TempDir()
	must(t, exec.Command("git", "init", "-q", dir).Run())
	must(t, exec.Command("git", "-C", dir, "-c", "user.email=t@t", "-c", "user.name=t",
		"commit", "-q", "--allow-empty", "-m", "init").Run())
	must(t, exec.Command("git", "-C", dir, "checkout", "-q", "--detach", "HEAD").Run())

	branch, _ := gitInfo(dir)
	if len(branch) < len("detached@") {
		t.Errorf("expected detached branch label, got %q", branch)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
}
