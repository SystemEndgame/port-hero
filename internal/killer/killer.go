// Package killer implements graceful, tree-aware process termination:
// SIGTERM to the whole tree first, a short grace period, then SIGKILL for
// anything that refused to die. All safety checks are enforced by
// guardrails before any signal is sent, and the target process identity is
// re-verified immediately before signalling to close the TOCTOU window.
package killer

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/SystemEndgame/port-hero/internal/ancestry"
	"github.com/SystemEndgame/port-hero/internal/guardrails"
	"github.com/SystemEndgame/port-hero/internal/inspector"
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

// Result reports the outcome of a kill operation. It is JSON-serialisable so
// CI pipelines can consume `port N --kill --json`.
type Result struct {
	PID         int           `json:"pid"`
	ProcessName string        `json:"process_name,omitempty"`
	Terminated  []int         `json:"terminated,omitempty"`   // PIDs that exited after SIGTERM
	ForceKilled []int         `json:"force_killed,omitempty"` // PIDs that needed SIGKILL
	Failed      []int         `json:"failed,omitempty"`       // PIDs that could not be signalled
	Protected   []int         `json:"protected,omitempty"`    // PIDs blocked by guardrails
	Warnings    []string      `json:"warnings,omitempty"`     // non-blocking observations
	Elapsed     time.Duration `json:"-"`                      // internal; exported as ElapsedMS
	ElapsedMS   int64         `json:"elapsed_ms,omitempty"`   // wall-clock duration in milliseconds
	Graceful    bool          `json:"graceful"`               // true if no SIGKILL was needed
	AllGone     bool          `json:"all_gone"`               // true if the whole tree is gone
}

// MarshalJSON renders the result with Elapsed as milliseconds (encoding/json
// would otherwise emit a raw time.Duration in nanoseconds).
func (r *Result) MarshalJSON() (data []byte, err error) {
	type resultAlias Result
	return json.Marshal(struct {
		resultAlias
		ElapsedMS int64 `json:"elapsed_ms,omitempty"`
	}{
		resultAlias: resultAlias(*r),
		ElapsedMS:   r.Elapsed.Milliseconds(),
	})
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

	log := slog.With("pid", p.PID, "name", p.Name, "port", p.Port)

	// -- Safety shield: guardrail checks -------------------------------
	active, _ := guardrails.Check(p, opts.Force)
	if guardrails.HasCritical(active) {
		log.Warn("kill blocked by safety shield", "violation", active[0].Message)
		return Result{PID: p.PID, ProcessName: p.Name, Protected: []int{p.PID}}, fmt.Errorf("blocked by guardrails: %s", active[0].Message)
	}

	res := Result{PID: p.PID, ProcessName: p.Name}

	// -- Supervisor auto-restart warning (non-blocking) -----------------
	// launchd/systemd may respawn a managed process after we kill it; warn
	// so the user is not surprised when their "stopped" daemon comes back.
	if !opts.DryRun {
		if managed, manager, detail := ancestry.AutoRestart(p.PID); managed {
			msg := fmt.Sprintf("managed by %s (%s) — it may restart automatically after being killed", manager, detail)
			res.Warnings = append(res.Warnings, msg)
			log.Warn("kill target is supervisor-managed", "manager", manager, "detail", detail)
		}
	}

	// -- Re-verify identity immediately before signalling ---------------
	// Closes the TOCTOU window: the PID may have been reused or the owner
	// may have changed between inspection and now.
	if err := reverify(p); err != nil {
		log.Warn("kill aborted: target identity changed", "error", err)
		return res, fmt.Errorf("refusing to signal: %w", err)
	}

	// Determine the kill set (children first, root last).
	targets := []int{p.PID}
	if opts.Tree {
		if snap, err := inspector.AllProcesses(); err == nil {
			s := inspector.NewSnapshot(snap)
			targets = s.TreeOrder(p.PID)
		}
	}

	if opts.DryRun {
		res.Graceful = true
		res.AllGone = true
		res.Terminated = targets
		log.Info("dry run — would terminate tree", "targets", len(targets))
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
			log.Warn("SIGTERM failed", "target_pid", pid, "error", err)
			continue
		}
		stillAlive[pid] = true
	}
	log.Info("SIGTERM sent to process tree", "targets", len(targets))

	// -- Grace period --------------------------------------------------
	waitGrace(&res, stillAlive, opts.GracePeriod)

	res.Elapsed = time.Since(start)
	res.Graceful = len(stillAlive) == 0

	// -- Phase 2: SIGKILL survivors (children first) --------------------
	// Defence in depth: re-verify the root once more before SIGKILL. A PID
	// recycled during the grace window must never receive SIGKILL — it is
	// reported as failed instead (the window is ~1.5s, so this is a
	// theoretical race, but the check costs one /proc read).
	if stillAlive[p.PID] {
		if err := reverify(p); err != nil {
			log.Warn("SIGKILL aborted: root identity changed during grace", "error", err)
			res.Failed = append(res.Failed, p.PID)
			delete(stillAlive, p.PID)
		}
	}
	forceKill(&res, targets, stillAlive)

	res.AllGone = len(res.Failed) == 0
	log.Info("kill complete",
		"graceful", res.Graceful,
		"force_killed", len(res.ForceKilled),
		"failed", len(res.Failed),
		"elapsed_ms", res.Elapsed.Milliseconds(),
	)
	return res, nil
}

// reverify confirms the process is still the exact process we inspected:
// alive, same owner, and (on platforms that expose it) the same start time.
func reverify(p *inspector.Process) error {
	if !inspector.IsAlive(p.PID) {
		return fmt.Errorf("process %d already exited", p.PID)
	}
	fresh, err := inspector.VerifyProcess(p.PID)
	if err != nil {
		return fmt.Errorf("process %d disappeared during verification: %w", p.PID, err)
	}
	if p.StartTime > 0 && fresh.StartTime > 0 && fresh.StartTime != p.StartTime {
		return fmt.Errorf("PID %d was reused by a different process (start time changed); aborting", p.PID)
	}
	if p.User != "" && fresh.User != "" && fresh.User != p.User {
		return fmt.Errorf("process %d owner changed from %q to %q; aborting", p.PID, p.User, fresh.User)
	}
	return nil
}

// forceKill sends SIGKILL to the survivors in the original (children-first)
// order and verifies the whole tree is gone.
func forceKill(res *Result, targets []int, stillAlive map[int]bool) {
	if len(stillAlive) == 0 {
		return
	}
	log := slog.With("pid", res.PID)
	for _, pid := range targets {
		if !stillAlive[pid] {
			continue
		}
		if err := send(pid, SignalKill); err != nil {
			res.Failed = append(res.Failed, pid)
			log.Warn("SIGKILL failed", "target_pid", pid, "error", err)
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
