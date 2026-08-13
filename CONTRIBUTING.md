# Contributing to Port Hero

First off — thank you for considering contributing! Port Hero is a safety-critical
developer tool: it kills processes. That means **code quality and safety are
non-negotiable**.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting started

1. Fork the repository.
2. `git clone` your fork.
3. `make build` — everything should build on first try.
4. `make test` — the full suite must pass before you start.

## Development workflow

```bash
make build      # build for the current platform
make test       # run the full test suite
make vet        # static analysis
make lint       # vet + gofmt check
make release    # cross-compile all platform binaries
```

### Checklist for every change

- [ ] `gofmt -l .` produces no output
- [ ] `go vet ./...` is clean
- [ ] `go test ./... -race` passes
- [ ] New behaviour is covered by a test
- [ ] Public API has doc comments starting with the exported name
- [ ] No new runtime dependencies unless justified

## Safety-critical design rules

Port Hero's core promise is **never kill something you shouldn't**. When you touch
any of these subsystems, keep the rules in mind:

| Subsystem | Non-negotiable rule |
|---|---|
| `internal/guardrails` | Critical violations (PID 1, kernel threads, system daemons, foreign users) must **never** be bypassable, not even with `--force`. |
| `internal/killer` | Always SIGTERM first, wait the grace period, SIGKILL only survivors. Children before parents. |
| `internal/inspector` | Reading process state must be non-destructive. No signals from this package. |
| `internal/restart` | Respawned processes must be detached (`setsid` / `DETACHED_PROCESS`) so they survive the TUI. |

If a change weakens any of these rules, it will not be merged.

## Platform support

| Platform | Port→PID | Ancestry | Locks | Notes |
|---|---|---|---|---|
| Linux | pure Go `/proc` | ✅ | ✅ `/proc/locks` | zero external deps |
| macOS | `lsof` + `ps` | ✅ | ✅ per-file via `lsof` | preinstalled tools only |
| Windows | `netstat` + PowerShell | basic | ❌ | no runtime deps added |

When adding platform code, prefer build-tagged files (`_darwin.go`, `_linux.go`,
`_windows.go`) over runtime `runtime.GOOS` branches. **Always verify
cross-compilation**:

```bash
GOOS=linux GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```

## Testing

- Unit tests live next to the code (`*_test.go`).
- Tests that spawn or kill processes must clean up after themselves with
  `t.Cleanup` and must never touch real system processes (use `sleep`).
- Do not rely on the CI runner having specific ports free — bind ephemeral
  ports (`127.0.0.1:0` or high ranges).

## Commit conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(guardrails): refuse to kill processes in protected PIDs range
fix(inspector): resolve zombie detection on macOS
docs: explain the --why causality engine
test(killer): cover tree orphan prevention
```

## Pull requests

- Keep PRs focused: one logical change per PR.
- Reference the issue you're fixing (`Fixes #12`).
- Make sure the CI matrix (lint + test on Linux/macOS + cross-build) is green.

## Questions?

Open a discussion or an issue — we're friendly.

**Built with ❤ by GoLive — golive.ly**
