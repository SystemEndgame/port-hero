package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SystemEndgame/port-hero/internal/guardrails"
	"github.com/SystemEndgame/port-hero/internal/inspector"
	"github.com/SystemEndgame/port-hero/internal/killer"
	"github.com/SystemEndgame/port-hero/internal/restart"
)

// Version is stamped at build time via -ldflags.
var Version = "dev"

// ---------------------------------------------------------------------------
// Views.
// ---------------------------------------------------------------------------

type view int

const (
	viewLoading view = iota
	viewList
	viewDetail
	viewConfirm
	viewWorking
	viewResult
	viewError
)

// ActionType is the user-chosen kill action.
type ActionType int

const (
	// ActionGraceful sends SIGTERM and waits the grace period.
	ActionGraceful ActionType = iota
	// ActionForce sends SIGKILL after the grace period.
	ActionForce
	// ActionRestart kills gracefully and respawns the command detached.
	ActionRestart
)

// Label returns the human-readable name of the action.
func (a ActionType) Label() string {
	switch a {
	case ActionGraceful:
		return "Graceful Kill (SIGTERM)"
	case ActionForce:
		return "Force Kill (SIGKILL)"
	case ActionRestart:
		return "Kill & Restart"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Messages.
// ---------------------------------------------------------------------------

type loadedMsg struct {
	procs []*inspector.Process
	err   error
}

type focusedMsg struct {
	procs []*inspector.Process
	err   error
}

type opDoneMsg struct {
	action  ActionType
	res     killer.Result
	killErr error
	restart restart.Result
	proc    *inspector.Process
}

// ---------------------------------------------------------------------------
// Model.
// ---------------------------------------------------------------------------

// Model is the top-level Bubble Tea model. It owns the current view state,
// the process list, the focused process, and any in-flight operation.
type Model struct {
	view view

	procs     []*inspector.Process
	cursor    int
	focusPort int    // when > 0, open directly on this port's detail view
	filter    string // when non-empty, only show matching process names

	focused     []*inspector.Process // processes on the focused port
	focusCursor int

	confirm *confirmState
	working *workingState
	result  *resultState
	spinner spinner.Model

	width  int
	height int
	err    error
}

type confirmState struct {
	action    ActionType
	proc      *inspector.Process
	active    []guardrails.Violation
	bypassed  []guardrails.Violation
	force     bool
	forceUsed bool
}

type workingState struct {
	action ActionType
	proc   *inspector.Process
}

type resultState struct {
	action  ActionType
	res     killer.Result
	killErr error
	restart restart.Result
}

// New creates the TUI model. If port > 0, the UI opens directly on that
// port's detail view. If filter is non-empty, only processes whose name
// matches are listed.
func New(port int, filter string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipglossSpinnerStyle()

	m := Model{
		view:      viewLoading,
		cursor:    0,
		focusPort: port,
		filter:    filter,
		spinner:   s,
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.focusPort > 0 {
		return tea.Batch(func() tea.Msg {
			procs, err := findPort(m.focusPort)
			return focusedMsg{procs: procs, err: err}
		}, m.spinner.Tick)
	}
	return tea.Batch(loadAll(m.filter), m.spinner.Tick)
}

// loadAll fetches every listening process (optionally filtered by name).
func loadAll(filter string) tea.Cmd {
	return func() tea.Msg {
		if filter != "" {
			procs, err := inspector.FindByName(filter, 200)
			return loadedMsg{procs: procs, err: err}
		}
		procs, err := inspector.FindAll()
		return loadedMsg{procs: procs, err: err}
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case loadedMsg:
		m.view = viewList
		if msg.err != nil {
			m.view = viewError
			m.err = msg.err
			return m, nil
		}
		m.procs = msg.procs
		m.cursor = 0
		return m, nil

	case focusedMsg:
		if msg.err != nil {
			m.view = viewError
			m.err = msg.err
			return m, nil
		}
		m.focused = msg.procs
		m.focusCursor = 0
		if len(m.focused) == 0 {
			m.err = fmt.Errorf("port is free or no longer occupied")
			m.view = viewError
			return m, nil
		}
		m.view = viewDetail
		return m, nil

	case opDoneMsg:
		m.view = viewResult
		m.result = &resultState{
			action:  msg.action,
			res:     msg.res,
			killErr: msg.killErr,
			restart: msg.restart,
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

// handleKey routes keys by current view.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewList:
		return m.handleListKey(msg)
	case viewDetail:
		return m.handleDetailKey(msg)
	case viewConfirm:
		return m.handleConfirmKey(msg)
	case viewResult, viewError:
		return m.handleResultKey(msg)
	case viewWorking, viewLoading:
		return m, nil // ignore input while busy
	}
	return m, nil
}

// handleResultKey: any key returns to list (or quits on q/esc).
func (m Model) handleResultKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	m.view = viewList
	return m, nil
}

// currentProcess returns the focused process in the current view.
func (m Model) currentProcess() *inspector.Process {
	switch m.view {
	case viewList:
		if len(m.procs) == 0 || m.cursor >= len(m.procs) {
			return nil
		}
		return m.procs[m.cursor]
	case viewDetail:
		if len(m.focused) == 0 || m.focusCursor >= len(m.focused) {
			return nil
		}
		return m.focused[m.focusCursor]
	}
	return nil
}

// setWorking switches to the spinner view and returns the op Cmd.
func (m *Model) setWorking(action ActionType, p *inspector.Process) tea.Cmd {
	m.view = viewWorking
	m.working = &workingState{action: action, proc: p}
	var cmd tea.Cmd
	switch action {
	case ActionRestart:
		cmd = executeRestart(p)
	case ActionForce:
		cmd = executeKill(p, true)
	default:
		cmd = executeKill(p, false)
	}
	return cmd
}

// executeKill runs a tree-aware graceful/force kill in a goroutine.
func executeKill(p *inspector.Process, force bool) tea.Cmd {
	return func() tea.Msg {
		res, err := killer.Kill(p, killer.Options{
			Force: force,
			Tree:  true,
		})
		return opDoneMsg{action: ActionGraceful, res: res, killErr: err, proc: p}
	}
}

// executeRestart kills gracefully and respawns the command detached.
func executeRestart(p *inspector.Process) tea.Cmd {
	return func() tea.Msg {
		res, err := killer.Kill(p, killer.Options{
			Force: false,
			Tree:  true,
		})
		var rr restart.Result
		if err == nil {
			rr = restart.Restart(p, p.CWD)
		}
		return opDoneMsg{
			action:  ActionRestart,
			res:     res,
			killErr: err,
			restart: rr,
			proc:    p,
		}
	}
}

// findPort re-resolves a port to fresh process data.
func findPort(port int) ([]*inspector.Process, error) {
	return inspector.FindByPort(port)
}

// trim wraps long strings for the detail view.
func trimWidth(s string, w int) string {
	if w <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= w {
		return s
	}
	return string(runes[:w-1]) + "…"
}
