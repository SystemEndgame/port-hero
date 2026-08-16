// Port Hero — an interactive, enterprise-grade local port manager.
//
// Usage:
//
//	port                          interactive list of all listening ports
//	port 3000                     interactive detail view for port 3000
//	port node                     interactive list filtered by process name
//	port 3000 --why               causality chain: why is it running?
//	port node --why               causality for every matching process
//	port --pid 1234 --why         causality for a specific PID
//	port 3000 --kill              graceful kill (SIGTERM, tree-aware)
//	port 3000 --force             force kill (SIGKILL after grace)
//	port 3000 --restart           kill & restart the command detached
//	port --pid 1234 --kill        kill a specific PID (no port needed)
//	port 3000 --kill --all        kill every process on the port
//	port 3000 --kill --dry-run    show what would happen, send nothing
//	port --file /path/to/file     who holds a lock on this file?
//	port --json                   machine-readable dump of all listeners
//	port 3000 --json              machine-readable dump of one port
//	port 3000 --kill --json       machine-readable kill result
//	port --completion bash        print a shell completion script
//	port --version                print version
//
// Exit codes: 0 ok · 1 warnings · 2 not found · 3 blocked · 4 invalid · 5 error
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SystemEndgame/port-hero/internal/ancestry"
	"github.com/SystemEndgame/port-hero/internal/cli"
	"github.com/SystemEndgame/port-hero/internal/completion"
	"github.com/SystemEndgame/port-hero/internal/config"
	"github.com/SystemEndgame/port-hero/internal/guardrails"
	"github.com/SystemEndgame/port-hero/internal/inspector"
	"github.com/SystemEndgame/port-hero/internal/killer"
	"github.com/SystemEndgame/port-hero/internal/locks"
	"github.com/SystemEndgame/port-hero/internal/restart"
	"github.com/SystemEndgame/port-hero/internal/tui"
)

var version = "dev"

type options struct {
	port        int
	name        string
	pid         int
	file        string
	kill        bool
	force       bool
	restart     bool
	all         bool
	dryRun      bool
	why         bool
	json        bool
	check       bool
	wait        bool
	next        bool
	timeout     time.Duration
	protocol    string
	completion  string
	logLevel    string
	logFormat   string
	noAltScreen bool
	version     bool
	help        bool
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fatalExit(err, cli.ExitInvalid)
	}
	if handleMetaFlags(&opts) {
		return
	}

	// Load the optional ~/.port-hero/config.yaml, then apply CLI overrides.
	cfg := loadConfig(&opts)

	// File lock queries.
	if opts.file != "" {
		runFileLocks(&opts)
		return
	}

	// Causality queries.
	if opts.why {
		runWhy(&opts)
		return
	}

	// Kill / force / restart must be handled before the plain --json dump so
	// that `port 3000 --kill --json` produces a kill result, not a listing.
	if opts.kill || opts.force || opts.restart {
		runAction(&opts, cfg)
		return
	}

	if opts.json {
		runJSON(&opts)
		return
	}

	// CI / scripting helpers.
	if opts.check {
		runCheck(&opts)
		return
	}
	if opts.wait {
		runWait(&opts)
		return
	}
	if opts.next {
		runNext(&opts)
		return
	}

	runTUI(&opts, cfg)
}

// checkPort reports whether a port is currently busy. Exit 0 = busy, 2 =
// free, so `port --check 3000 && echo busy || echo free` works in scripts.
func runCheck(o *options) {
	if o.port <= 0 {
		fatalExit(fmt.Errorf("--check requires a port (e.g. port --check 3000)"), cli.ExitInvalid)
	}
	_, err := inspector.FindByPortProto(o.port, o.protocol)
	if err == inspector.ErrPortFree {
		fmt.Printf("port %d is free\n", o.port)
		os.Exit(cli.ExitNotFound)
	}
	if err != nil {
		fatalExit(err, cli.ExitInternal)
	}
	fmt.Printf("port %d is busy\n", o.port)
	os.Exit(cli.ExitOK)
}

