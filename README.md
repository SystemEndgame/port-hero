<div align="center">

# ⚓ PORT HERO

**The port manager that knows your projects.** Stop killing your databases by mistake.

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/github/license/SystemEndgame/port-hero)
![Release](https://img.shields.io/github/v/release/SystemEndgame/port-hero)
![Stars](https://img.shields.io/github/stars/SystemEndgame/port-hero?style=social)
![Platforms](https://img.shields.io/badge/macOS-Linux-Windows-important)

> **What's running on port 3000?** Port Hero answers in one command — the process, its **project name**, its **git branch**, its **container**, its **causality chain** ("why is it running?") — then lets you **kill it gracefully** or **kill & restart it** from a beautiful terminal UI.

`lsof -i :3000` → `kill -9` → *"did I just kill my database?"* — never again.

</div>

<!--
SEO keywords: port manager, kill process on port, what is running on port, find process using port,
lsof alternative, lsof port 3000, free port, kill port process, mac port manager, linux port manager,
git branch process, restart dev server, graceful kill SIGTERM, TUI cli tool
-->

---

## ✨ Features

| | |
|---|---|
| 🏷️ **Project-aware context** | The feature nobody else has: Port Hero knows **which project** owns a port — `golively-app · ⎇ feature/auth-flow [CLEAN]` — by walking up to the git repo root. It's a *developer workflow tool*, not a sysadmin utility. |
| 🧬 **Causality engine — "why is this running?"** | Traces the full ancestry chain (`launchd → pm2 → node`) and identifies the supervisor, systemd unit / launchd label, container, and session. The witr-class feature, built in. |
| 🔍 **Instant port → process resolution** | Pure-Go on Linux (`/proc/net/tcp` + inode→PID scan, zero dependencies). `lsof`+`ps` on macOS, `netstat`+PowerShell on Windows. |
| 🔎 **Process tracing by name / PID** | `port node`, `port --pid 4821` — find every process by name or exact PID. |
| 🔒 **File lock detection** | `port --file /path` reveals which process holds a lock (Linux `/proc/locks` pure-Go, macOS via `lsof`). |
| 🌿 **Git branch detection** | Reads `.git/HEAD` directly (worktree-aware) — shows `⎇ feature/payment-fix [CLEAN]` in the UI. No `git` call needed for branches. |
| 🐳 **Container detection** | cgroup parsing on Linux resolves the Docker container and its short name. |
| 🌳 **Process tree & orphan prevention** | Builds the full parent/child tree and terminates children **first**, so no orphaned workers or DB pools are left behind. |
| 🛡️ **Graceful termination** | `SIGTERM` to the whole tree → **1.5 s grace** → `SIGKILL` only if needed. Clean connection shutdown, no data corruption. |
| 🚧 **The Safety Shield** | Refuses to touch **PID 1**, kernel threads, `launchd`/`systemd`/`sshd`/`dockerd` & 60+ system daemons, foreign users' processes, and well-known system ports (22 SSH, 53 DNS, 80, 443…). `--force` bypasses *warnings only* — critical protections can never be bypassed. |
| 🖥️ **Terminal UI** | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) — interactive list, rich detail view with causality chain, safety confirmations, result feedback. |
| 🔁 **Kill & Restart** | Respawns the exact command **detached**, from its original working directory, with output logged to `~/.port-hero/restarts/`. |
| 🚀 **Cross-platform single binary** | macOS (Intel/Apple Silicon), Linux, Windows. ~3.5 MB, starts in milliseconds, zero runtime dependencies. |
| 🧪 **Scriptable** | `--json` output, meaningful exit codes (0–5), non-interactive `--kill` / `--force` / `--restart` for CI and shell pipelines. |
| 🐚 **Shell completions** | `port --completion bash|zsh|fish` — tab completion for ports, flags and PIDs. |

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
make install        # installs to ~/bin or /usr/local/bin
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
port 3000 --force           # force kill (SIGKILL after 1.5 s grace)
port 3000 --restart         # kill & restart the command, detached
port --file /var/lib/dpkg/lock   # who holds this file lock?
port --json                 # machine-readable dump of all listeners
port 3000 --json            # machine-readable dump of one port
port --completion bash      # shell completion (bash|zsh|fish)
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
├── main.go                      # CLI entry point
├── internal/
│   ├── inspector/               # port→process resolution + enrichment
│   │   ├── api.go               # public API (FindByPort, FindAll…)
│   │   ├── darwin.go            # macOS (lsof + ps)
│   │   ├── linux.go             # Linux (pure Go /proc)
│   │   ├── windows.go           # Windows (netstat + PowerShell)
│   │   ├── git.go               # branch/dirty detection (worktree-aware)
│   │   ├── container_linux.go   # Docker cgroup detection
│   │   └── tree.go              # process tree / kill ordering
│   ├── guardrails/              # the Safety Shield
│   ├── killer/                  # SIGTERM → grace → SIGKILL
│   ├── restart/                 # detached kill & restart
│   └── tui/                     # bubbletea + lipgloss UI
├── Makefile
└── install.sh
```

---

## 📦 Releases

| Platform | Arch | Binary |
|---|---|---|
| macOS | arm64 / amd64 | `port-hero-darwin-{arm64,amd64}` |
| Linux | amd64 / arm64 | `port-hero-linux-{amd64,arm64}` |
| Windows | amd64 | `port-hero-windows-amd64.exe` |

---

## 🔒 Security & Privacy

- **Zero telemetry.** No network calls at any point (the only subprocesses are `lsof`/`ps` on macOS, `netstat`/PowerShell on Windows, and `git status` for the dirty check — all local).
- Runs entirely on your machine; your processes and paths never leave it.
- The installer script is plain-text and auditable.

---

## 📄 License

[MIT](LICENSE)

---

<div align="center">

**Built with ❤️ by [GoLive](https://golive.ly)** — free, zero-knowledge dev tools.

Check out our other tools at [golive.ly](https://golive.ly) · [fwd.gr](https://fwd.gr)

⭐ Star the repo if Port Hero saved you from killing a database today!

</div>
