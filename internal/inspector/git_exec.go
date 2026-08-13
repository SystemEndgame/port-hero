//go:build !windows

package inspector

import (
	"context"
	"os/exec"
)

// execGit builds the git invocation for the dirty check.
func execGit(ctx context.Context, cwd string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", "-C", cwd, "status", "--porcelain")
	cmd.Env = append([]string{"GIT_OPTIONAL_LOCKS=0"}, osEnviron()...)
	return cmd
}
