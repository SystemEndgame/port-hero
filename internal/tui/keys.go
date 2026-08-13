package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/golive-ly/port-hero/internal/guardrails"
)

// ---------------------------------------------------------------------------
// List view.
// ---------------------------------------------------------------------------

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.procs)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		if len(m.procs) > 0 {
			m.cursor = len(m.procs) - 1
		}
	case "enter":
		if len(m.procs) == 0 {
			return m, nil
		}
		port := m.procs[m.cursor].Port
		return m, tea.Batch(
			func() tea.Msg {
				procs, err := findPort(port)
				return focusedMsg{procs: procs, err: err}
			},
		)
	case "r":
		// Refresh the list.
		return m, loadAll(m.filter)
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Detail view.
// ---------------------------------------------------------------------------

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "b", "left":
		// Back to the list.
		m.view = viewList
		return m, nil
	case "j", "down":
		if len(m.focused) > 1 && m.focusCursor < len(m.focused)-1 {
			m.focusCursor++
		}
	case "k", "up":
		if m.focusCursor > 0 {
			m.focusCursor--
		}
	case " ", "K":
		return m.beginAction(ActionGraceful)
	case "F":
		return m.beginAction(ActionForce)
	case "R":
		return m.beginAction(ActionRestart)
	}
	return m, nil
}

// beginAction runs the guardrail pre-check and routes to confirm or execute.
func (m Model) beginAction(action ActionType) (tea.Model, tea.Cmd) {
	p := m.currentProcess()
	if p == nil {
		return m, nil
	}
	active, bypassed := guardrails.Check(p, false)

	// Critical violations block immediately.
	if guardrails.HasCritical(active) {
		m.view = viewError
		m.err = active[0]
		return m, nil
	}

	// Warnings present → require explicit confirmation (with force option).
	if len(active) > 0 {
		m.view = viewConfirm
		m.confirm = &confirmState{
			action:   action,
			proc:     p,
			active:   active,
			bypassed: bypassed,
		}
		return m, nil
	}

	// Clean — execute immediately.
	cmd := m.setWorking(action, p)
	return m, cmd
}

// ---------------------------------------------------------------------------
// Confirm view.
// ---------------------------------------------------------------------------

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.confirm
	if c == nil {
		m.view = viewList
		return m, nil
	}
	switch msg.String() {
	case "esc", "q", "ctrl+c":
		// Cancel returns to the detail view.
		m.view = viewDetail
		m.confirm = nil
		return m, nil
	case "f":
		// Toggle force: bypass warning-level violations.
		if !guardrails.HasCritical(c.active) {
			c.force = !c.force
			c.forceUsed = true
			if c.force {
				_, c.bypassed = guardrails.Check(c.proc, true)
				// With force, no active warnings remain.
				active, _ := guardrails.Check(c.proc, true)
				c.active = active
			} else {
				active, bypassed := guardrails.Check(c.proc, false)
				c.active, c.bypassed = active, bypassed
			}
			return m, nil
		}
	case "enter", "y", "Y":
		// Execute — with force applied only if toggled.
		action := c.action
		if action == ActionGraceful && c.force {
			action = ActionForce
		}
		p := c.proc
		m.confirm = nil
		// Re-verify with force setting.
		active, _ := guardrails.Check(p, c.force)
		if guardrails.HasCritical(active) {
			m.view = viewError
			m.err = active[0]
			return m, nil
		}
		cmd := m.setWorking(action, p)
		return m, cmd
	case "r":
		// Restart with force semantics.
		c.force = false
		m.confirm = nil
		cmd := m.setWorking(ActionRestart, c.proc)
		return m, cmd
	}
	return m, nil
}
