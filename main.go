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
//	port 3000 --force             force kill (SIGKILL after 1.5 s grace)
//	port 3000 --restart           kill & restart the command detached
//	port --file /path/to/file     who holds a lock on this file?
//	port --json                   machine-readable dump of all listeners
//	port 3000 --json              machine-readable dump of one port
//	port --completion bash        print a shell completion script
//	port --version                print version
//
// Exit codes: 0 ok · 1 warnings · 2 not found · 3 blocked · 4 invalid · 5 error
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SystemEndgame/port-hero/internal/ancestry"
	"github.com/SystemEndgame/port-hero/internal/cli"
	"github.com/SystemEndgame/port-hero/internal/completion"
	"github.com/SystemEndgame/port-hero/internal/guardrails"
	"github.com/SystemEndgame/port-hero/internal/inspector"
	"github.com/SystemEndgame/port-hero/internal/killer"
	"github.com/SystemEndgame/port-hero/internal/locks"
	"github.com/SystemEndgame/port-hero/internal/restart"
	"github.com/SystemEndgame/port-hero/internal/tui"
)

var version = "dev"

type options struct {
	port       int
	name       string
	pid        int
	file       string
	kill       bool
	force      bool
	restart    bool
	why        bool
	json       bool
	completion string
	version    bool
	help       bool
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fatalExit(err, cli.ExitInvalid)
	}
	if opts.help {
		printUsage()
		return
	}
	if opts.version {
		fmt.Printf("port-hero v%s\n", version)
		return
	}
	if opts.completion != "" {
		script, err := completion.Script(opts.completion)
		if err != nil {
			fatalExit(err, cli.ExitInvalid)
		}
		fmt.Print(script)
		return
	}

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

	if opts.json {
		runJSON(&opts)
		return
	}

	if opts.kill || opts.force || opts.restart {
		runAction(&opts)
		return
	}

	// Interactive TUI.
	p := tea.NewProgram(tui.New(opts.port, opts.name), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatalExit(err, cli.ExitInternal)
	}
}

func parseArgs(args []string) (options, error) {
	o := options{}
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
	if o.port > 0 && o.name != "" {
		return o, fmt.Errorf("cannot combine a port (%d) and a process name (%q)", o.port, o.name)
	}
	return o, nil
}

// isBoolFlag reports whether a is a standalone boolean flag.
func isBoolFlag(a string) bool {
	switch a {
	case "--kill", "-k", "--force", "-F", "--restart", "-r", "--why",
		"--json", "-j", "--version", "-v", "--help", "-h":
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
	case "--why":
		o.why = true
	case "--json", "-j":
		o.json = true
	case "--version", "-v":
		o.version = true
	case "--help", "-h":
		o.help = true
	}
}

// isValueFlag reports whether a requires a following value argument.
func isValueFlag(a string) bool {
	return a == "--pid" || a == "--file" || a == "--completion"
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
  port 3000 --force        Force kill (SIGKILL after 1.5 s grace)
  port 3000 --restart      Kill & restart the command detached
  port --file /path        Show which process holds a lock on a file
  port --json              Machine-readable list of all listeners
  port 3000 --json         Machine-readable detail for one port
  port --completion bash   Print shell completion (bash|zsh|fish)
  port --version           Print version

EXIT CODES:
  0 ok · 1 warnings · 2 not found · 3 blocked by safety shield · 4 invalid · 5 error

SAFETY:
  The Safety Shield refuses to touch PID 1, kernel threads, system daemons
  (sshd, launchd, docker…), foreign users' processes, or protected system
  ports (22 SSH, 53 DNS, 80, 443…). Use --force only to bypass the
  warning-level protections — critical protections always stay active.

Built with ❤ by GoLive — free, zero-knowledge dev tools at golive.ly
`)
}

// ---------------------------------------------------------------------------
// Causality tracing (the "why is this running?" engine).
// ---------------------------------------------------------------------------

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
		procs, err := inspector.FindByPort(o.port)
		if err != nil {
			if err == inspector.ErrPortFree {
				fmt.Printf("Nothing is listening on port %d.\n", o.port)
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

// ---------------------------------------------------------------------------
// File locks.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// JSON.
// ---------------------------------------------------------------------------

func runJSON(o *options) {
	var (
		procs []*inspector.Process
		err   error
	)
	switch {
	case o.port > 0:
		procs, err = inspector.FindByPort(o.port)
	case o.name != "":
		procs, err = inspector.FindByName(o.name, 0)
	case o.pid > 0:
		var p *inspector.Process
		p, err = inspector.GetProcess(o.pid)
		if p != nil {
			procs = []*inspector.Process{p}
		}
	default:
		procs, err = inspector.FindAll()
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

// ---------------------------------------------------------------------------
// Kill / force / restart.
// ---------------------------------------------------------------------------

func runAction(o *options) {
	if o.port <= 0 {
		fatalExit(fmt.Errorf("a port is required for --kill / --force / --restart (e.g. port 3000 --kill)"), cli.ExitInvalid)
	}
	procs, err := inspector.FindByPort(o.port)
	if err != nil {
		if err == inspector.ErrPortFree {
			fmt.Println("Nothing listening on port", o.port)
			os.Exit(cli.ExitNotFound)
		}
		fatalExit(err, cli.ExitInternal)
	}
	if len(procs) == 0 {
		fmt.Println("Nothing listening on port", o.port)
		os.Exit(cli.ExitNotFound)
	}
	p := procs[0]

	if o.restart {
		runRestart(p)
		return
	}

	// Guardrails first.
	active, _ := guardrails.Check(p, o.force)
	if guardrails.HasCritical(active) {
		fmt.Println("⛔ BLOCKED by Safety Shield:")
		for _, v := range active {
			fmt.Println("   •", v)
		}
		os.Exit(cli.ExitBlocked)
	}
	warned := false
	for _, v := range active {
		warned = true
		fmt.Println("⚠", v)
	}

	res, err := killer.Kill(p, killer.Options{Force: o.force, Tree: true})
	if err != nil {
		fmt.Println("⛔", err)
		os.Exit(cli.ExitBlocked)
	}
	fmt.Printf("✅ %s (in %s)\n", res.Summary(), inspector.FormatDuration(res.Elapsed))
	if !res.AllGone {
		os.Exit(cli.ExitWarnings)
	}
	if warned {
		os.Exit(cli.ExitWarnings)
	}
	os.Exit(cli.ExitOK)
}

func runRestart(p *inspector.Process) {
	active, _ := guardrails.Check(p, false)
	if guardrails.HasCritical(active) {
		fmt.Println("⛔ BLOCKED by Safety Shield:")
		for _, v := range active {
			fmt.Println("   •", v)
		}
		os.Exit(cli.ExitBlocked)
	}
	res, err := killer.Kill(p, killer.Options{Tree: true})
	if err != nil {
		fmt.Println("⛔", err)
		os.Exit(cli.ExitBlocked)
	}
	rr := restart.Restart(p, p.CWD)
	if rr.Error != nil {
		fmt.Println("⚠ killed:", res.Summary())
		fmt.Println("⛔ restart failed:", rr.Error)
		os.Exit(cli.ExitWarnings)
	}
	fmt.Printf("✅ %s in %s → respawned as PID %d (log: %s)\n",
		res.Summary(), inspector.FormatDuration(res.Elapsed), rr.NewPID, rr.Log)
	os.Exit(cli.ExitOK)
}

func fatalExit(err error, code int) {
	if err == nil {
		err = fmt.Errorf("unknown error")
	}
	fmt.Fprintln(os.Stderr, "⛔ port-hero:", err)
	os.Exit(code)
}