// waitForFree polls until the port is free or the timeout elapses.
func runWait(o *options) {
	if o.port <= 0 {
		fatalExit(fmt.Errorf("--wait requires a port (e.g. port --wait 3000 --timeout 30s)"), cli.ExitInvalid)
	}
	timeout := o.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := inspector.FindByPortProto(o.port, o.protocol); err == inspector.ErrPortFree {
			fmt.Printf("port %d is free\n", o.port)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "⛔ port %d still busy after %s\n", o.port, timeout)
	os.Exit(cli.ExitWarnings)
}

// nextFreePort prints the first free port at or above the given start.
func runNext(o *options) {
	start := o.port
	if start <= 0 {
		start = 3000
	}
	procs, err := inspector.FindAllProto(o.protocol)
	if err != nil {
		fatalExit(err, cli.ExitInternal)
	}
	busy := make(map[int]bool, len(procs))
	for _, p := range procs {
		busy[p.Port] = true
	}
	for p := start; p <= 65535; p++ {
		if !busy[p] {
			fmt.Println(p)
			return
		}
	}
	fatalExit(fmt.Errorf("no free port found starting at %d", start), cli.ExitNotFound)
}

// handleMetaFlags handles the immediate-exit flags (--help, --version,
// --completion). It reports whether the caller should return.
func handleMetaFlags(o *options) bool {
	if o.help {
		printUsage()
		return true
	}
	if o.version {
		fmt.Printf("port-hero %s\n", version)
		return true
	}
	if o.completion != "" {
		script, err := completion.Script(o.completion)
		if err != nil {
			fatalExit(err, cli.ExitInvalid)
		}
		fmt.Print(script)
		return true
	}
	return false
}

// loadConfig loads the user config file, applies CLI overrides, configures
// structured logging and the Safety Shield.
func loadConfig(o *options) *config.Config {
	cfg, err := config.LoadFromHome()
	if err != nil {
		fatalExit(err, cli.ExitInvalid)
	}
	if o.logLevel != "" {
		cfg.LogLevel = o.logLevel
	}
	if o.logFormat != "" {
		cfg.LogFormat = o.logFormat
	}
	setupLogging(cfg.LogLevel, cfg.LogFormat)
	guardrails.Configure(cfg)
	return cfg
}

// runTUI starts the interactive Bubble Tea interface.
func runTUI(o *options, cfg *config.Config) {
	m := tui.New(o.port, o.name, cfg.GracePeriod)
	var p *tea.Program
	if o.noAltScreen {
		// Inline rendering — no alternate screen buffer (VHS, scripts).
		p = tea.NewProgram(m)
	} else {
		p = tea.NewProgram(m, tea.WithAltScreen())
	}
	if _, err := p.Run(); err != nil {
		fatalExit(err, cli.ExitInternal)
	}
}

// setupLogging configures the process-wide structured logger (stdlib slog).
// JSON format is intended for CI pipelines and log aggregation.
func setupLogging(level, format string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func parseArgs(args []string) (options, error) {
	o := options{protocol: "tcp"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case isBoolFlag(a):
			setBoolFlag(&o, a)
		case isValueFlag(a):
			val, next, err := flagValue(args, i, a)
			if err != nil {
				return o, err
			}
			i = next
			if err := setValueFlag(&o, a, val); err != nil {
				return o, err
			}
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("unknown flag: %s", a)
		default:
			setPositional(&o, a)
		}
	}
	if err := o.validate(); err != nil {
		return o, err
	}
	return o, nil
}

// validate checks cross-flag constraints. It is separate so parseArgs stays
// a simple loop and the rules are easy to test.
func (o *options) validate() error {
	if err := o.validateTargets(); err != nil {
		return err
	}
	return o.validateLogging()
}

// validateTargets checks flag combinations that affect what the command acts on.
func (o *options) validateTargets() error {
	switch {
	case o.port > 0 && o.name != "":
		return fmt.Errorf("cannot combine a port (%d) and a process name (%q)", o.port, o.name)
	case o.all && !o.kill && !o.force && !o.restart:
		return fmt.Errorf("--all requires --kill, --force or --restart")
	case o.dryRun && !o.kill && !o.force && !o.restart:
		return fmt.Errorf("--dry-run requires --kill, --force or --restart")
	case o.check && o.port <= 0:
		return fmt.Errorf("--check requires a port (e.g. port --check 3000)")
	case o.wait && o.port <= 0:
		return fmt.Errorf("--wait requires a port (e.g. port --wait 3000 --timeout 30s)")
	}
	return nil
}

