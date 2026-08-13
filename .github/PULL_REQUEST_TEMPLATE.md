## What does this PR do?

Briefly describe the change.

## Related issue
Fixes #<!-- issue number -->

## Safety impact
Does this change touch any of the safety-critical subsystems?

- [ ] `guardrails` (what can/cannot be killed)
- [ ] `killer` (signals, grace period, tree ordering)
- [ ] `inspector` (process reading)
- [ ] `restart` (detached respawn)
- [ ] No safety-critical code touched

If you checked any box, explain how the safety invariants are preserved.

## Checklist
- [ ] `gofmt -l .` is clean
- [ ] `go vet ./...` passes
- [ ] `go test ./... -race` passes
- [ ] Cross-compiles: Linux, macOS, Windows (`make release` or CI build job)
- [ ] New behaviour has tests
- [ ] Public API documented

## Screenshots / output
If user-facing, show before/after output.
