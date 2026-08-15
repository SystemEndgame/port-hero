package inspector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SystemEndgame/port-hero/internal/cache"
)

// gitInfo resolves the current branch and dirty state for a working
// directory. It is fully dependency-free for branch detection (reads
// .git/HEAD directly, including worktrees) and uses `git status` only
// for the dirty check, with a short timeout and per-repo caching.
func gitInfo(cwd string) (branch string, dirty bool) {
	if cwd == "" {
		return "", false
	}
	gitDir := resolveGitDir(cwd)
	if gitDir == "" {
		return "", false
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false
	}
	ref := strings.TrimSpace(string(head))
	switch {
	case strings.HasPrefix(ref, "ref: refs/heads/"):
		branch = strings.TrimPrefix(ref, "ref: refs/heads/")
	case strings.HasPrefix(ref, "ref: "):
		// Detached symbolic ref pointing to a tag or similar.
		branch = "detached@" + strings.TrimPrefix(ref, "ref: ")
	default:
		branch = "detached@" + shortHash(ref)
	}
	dirty = gitDirtyCached(cwd, gitDir)
	return branch, dirty
}

// resolveGitDir locates the effective .git directory, following git
// worktree "gitdir:" pointer files.
func resolveGitDir(cwd string) string {
	dir := filepath.Join(cwd, ".git")
	fi, err := os.Stat(dir)
	if err != nil {
		return ""
	}
	if fi.IsDir() {
		return dir
	}
	// .git is a file containing "gitdir: /path/to/gitdir" (linked worktree).
	data, err := os.ReadFile(dir)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}
	gd := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(cwd, gd)
	}
	return gd
}

func shortHash(h string) string {
	h = strings.TrimSpace(h)
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// ProjectName resolves the human-readable project name for a working
// directory: the git repository root when inside a repo (walking up), or
// the base name of the directory otherwise.
func ProjectName(cwd string) string {
	if cwd == "" {
		return ""
	}
	// Walk up looking for a git root.
	for dir := cwd; ; {
		root := resolveGitDir(dir)
		if root != "" {
			if base := filepath.Base(dir); base != "" && base != "." && base != string(filepath.Separator) {
				return base
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if base := filepath.Base(cwd); base != "." && base != string(filepath.Separator) {
		return base
	}
	return cwd
}

// dirtyCache caches `git status --porcelain` results per repository so list
// views stay fast. Entries expire after 30 s so freshly-edited working trees
// are picked up on the next refresh.
var dirtyCache = cache.New[bool](30 * time.Second)

// gitDirtyCached runs `git status --porcelain` with a timeout and caches
// the result per repository so list views stay fast.
func gitDirtyCached(cwd, gitDir string) bool {
	if v, ok := dirtyCache.Get(gitDir); ok {
		return v
	}

	dirty := false
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	cmd := execGit(ctx, cwd)
	out, err := cmd.Output()
	if err == nil {
		dirty = len(out) > 0
	}

	dirtyCache.Set(gitDir, dirty)
	return dirty
}
