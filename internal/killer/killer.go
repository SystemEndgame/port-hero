// Package killer implements graceful, tree-aware process termination:
// SIGTERM to the whole tree first, a short grace period, then SIGKILL for
// anything that refused to die. All safety checks are enforced by
// guardrails before any signal is sent.
package killer

import (
	"fmt"
	"strings"
	"time"

	"github.com/golive-ly/port-hero/internal/guardrails"
	"github.com/golive-ly/port-hero/internal/inspector"
)

// DefaultGracePeriod is how long we wait after SIGTERM before SIGKILL.
const DefaultGracePeriod = 1500 * time.Millisecond

// Signal is a process signal abstraction.
type Signal int

const (
	// SignalTerm is SIGTERM — graceful termination.
	SignalTerm Signal = 15
	// SignalKill is SIGKILL — forced termination.
	SignalKill Signal = 9
)

// Options controls a kill operation.
type Options struct {
	// GracePeriod between SIGTERM and SIGKILL. Zero → DefaultGracePeriod.
	GracePeriod time.Duration
	// Force bypasses warning-level guardrail violations (protected ports,
	// low PIDs). Critical violations always block.
	Force bool
	// Tree controls whether the whole process tree is terminated
	// (orphan prevention). Always true in the interactive UI.
	Tree bool
	// DryRun prints what would be done without sending signals.
	DryRun bool
}

// Result reports the outcome of a kill operation.
type Result struct {
	PID         int
	Terminated  []int // PIDs that exited after SIGTERM
	ForceKilled []int // PIDs that needed SIGKILL
	Failed      []int // PIDs that could not be signalled
	Protected   []int // PIDs blocked by guardrails
	Elapsed     time.Duration
	Graceful    bool // true if no SIGKILL was needed
	AllGone     bool // true if the whole tree is gone
}

// Summary renders a compact human-readable outcome of the kill operation.
func (r *Result) Summary() string {
	var parts []string
	if len(r.Terminated) > 0 {
		parts = append(parts, fmt.Sprintf("%d graceful", len(r.Terminated)))
	}
	if len(r.ForceKilled) > 0 {
		parts = append(parts, fmt.Sprintf("%d force-killed", len(r.ForceKilled)))
	}
	if len(r.Failed) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", len(r.Failed)))
	}
	if len(r.Protected) > 0 {
		parts = append(parts, fmt.Sprintf("%d protected", len(r.Protected)))
	}
	if len(parts) == 0 {
		return "nothing done"
	}
	return strings.Join(parts, ", ")
}

// Kill terminates the given process. If opts.Tree is true, the entire
// descendant tree is terminated in post-order (children first).
// It returns an error only when the operation is completely blocked.
func Kill(p *inspector.Process, opts Options) (Result, error) {
	if p == nil {
		return Result{}, fmt.Errorf("no process to kill")
	}
	if opts.GracePeriod <= 0 {
		opts.GracePeriod = DefaultGracePeriod
	}

	// -- Safety shield: guardrail checks -------------------------------
	active, _ := guardrails.Check(p, opts.Force)
	if guardrails.HasCritical(active) {
		return Result{PID: p.PID, Protected: []int{p.PID}}, fmt.Errorf("blocked by guardrails: %s", active[0].Message)
	}

	// Determine the kill set (children first, root last).
	targets := []int{p.PID}
	if opts.Tree {
		if snap, err := inspector.AllProcesses(); err == nil {
			s := inspector.NewSnapshot(snap)
			targets = s.TreeOrder(p.PID)
		}
	}

	res := Result{PID: p.PID}
	if opts.DryRun {
		res.Graceful = true
		res.AllGone = true
		res.Terminated = targets
		return res, nil
	}

	start := time.Now()

	// -- Phase 1: SIGTERM to the whole tree (children first) ----------
	stillAlive := map[int]bool{}
	for _, pid := range targets {
		if !inspector.IsAlive(pid) {
			res.Terminated = append(res.Terminated, pid)
			continue
		}
		if err := send(pid, SignalTerm); err != nil {
			res.Failed = append(res.Failed, pid)
			continue
		}
		stillAlive[pid] = true
	}

	// -- Grace period --------------------------------------------------
	waitGrace(&res, stillAlive, opts.GracePeriod)

	res.Elapsed = time.Since(start)
	res.Graceful = len(stillAlive) == 0

	// -- Phase 2: SIGKILL survivors (children first) --------------------
	forceKill(&res, targets, stillAlive)

	res.AllGone = len(res.Failed) == 0
	return res, nil
}

// forceKill sends SIGKILL to the survivors in the original (children-first)
// order and verifies the whole tree is gone.
func forceKill(res *Result, targets []int, stillAlive map[int]bool) {
	if len(stillAlive) == 0 {
		return
	}
	for _, pid := range targets {
		if !stillAlive[pid] {
			continue
		}
		if err := send(pid, SignalKill); err != nil {
			res.Failed = append(res.Failed, pid)
			continue
		}
		res.ForceKilled = append(res.ForceKilled, pid)
	}
	time.Sleep(80 * time.Millisecond)
	for _, pid := range targets {
		if inspector.IsAlive(pid) {
			res.Failed = append(res.Failed, pid)
		}
	}
}

// waitGrace polls the surviving PIDs until the grace period elapses or
// every process has exited, recording graceful terminations.
func waitGrace(res *Result, stillAlive map[int]bool, grace time.Duration) {
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) && len(stillAlive) > 0 {
		for pid := range stillAlive {
			if !inspector.IsAlive(pid) {
				res.Terminated = append(res.Terminated, pid)
				delete(stillAlive, pid)
			}
		}
		if len(stillAlive) > 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}