// validateLogging validates the structured-logging flags.
func (o *options) validateLogging() error {
	if o.logLevel != "" {
		switch o.logLevel {
		case "debug", "info", "warn", "error":
		default:
			return fmt.Errorf("--log-level must be one of debug|info|warn|error")
		}
	}
	if o.logFormat != "" {
		switch o.logFormat {
		case "text", "json":
		default:
			return fmt.Errorf("--log-format must be one of text|json")
		}
	}
	return nil
}

// isBoolFlag reports whether a is a standalone boolean flag.
func isBoolFlag(a string) bool {
	switch a {
	case "--kill", "-k", "--force", "-F", "--restart", "-r", "--why",
		"--json", "-j", "--all", "--dry-run", "--check", "--wait", "--next",
		"--udp", "--no-altscreen", "--version", "-v", "--help", "-h":
		return true
	}
	return false
}

func setBoolFlag(o *options, a string) {
	switch a {
	case "--kill", "-k":
		o.kill = true
	case "--force", "-F":
		o.force, o.kill = true, true
	case "--restart", "-r":
		o.restart = true
	case "--all":
		o.all = true
	case "--dry-run":
		o.dryRun = true
	case "--why":
		o.why = true
	case "--check":
		o.check = true
	case "--wait":
		o.wait = true
	case "--next":
		o.next = true
	case "--udp":
		o.protocol = "udp"
	case "--json", "-j":
		o.json = true
	case "--no-altscreen":
		o.noAltScreen = true
	case "--version", "-v":
		o.version = true
	case "--help", "-h":
		o.help = true
	}
}

// isValueFlag reports whether a requires a following value argument.
func isValueFlag(a string) bool {
	return a == "--pid" || a == "--file" || a == "--completion" ||
		a == "--log-level" || a == "--log-format" || a == "--timeout" ||
		a == "--protocol"
}

// flagValue returns the value following a value flag.
func flagValue(args []string, i int, flag string) (val string, next int, err error) {
	if i+1 >= len(args) {
		return "", i, fmt.Errorf("%s requires a value", flag)
	}
	return args[i+1], i + 1, nil
}

func setValueFlag(o *options, a, val string) error {
	switch a {
	case "--pid":
		p, err := strconv.Atoi(val)
		if err != nil || p <= 0 {
			return fmt.Errorf("invalid --pid: %q", val)
		}
		o.pid = p
	case "--file":
		o.file = val
	case "--completion":
		o.completion = val
	case "--log-level":
		o.logLevel = val
	case "--log-format":
		o.logFormat = val
	case "--timeout":
		d, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid --timeout %q (use e.g. 30s, 2m)", val)
		}
		o.timeout = d
	case "--protocol":
		if err := inspector.ValidateProtocol(val); err != nil {
			return err
		}
		o.protocol = val
	}
	return nil
}

func setPositional(o *options, a string) {
	if p, err := strconv.Atoi(a); err == nil {
		o.port = p
		return
	}
	o.name = a
}

