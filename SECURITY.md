# Security Policy

## Reporting a vulnerability

Port Hero is a **process-management tool with the power to terminate
processes**. A vulnerability in it can damage a user's system, so we take
security extremely seriously.

**Please do not open a public issue for security problems.**

Instead, email us privately at **security@golive.ly** (or open a
[GitHub Security Advisory](https://github.com/golive-ly/port-hero/security/advisories/new)).

Please include:

- The affected version(s)
- A minimal reproduction
- Impact description
- (optional) suggested fix

We aim to respond within **48 hours** and to ship a fix for high-severity
issues within **7 days**.

## Safety model

The project's core promise is that no matter what goes wrong, Port Hero must
**never terminate a process it shouldn't**. This is enforced by the
`internal/guardrails` package:

| Protection | Level | Bypassable? |
|---|---|---|
| PID 1 (init / launchd / systemd) | CRITICAL | Never |
| Kernel threads (`[kworker]`, …) | CRITICAL | Never |
| System daemons (sshd, dockerd, launchd, …) | CRITICAL | Never |
| Self-kill | CRITICAL | Never |
| Foreign users' processes | CRITICAL | Never (unless root) |
| Well-known system ports (22, 53, 80, 443…) | WARNING | Only with `--force` |
| Low PIDs (< 10) | WARNING | Only with `--force` |

Security-sensitive changes (anything touching guardrails, killer, or signal
handling) require explicit review.

## Supported versions

| Version | Supported |
|---|---|
| latest release | ✅ |
| older releases | ❌ |

## Privacy

Port Hero performs **zero network communication**. It never telemetries,
phones home, or uploads process data. If you find any code path that makes a
network request, that is a security bug — report it.
