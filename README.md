<div align="center">

# ⚓ PORT HERO

**The port manager that knows your projects.** Stop killing your databases by mistake.

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/github/license/SystemEndgame/port-hero)
![Release](https://img.shields.io/github/v/release/SystemEndgame/port-hero)
![CI](https://img.shields.io/github/actions/workflow/status/SystemEndgame/port-hero/ci.yml?branch=main&label=CI)
![Stars](https://img.shields.io/github/stars/SystemEndgame/port-hero?style=social)
![Platforms](https://img.shields.io/badge/Platforms-macOS--Linux--Windows-important)
[![Built by GoLive](https://img.shields.io/badge/built%20by-GoLive-6f42c1)](https://golive.ly)

> **What's running on port 3000?** Port Hero answers in one command — the process, its **project name**, its **git branch**, its **container**, its **causality chain** ("why is it running?") — then lets you **kill it gracefully** or **kill & restart it** from a beautiful terminal UI.

`lsof -i :3000` → `kill -9` → *"did I just kill my database?"* — never again.

</div>

### Quick start

```bash
port 3000                  # what is this, and whose project is it?
port 3000 --why            # why is it running? (launchd → pm2 → node)
port 3000 --kill           # graceful SIGTERM of the whole process tree
port 3000 --kill --dry-run # preview without sending a signal
port 3000 --restart        # kill and respawn it, detached
port 53 --udp              # UDP listeners too (DNS, NTP, mDNS…)
port --check 3000          # exit 0 if busy, 2 if free (CI scripting)
```

<!--
SEO keywords: port manager, kill process on port, what is running on port, find process using port,
lsof alternative, lsof port 3000, free port, kill port process, mac port manager, linux port manager,
git branch process, restart dev server, graceful kill SIGTERM, TUI cli tool
-->

<div align="center">

![Port Hero demo — project-aware port manager with causality tracing, graceful kill & restart](demo/port-hero-demo.gif)

</div>

------

## ✨ Features

| | |
|---|---|
| 🏷️ **Project-aware context** | The feature nobody else has: Port Hero knows **which project** owns a port — `golively-app · ⎇ feature/auth-flow [CLEAN]` — by walking up to the git repo root. It's a *developer workflow tool*, not a sysadmin utility. |
| 🧬 **Causality engine — "why is this running?"** | Traces the full ancestry chain (`launchd → pm2 → node`) and identifies the supervisor, systemd unit / launchd label, container, and session. The witr-class feature, built in. |
| 🔍 **Instant port → process resolution** | **TCP and UDP** (`--udp`). Pure-Go on Linux (`/proc/net/{tcp,udp}` + inode→PID scan, zero dependencies), `lsof` on macOS, `netstat` on Windows. |
| 🔎 **Process tracing by name / PID** | `port node`, `port --pid 4821` — find every process by name or exact PID. |
| 🔒 **File lock detection** | `port --file /path` reveals which process holds a lock (Linux `/proc/locks` pure-Go, macOS via `lsof`). |
| 🌿 **Git branch detection** | Reads `.git/HEAD` directly (worktree-aware) — shows `⎇ feature/payment-fix [CLEAN]` in the UI. No `git` call needed for branches. |
| 🐳 **Container detection** | cgroup parsing on Linux resolves the Docker container and its short name. |
| 🌳 **Process tree & orphan prevention** | Builds the full parent/child tree and terminates children **first**, so no orphaned workers or DB pools are left behind. |
| 🛡️ **Graceful termination** | `SIGTERM` to the whole tree → **1.5 s grace** → `SIGKILL` only if needed. Clean connection shutdown, no data corruption. |
| 🚧 **The Safety Shield** | Refuses to touch **PID 1**, kernel threads, `launchd`/`systemd`/`sshd`/`dockerd` & 60+ system daemons, foreign users' processes, and well-known system ports (22 SSH, 53 DNS, 80, 443…). `--force` bypasses *warnings only* — critical protections can never be bypassed. |
| 🖥️ **Terminal UI** | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) — interactive list, rich detail view with causality chain, safety confirmations, result feedback. |
| 🔁 **Kill & Restart** | Respawns the exact command **detached** from its original working directory — **exact argv** everywhere (Linux `/proc/pid/cmdline`, macOS `KERN_PROCARGS2`), or the repo's `start` command from `.port-hero.yaml`. Output is logged to `~/.port-hero/restarts/`. |
| 🚀 **Cross-platform single binary** | macOS (Intel/Apple Silicon), Linux, Windows. ~3.5 MB, starts in milliseconds, zero runtime dependencies. |
| 🧪 **Scriptable** | `--json` output (including `--kill --json` / `--restart --json` for CI), meaningful exit codes (0–5), non-interactive `--kill` / `--force` / `--restart` for CI and shell pipelines. |
| 🐚 **Shell completions** | `port --completion bash|zsh|fish` — tab completion for ports, flags and PIDs. |
| ⚙️ **Config files** | Global `~/.port-hero/config.yaml` (grace period, whitelist, logging) **and** per-repo `.port-hero.yaml` (display name, start command) — discovered by walking up to the repository root. |
| 🛡️ **PID-reuse protection** | Every signal is safe against PID recycling: **pidfd** on Linux (atomic), **start-time verification** on macOS (`PROC_PIDTBSDINFO`) and Linux, plus owner re-verification before signalling. |
| 👁️ **Dry-run preview** | `port 3000 --kill --dry-run` shows exactly what would be terminated without sending a single signal. |
| 📝 **Structured logging** | `--log-level debug|info|warn|error` and `--log-format text|json` (stdlib `slog`) for CI audit trails and log aggregation. |

---

## 🚀 Install

### Homebrew

```bash
brew install SystemEndgame/tap/port-hero
```

### Curl (macOS & Linux)

```bash
curl -sL https://raw.githubusercontent.com/SystemEndgame/port-hero/main/install.sh | sh
```

### Go

```bash
go install github.com/SystemEndgame/port-hero@latest
```

### Build from source

```bash
git clone https://github.com/SystemEndgame/port-hero.git
cd port-hero
make build          # binaries/port-hero
make install        # installs to ~/.local/bin (add it to PATH if needed)
```

---

## 📖 Usage

```bash
port                        # interactive list of every listening port
port 3000                   # interactive detail view for port 3000
port node                   # interactive list filtered by process name
port 3000 --why             # causality chain — why is this running?
port node --why             # causality for every matching process
port --pid 4821 --why       # causality for a specific PID
port 3000 --kill            # graceful kill (SIGTERM, whole process tree)
port 3000 --force           # force kill (SIGKILL after grace period)
port 3000 --restart         # kill & restart the command, detached
port --pid 4821 --kill      # kill a specific PID by number
port 3000 --kill --all      # kill every process listening on the port
port 3000 --kill --dry-run  # preview what would be killed, send nothing
port --file /var/lib/dpkg/lock   # who holds this file lock?
port --json                 # machine-readable dump of all listeners
port 3000 --json            # machine-readable dump of one port
port 3000 --kill --json     # machine-readable kill result (CI)
port 3000 --restart --json  # machine-readable restart result (CI)
port --completion bash      # shell completion (bash|zsh|fish)
port --log-level debug      # structured log level (debug|info|warn|error)
port --log-format json      # structured log format (text|json)
port --check 3000           # exit 0 if busy, 2 if free (CI scripts)
port --wait 3000            # wait until the port is free (default 30s)
port --wait 3000 --timeout 90s   # wait with a custom timeout
port --next 3000            # print the first free port at or above 3000
port 53 --udp               # query UDP instead of TCP (DNS, NTP, mDNS…)
port --protocol udp 3000    # same as --udp (tcp|udp)
port --json --jq '.[].name' # filter any --json output with a jq expression
port --version
```

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, no warnings |
| `1` | Success with warnings (protected port, force-kill needed) |
| `2` | Not found (port free / no matching process) |
| `3` | Blocked by the Safety Shield / permission denied |
| `4` | Invalid input or ambiguous match |
| `5` | Internal error |

### Filtering JSON output with jq

`--jq` applies a full [jq](https://jqlang.github.io/jq/) expression to any
`--json` output — listings, kill results and restart results. It runs on the
pure-Go [`gojq`](https://github.com/itchyny/gojq) engine built into the
binary, so **no external `jq` is required on any platform**.

```bash
port --json --jq '.[].name'                           # every process name
port --json --jq '[.[] | select(.port > 8000) | {name, port}]'   # heavy listeners
port --json --jq '[.[] | select(.project == "golively-app")]'    # one project
port 3000 --kill --json --jq '.graceful'              # one field from a kill result
```

A single result is printed as-is; multiple results are wrapped in a JSON array.

### Causality example

```
$ port 3000 --why

Target    : node (pid 14233)
User      : deploy
Command   : node dist/server.js
Working Dir: /home/deploy/myapp
Git Branch: main [CLEAN]
Started By: pm2 (pid 5034)

Why It Runs:
launchd (pid 1)
└─ pm2 (pid 5034)  [pm2]
   └─ node (pid 14233)
```

### Interactive keys

| View | Keys |
|---|---|
| List | `↑/↓` or `j/k` navigate · `enter` inspect · `r` refresh · `q` quit |
| Detail | `space`/`K` graceful kill · `F` force kill · `R` kill & restart · `b` back · `q` quit |
| Confirm | `y`/`enter` confirm · `f` toggle force · `esc` cancel |

### Example

```
⚓ PORT HERO v0.1.0 — Local Port Manager

  🔍 Port 3000 is occupied by:

    Process:    node (PID 48210)
    User:       alex
    Project:    golively-app
    Memory:     142.5 MB | CPU: 1.2%
    Directory:  ~/projects/golively-app
    Git Branch: ⎇ feature/auth-flow  [CLEAN]
    Command:    node --inspect server.js

  ─────────────── ACTIONS ───────────────
   [Space/K]  Graceful Kill (SIGTERM)
   [F]        Force Kill (SIGKILL)
   [R]        Kill & Restart Command
   [Esc/Q]    Cancel

  [⚡ GoLive Utilities — golive.ly]
```

---

## 🧠 How it works

```
┌────────────────────────────────────────────────────────────────────┐
│                              port-hero                             │
├────────────────────────────────────────────────────────────────────┤
│  inspector        ports → PIDs → cwd → git branch → container      │
│    · linux.go     pure-Go /proc/net/tcp + inode→PID (no deps)      │
│    · darwin.go    lsof -iTCP -sTCP:LISTEN + ps (batched)           │
│    · windows.go   netstat -ano + PowerShell                        │
│  tree.go          parent/child snapshot → post-order kill order    │
├────────────────────────────────────────────────────────────────────┤
│  ancestry         causality engine ("why is this running?")        │
│    · parent-chain walk to PID 1                                    │
│    · supervisor detection (systemd, launchd, pm2, tmux, ssh…)      │
│    · service detection (systemd unit / launchd label)              │
│    · session detection (SSH, tmux, interactive shell)              │
├────────────────────────────────────────────────────────────────────┤
│  locks            file-lock holders (Linux /proc/locks, lsof)      │
├────────────────────────────────────────────────────────────────────┤
│  guardrails       THE SAFETY SHIELD (checked before ANY signal)    │
│    · critical: PID 1, kernel threads, system daemons, self,        │
│      foreign users → ALWAYS blocked                                │
│    · warning: protected ports (22/53/80/443…), low PIDs →          │
│      require confirmation; --force may bypass                      │
├────────────────────────────────────────────────────────────────────┤
│  killer           SIGTERM (tree, children first)                   │
│                   → 1.5 s grace (50 ms polls)                      │
│                   → SIGKILL survivors only                         │
├────────────────────────────────────────────────────────────────────┤
│  restart          detached respawn (setsid / DETACHED_PROCESS)     │
│                   from original cwd, output → ~/.port-hero/logs    │
├────────────────────────────────────────────────────────────────────┤
│  tui              bubbletea + lipgloss                             │
│  main             CLI parsing + --why/--json/exit codes/           │
│                   non-interactive actions + completions            │
└────────────────────────────────────────────────────────────────────┘
```

### Safety guarantees

- Every kill action passes through `guardrails.Check` before a single signal is sent.
- **Critical violations** (PID 1, kernel threads, `launchd`, `systemd`, `sshd`, `dockerd`, self-kill, other users' processes) cannot be bypassed — even with `--force`.
- Protected system ports (22, 53, 80, 443, 3306, 5432…) raise an explicit confirmation dialog.
- Graceful `SIGTERM` first, with a configurable grace period, so databases and servers close connections cleanly.
- Process-tree termination is **child-first**, preventing orphaned workers.
- Before any signal, the target's **identity is re-verified** so a recycled PID can never be killed by mistake.

### Security model

Port Hero's safety is **defence in depth**:

1. **Guardrails (Safety Shield)** — every kill passes through [`guardrails.Check`](internal/guardrails/guardrails.go) before a single signal is sent. Critical protections (PID 1, kernel threads, system daemons, foreign processes, self-kill) can never be bypassed.
2. **Identity re-verification** — immediately before signalling, the target is re-checked against the inspection snapshot: alive, same owner, and — where the platform exposes it — the same start time.
   - **Linux** — start time from `/proc/<pid>/stat` (field 22).
   - **macOS** — start time from `PROC_PIDTBSDINFO` (libproc). The best defence macOS offers; not atomic like pidfd.
   - **Windows** — fresh `tasklist` re-query + owner check (start time is not exposed via tasklist).
3. **Atomic signalling (Linux only)** — every signal goes through a **pidfd** (`pidfd_open` + `pidfd_send_signal`), so a recycled PID returns `ESRCH` instead of being hit. Strictly stronger than re-verification; macOS and Windows rely on layer 2.
4. **Respawn awareness** — if the target is managed by a supervisor (launchd, systemd, npm, yarn, pm2…) that will restart it, Port Hero warns you before killing.
5. **Zero escalation** — Port Hero never escalates privileges and never modifies system state outside the target process.

---

## ⚙️ Configuration

Port Hero works with zero configuration. To customise it, create
`~/.port-hero/config.yaml`:

```yaml
# Grace period between SIGTERM and SIGKILL (minimum 100ms).
grace_period: 2s

# Structured logging defaults (overridable via --log-level / --log-format).
log_level: info        # debug | info | warn | error
log_format: text       # text | json

# "Never ask me about these" — suppresses warning-level confirmations only.
# Critical protections (PID 1, kernel threads, system daemons, foreign
# processes, self-kill) can NEVER be bypassed by the whitelist.
whitelist:
  ports: [3000, 5173]
  processes: ["npm start", "go run"]

# Extra Safety Shield entries — treated as CRITICAL, never bypassable.
protection:
  extra_protected_ports:
    9000: "Admin dashboard"
  extra_protected_daemons: ["mycriticald"]
```

### Project configuration — `.port-hero.yaml`

Drop a file at your repository root to make Port Hero **team-aware**. It is
discovered by walking up from the process working directory to the enclosing
git root, so it works even when a process `chdir`s into a subdirectory.

```yaml
# .port-hero.yaml
name: "golively-api"     # shown instead of the repo directory name
start: "npm run dev"     # used by --restart (preserves npm/go/docker context)
```

- `name` overrides the project name shown in the TUI and `--json`.
- `start` is what `--restart` runs. Preferring it over raw argv reconstruction
  keeps the package-manager context (`npm run dev`, `go run`, `docker compose
  up`) and round-trips quotes on every platform — the one case raw argv cannot.

### Whitelist — What It Does & Doesn't Do

✅ **Does:** suppress warning-level violations for matching ports/processes —
the "⚠ protected port / low PID" confirmation is skipped, so your dev port
(3000, 5173…) no longer asks for confirmation on every kill.

❌ **Does NOT:** auto-kill or weaken critical protections. PID 1, kernel
threads, system daemons, foreign processes and self-kill are **always**
blocked — whitelist or not, `--force` or not.

❌ **Does NOT:** hide anything from the audit trail. Whitelisted kills are
still logged like every other kill (`--log-format json` includes them), and
the JSON `warnings` field still reports any remaining observations.

---

## 🛠️ Development

```bash
make build      # build for the current platform
make test       # run the full test suite
make vet        # static analysis
make release    # cross-compile binaries/ for all platforms
```

### Project layout

```
port-hero/
├── main.go                      # CLI entry point (flags, config, logging)
├── internal/
│   ├── inspector/               # port→process resolution + enrichment
│   │   ├── api.go               # public API (FindByPort, FindAll…)
│   │   ├── darwin.go            # macOS (lsof + ps)
│   │   ├── linux.go             # Linux (pure Go /proc + batched CPU)
│   │   ├── windows.go           # Windows (netstat + PowerShell)
│   │   ├── git.go               # branch/dirty detection (worktree-aware)
│   │   ├── container_linux.go   # Docker cgroup detection
│   │   └── tree.go              # process tree / kill ordering
│   ├── ancestry/                # causality engine + supervisor auto-restart
│   ├── cache/                   # generic TTL cache (bounded memory)
│   ├── config/                  # ~/.port-hero/config.yaml loader
│   ├── guardrails/              # the Safety Shield (+ user config/whitelist)
│   ├── killer/                  # SIGTERM → grace → SIGKILL (identity-safe)
│   ├── locks/                   # file-lock holders
│   ├── restart/                 # detached kill & restart
│   └── tui/                     # bubbletea + lipgloss UI
├── Makefile
└── install.sh
```

---

## 📦 Releases

Every release publishes prebuilt archives and a `checksums.txt` for SHA256
verification. The [`install.sh`](install.sh) installer downloads the archive
for your platform and verifies its checksum before installing.

| Platform | Arch | Asset |
|---|---|---|
| macOS | arm64 / amd64 | `port-hero-darwin-{arm64,amd64}.tar.gz` |
| Linux | amd64 / arm64 | `port-hero-linux-{amd64,arm64}.tar.gz` |
| Windows | amd64 | `port-hero-windows-amd64.zip` |

> Built from a git tag by [GoReleaser](.goreleaser.yaml); releases start as
> drafts and are published manually after review.

---

## 🛠️ Troubleshooting

| Symptom | Cause & fix |
|---|---|
| `Nothing listening on port N` (exit 2) | The port is free — check `netstat`/`lsof` for UDP listeners; Port Hero lists TCP listeners only. |
| `process N not found` when using `--pid` | The process exited, or you lack permission to read `/proc`/`ps` for it. |
| Kill succeeds but the process comes back | The process is managed by **launchd/systemd** — Port Hero warns about this before killing. Use `launchctl unload` / `systemctl stop` instead. |
| `lsof failed` / `ps failed` on macOS | `lsof` and `ps` are preinstalled; if they were removed, reinstall Command Line Tools (`xcode-select --install`). |
| PowerShell is slow on Windows | First run initialises .NET; subsequent runs are faster. `--json` output is unaffected. |
| Docker containers not labelled on macOS | Docker Desktop runs in a VM — container detection is Linux-only. See Known Limitations. |
| `port --file` lists nothing on macOS | macOS has no central lock table; `--file` uses `lsof` per path and reports open handles. System-wide listing is unsupported. |

## 🔑 Required permissions (per platform)

| Platform | Requirement |
|---|---|
| **Linux** | Read access to `/proc/net/tcp{,6}` and to the target's `/proc/<pid>/*` (usually satisfied for your own processes). Killing **another user's** process needs `sudo` — and is blocked by the Safety Shield unless run as root. |
| **macOS** | `lsof` and `ps` (preinstalled). Killing other users' processes needs `sudo` and is blocked by the Safety Shield for non-root users. |
| **Windows** | `netstat`, `tasklist` and PowerShell (preinstalled). Admin rights are only needed to kill elevated/other-user processes. |

Port Hero never escalates privileges and only inspects what the invoking user can see.

## ⚠️ Known limitations

- **TCP by default** — UDP is available via `--udp` / `--protocol udp`. UDP has no "listening" state, so bound client sockets appear too.
- **Container detection is Linux-only** (cgroup parsing). macOS/Windows containers are not labelled.
- **macOS lock listing** is per-file via `lsof`; a system-wide `--file` sweep is not supported (`port --file <path>` works).
- **macOS restart command reconstruction** splits the command line naively; arguments containing spaces/quotes may not round-trip perfectly.
- **Windows CPU%** is not reported (tasklist exposes memory only).
- **CPU sampling** adds a fixed ~250 ms to list refreshes on Linux (batched across all processes — one window for the whole list).
- **Non-interactive `--kill`** acts on the first process for a port unless `--all` is passed; use `--pid` for a specific one.

---

## 🔒 Security & Privacy

- **Zero telemetry.** No network calls at any point (the only subprocesses are `lsof`/`ps` on macOS, `netstat`/PowerShell on Windows, and `git status` for the dirty check — all local).
- Runs entirely on your machine; your processes and paths never leave it.
- The installer script is plain-text and auditable.

---

## 🗺️ Roadmap

| Area | What & why |
|---|---|
| **Windows native sockets** | Replace `netstat`/PowerShell with `GetExtendedTcpTable` / `GetExtendedUdpTable` (ntdll) — removes the .NET cold-start and the subprocess dependency entirely. |
| **macOS native socket enumeration** | Full `PROC_PIDLISTFDS` + `PROC_PIDFDSOCKETINFO` walk to drop the `lsof` dependency (the `socket_fdinfo` layout is version-fragile; currently gated behind a tolerant size check). |
| **Unix domain sockets** | `--unix /path/to.sock` — who is connected to this socket? Useful for Postgres, Redis and Docker control sockets. |
| **`--jq` filtering** | `port --json --jq '.processes[] \| select(.project == "golively-app")'` for powerful scripting. |
| **Registry / history** | Persistent history of kills & restarts (`~/.port-hero/history`) with undo-style metadata. |
| **Network namespace awareness** | Label processes by container network namespace on Linux, so the same port in different namespaces doesn't look ambiguous. |
| **VS Code extension** | Panel that surfaces `port-hero` results inline in the editor. |

## 📜 Changelog

### v0.3.1 (unreleased)
- **`--jq` filtering** — apply a jq expression to any `--json` output (listings, kill and restart results) using the pure-Go `gojq` engine built into the binary. No external `jq` required, works on every platform.

### v0.3.0
- **UDP support** — `--udp` / `--protocol tcp|udp` across macOS, Linux and Windows (pure-Go `/proc/net/udp{,6}` on Linux).
- **Exact command-line everywhere** — macOS now reads the true argv via `KERN_PROCARGS2` (env excluded); Linux via `/proc/<pid>/cmdline`.
- **PID-reuse protection on macOS** — start-time verification via `PROC_PIDTBSDINFO` (libproc), matching Linux pidfd.
- **Per-repo configuration** — `.port-hero.yaml` (display name + `start` command) discovered by walking up to the git root; `--restart` prefers it.
- **Smart restart** — respawns via shell so `npm run dev` / `go run` / `docker compose up` round-trip quotes correctly.
- **CI helpers** — `port --check`, `--wait`, `--next` for shell pipelines; Windows and macOS added to CI (now green on all three OSes).

### v0.2.0
- **The Safety Shield** — critical protections (PID 1, kernel threads, system daemons, foreign users, self-kill) that `--force` cannot bypass.
- **pidfd atomic signalling on Linux** (`pidfd_open` + `pidfd_send_signal`) — closes the PID-reuse race.
- **Respawn awareness** — warns when a supervisor (launchd, systemd, npm, yarn, pnpm, pm2, nodemon) will restart your target.
- **Config files** — global `~/.port-hero/config.yaml` with grace period, whitelist and logging.
- **`--all`, `--dry-run`, `--json` for kill/restart**, TTL caches, `slog` structured logging, fuzz tests, IPv6 decoding fix, SHA256-verified installer.

### v0.1.0
- Initial release — interactive TUI, port→process resolution, causality tracing, git branch detection, graceful tree kill.

---

## 📄 License

[MIT](LICENSE)

---

<div align="center">

**Built with ❤️ by [GoLive](https://golive.ly)** — free, zero-knowledge dev tools.

Check out our other tools at [golive.ly](https://golive.ly) · [fwd.gr](https://fwd.gr)

⭐ Star the repo if Port Hero saved you from killing a database today!

</div>