func printUsage() {
	fmt.Print(`⚓ PORT HERO — Local Port Manager & Process Tracer

USAGE:
  port                     Interactive list of all listening ports
  port 3000                Interactive detail for port 3000
  port node                Interactive list filtered by process name
  port 3000 --why          Trace causality: why is this running?
  port node --why          Trace causality for matching processes
  port --pid 1234 --why    Trace causality for a PID
  port 3000 --kill         Graceful kill (SIGTERM, whole process tree)
  port 3000 --force        Force kill (SIGKILL after grace period)
  port 3000 --restart      Kill & restart the command detached
  port --pid 1234 --kill   Kill a specific PID by number
  port 3000 --kill --all   Kill every process listening on the port
  port 3000 --kill --dry-run   Preview without sending any signal
  port --file /path        Show which process holds a lock on a file
  port --json              Machine-readable list of all listeners
  port 3000 --json         Machine-readable detail for one port
  port 3000 --kill --json  Machine-readable kill result
  port --completion bash   Print shell completion (bash|zsh|fish)
  port --log-level debug   Structured logging level (debug|info|warn|error)
  port --log-format json   Structured log format (text|json)
  port --check 3000        Exit 0 if busy, 2 if free (CI scripts)
  port --wait 3000         Wait until port 3000 is free (default 30s)
  port --wait 3000 --timeout 90s   Wait with a custom timeout
  port --next 3000         Print the first free port at or above 3000
  port 53 --udp            Query UDP (default is TCP)
  port --protocol udp 53   Same as above (tcp|udp)
  port --version           Print version

EXIT CODES:
  0 ok · 1 warnings · 2 not found · 3 blocked by safety shield · 4 invalid · 5 error

CONFIGURATION:
  Optional file ~/.port-hero/config.yaml — grace_period, whitelist ports and
  processes, extra protected ports/daemons, log level and format.

SAFETY:
  The Safety Shield refuses to touch PID 1, kernel threads, system daemons
  (sshd, launchd, docker…), foreign users' processes, or protected system
  ports (22 SSH, 53 DNS, 80, 443…). Use --force only to bypass the
  warning-level protections — critical protections always stay active.
  Before signalling, the target's identity (owner + start time) is
  re-verified to prevent PID-reuse attacks.

Built with ❤ by GoLive — free, zero-knowledge dev tools at golive.ly
`)
}

// Causality tracing (the "why is this running?" engine).

func runWhy(o *options) {
	targets, code := whyTargets(o)
	if code != 0 {
		os.Exit(code)
	}
	if o.json {
		chains := make([]*ancestry.Chain, 0, len(targets))
		for _, p := range targets {
			if c, err := ancestry.Build(p.PID); err == nil {
				chains = append(chains, c)
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(chains); err != nil {
			fatalExit(err, cli.ExitInternal)
		}
		return
	}
	for i, p := range targets {
		if i > 0 {
			fmt.Println()
		}
		printWhy(p)
	}
}

// whyTargets resolves the target(s) for a causality query.
func whyTargets(o *options) (targets []*inspector.Process, code int) {
	switch {
	case o.pid > 0:
		p, err := inspector.GetProcess(o.pid)
		if err != nil {
			fmt.Println("⛔ process not found:", err)
			return nil, cli.ExitNotFound
		}
		return []*inspector.Process{p}, 0
	case o.port > 0:
		procs, err := inspector.FindByPortProto(o.port, o.protocol)
		if err != nil {
			if err == inspector.ErrPortFree {
				fmt.Printf("Nothing is listening on port %d (%s).\n", o.port, o.protocol)
				return nil, cli.ExitNotFound
			}
			fatalExit(err, cli.ExitInternal)
		}
		return procs, 0
	case o.name != "":
		procs, err := inspector.FindByName(o.name, 0)
		if err != nil {
			fatalExit(err, cli.ExitInternal)
		}
		if len(procs) == 0 {
			fmt.Printf("No running process matches %q.\n", o.name)
			return nil, cli.ExitNotFound
		}
		return procs, 0
	default:
		fmt.Println("⛔ --why requires a port, a process name, or --pid.")
		return nil, cli.ExitInvalid
	}
}

func printWhy(p *inspector.Process) {
	chain, err := ancestry.Build(p.PID)
	if err != nil {
		fmt.Printf("⛔ could not trace %d: %v\n", p.PID, err)
		return
	}
	fmt.Printf("Target    : %s (pid %d)\n", p.Name, p.PID)
	if p.User != "" {
		fmt.Printf("User      : %s\n", p.User)
	}
	if p.Project != "" {
		fmt.Printf("Project   : %s\n", p.Project)
	}
	if p.Command != "" {
		fmt.Printf("Command   : %s\n", p.Command)
	}
	if p.CWD != "" {
		fmt.Printf("Working Dir: %s\n", p.CWD)
	}
	if p.GitBranch != "" {
		fmt.Printf("Git Branch: %s\n", gitStatusLabel(p))
	}
	if p.Container != "" {
		fmt.Printf("Container : %s\n", p.Container)
	}
	if chain.Service != "" {
		fmt.Printf("Service   : %s\n", chain.Service)
	}
	if chain.Session != "" {
		fmt.Printf("Session   : %s\n", chain.Session)
	}
	fmt.Printf("Started By: %s\n", chain.Source)
	fmt.Printf("Why It Runs:\n%s\n", chain.Tree())
	for _, w := range chain.Warnings {
		fmt.Printf("⚠ %s\n", w)
	}
}

func gitStatusLabel(p *inspector.Process) string {
	if p.GitDirty {
		return fmt.Sprintf("%s [DIRTY]", p.GitBranch)
	}
	return fmt.Sprintf("%s [CLEAN]", p.GitBranch)
}

// File locks.

func runFileLocks(o *options) {
	holders, err := locks.ByFile(o.file)
	if err != nil {
		fatalExit(err, cli.ExitInternal)
	}
	if o.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(holders)
		return
	}
	if len(holders) == 0 {
		fmt.Printf("No process holds a lock on %s.\n", o.file)
		os.Exit(cli.ExitNotFound)
	}
	fmt.Printf("File: %s\n", o.file)
	for _, h := range holders {
		fmt.Printf("  Locked by: %s (pid %d) [%s %s]\n", h.Name, h.PID, h.Type, h.Mode)
		if h.Command != "" {
			fmt.Printf("      Command: %s\n", h.Command)
		}
	}
}

// JSON.

func runJSON(o *options) {
	var (
		procs []*inspector.Process
		err   error
	)
	switch {
	case o.port > 0:
		procs, err = inspector.FindByPortProto(o.port, o.protocol)
	case o.name != "":
		procs, err = inspector.FindByName(o.name, 0)
	case o.pid > 0:
		var p *inspector.Process
		p, err = inspector.GetProcess(o.pid)
		if p != nil {
			procs = []*inspector.Process{p}
		}
	default:
		procs, err = inspector.FindAllProto(o.protocol)
	}
	if err != nil {
		if err == inspector.ErrPortFree {
			fmt.Print("[]")
			os.Exit(cli.ExitNotFound)
		}
		fatalExit(err, cli.ExitInternal)
	}
	if procs == nil {
		procs = []*inspector.Process{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(procs); err != nil {
		fatalExit(err, cli.ExitInternal)
	}
}

// Kill / force / restart.

// runAction resolves the target process(es) and executes kill/force/restart.
// It supports --pid targeting, --all for multi-process ports, --dry-run
// previews and --json machine-readable output.
func runAction(o *options, cfg *config.Config) {
	procs, code := actionTargets(o)
	if code != 0 {
		os.Exit(code)
	}
	if len(procs) > 1 && !o.all {
		fmt.Printf("⚠ Multiple processes are listening on port %d:\n", o.port)
		for _, p := range procs {
			fmt.Printf("   • %s (PID %d)\n", p.Name, p.PID)
		}
		fmt.Println("Use --all to kill them all, or target one with: port --pid <PID> --kill")
		os.Exit(cli.ExitInvalid)
	}

	var (
		killResults []killer.Result
		restarts    []restart.Result
	)
	warned := false

	for _, p := range procs {
		if o.restart {
			rr := performRestartOne(p, o, cfg)
			restarts = append(restarts, rr)
			if rr.Error != nil {
				warned = true
			}
			continue
		}
		res, w := performKillOne(p, o, cfg)
		killResults = append(killResults, res)
		if w {
			warned = true
		}
	}

	if o.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if o.restart {
			if restarts == nil {
				restarts = []restart.Result{}
			}
			_ = enc.Encode(restarts)
		} else {
			if killResults == nil {
				killResults = []killer.Result{}
			}
			_ = enc.Encode(killResults)
		}
	}

	if warned {
		os.Exit(cli.ExitWarnings)
	}
	os.Exit(cli.ExitOK)
}

// performKillOne runs the Safety Shield and kill for a single process,
// returning the result and whether warnings were emitted.
func performKillOne(p *inspector.Process, o *options, cfg *config.Config) (res killer.Result, warned bool) {
	_, warned = guardrailWarnings(p, o)

	var err error
	res, err = killer.Kill(p, killer.Options{
		Force:       o.force,
		Tree:        true,
		GracePeriod: cfg.GracePeriod,
		DryRun:      o.dryRun,
	})
	if err != nil {
		fmt.Println("⛔", err)
		os.Exit(cli.ExitBlocked)
	}
	for _, w := range res.Warnings {
		warned = true
		printWarn(o, w)
	}
	if !o.json {
		fmt.Printf("✅ %s (in %s)\n", res.Summary(), inspector.FormatDuration(res.Elapsed))
	}
	if !res.AllGone {
		warned = true
	}
	return res, warned
}

// performRestartOne runs the Safety Shield and restart for a single process.
func performRestartOne(p *inspector.Process, o *options, cfg *config.Config) restart.Result {
	guardrailWarnings(p, o)
	return runRestartOnce(p, o, cfg)
}

// guardrailWarnings evaluates the Safety Shield for a kill target. Critical
// violations block with exit 3; warning-level violations are printed and
// reported via the returned flag.
func guardrailWarnings(p *inspector.Process, o *options) (active []guardrails.Violation, warned bool) {
	active, _ = guardrails.Check(p, o.force)
	if guardrails.HasCritical(active) {
		// Blocked output goes to stdout in text mode, stderr in JSON mode
		// so the JSON stream stays clean (we exit before emitting it).
		fmt.Println("⛔ BLOCKED by Safety Shield:")
		for _, v := range active {
			fmt.Println("   •", v)
		}
		os.Exit(cli.ExitBlocked)
	}
	for _, v := range active {
		warned = true
		printWarn(o, v.String())
	}
	return active, warned
}

// printWarn routes a warning to stderr in JSON mode so stdout stays clean.
func printWarn(o *options, msg string) {
	if o.json {
		fmt.Fprintln(os.Stderr, "⚠", msg)
	} else {
		fmt.Println("⚠", msg)
	}
}

// actionTargets resolves the process(es) targeted by a kill/force/restart.
func actionTargets(o *options) (procs []*inspector.Process, code int) {
	switch {
	case o.pid > 0:
		p, err := inspector.GetProcess(o.pid)
		if err != nil {
			fmt.Println("⛔ process not found:", err)
			return nil, cli.ExitNotFound
		}
		return []*inspector.Process{p}, 0
	case o.port > 0:
		procs, err := inspector.FindByPortProto(o.port, o.protocol)
		if err != nil {
			if err == inspector.ErrPortFree {
				fmt.Printf("Nothing listening on port %d (%s).\n", o.port, o.protocol)
				return nil, cli.ExitNotFound
			}
			fatalExit(err, cli.ExitInternal)
		}
		return procs, 0
	default:
		fmt.Println("⛔ a port or --pid is required for --kill / --force / --restart")
		return nil, cli.ExitInvalid
	}
}

// runRestartOnce kills a single process and respawns it detached.
//
// Identity safety in the restart flow: killer.Kill re-verifies the ORIGINAL
// process (p.PID) before signalling it — the respawned process only comes
// into existence afterwards via restart.Restart and receives a brand-new
// PID, so reverify can never mistake the new process for the old one.
func runRestartOnce(p *inspector.Process, o *options, cfg *config.Config) restart.Result {
	res, err := killer.Kill(p, killer.Options{
		Force:       o.force,
		Tree:        true,
		GracePeriod: cfg.GracePeriod,
		DryRun:      o.dryRun,
	})
	if err != nil {
		fmt.Println("⛔", err)
		os.Exit(cli.ExitBlocked)
	}
	if o.dryRun {
		return restart.Result{}
	}

	rr := restart.Restart(p, p.CWD)
	if rr.Error != nil {
		if !o.json {
			fmt.Printf("⚠ killed: %s\n", res.Summary())
			fmt.Println("⛔ restart failed:", rr.Error)
		} else {
			fmt.Fprintln(os.Stderr, "⛔ restart failed:", rr.Error)
		}
	} else if !o.json {
		fmt.Printf("✅ %s in %s → respawned as PID %d (log: %s)\n",
			res.Summary(), inspector.FormatDuration(res.Elapsed), rr.NewPID, rr.Log)
	}
	return rr
}

func fatalExit(err error, code int) {
	if err == nil {
		err = fmt.Errorf("unknown error")
	}
	fmt.Fprintln(os.Stderr, "⛔ port-hero:", err)
	os.Exit(code)
}
