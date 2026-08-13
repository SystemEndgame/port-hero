package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SystemEndgame/port-hero/internal/ancestry"
	"github.com/SystemEndgame/port-hero/internal/inspector"
)

// View implements tea.Model.
func (m Model) View() string {
	var body string
	switch m.view {
	case viewLoading:
		body = m.spinner.View() + " Scanning listening ports…"
	case viewList:
		body = m.renderList()
	case viewDetail:
		body = m.renderDetail()
	case viewConfirm:
		body = m.renderConfirm()
	case viewWorking:
		body = m.renderWorking()
	case viewResult:
		body = m.renderResult()
	case viewError:
		body = m.renderError()
	}

	title := titleStyle.Render("⚓ PORT HERO ") + dim.Render("v"+Version)
	sub := subtitleStyle.Render("Local port manager — inspect, kill & restart safely")

	var out string
	out += title + "\n"
	out += sub + "\n\n"
	out += body
	out += "\n\n" + footerStyle.Render("Built with ❤ by GoLive — free, zero-knowledge dev tools at golive.ly")

	return lipgloss.NewStyle().MaxWidth(m.width).Render(out)
}

// ---------------------------------------------------------------------------
// List view.
// ---------------------------------------------------------------------------

func (m Model) renderList() string {
	if len(m.procs) == 0 {
		return infoTag.Render("No ports are currently in LISTEN state")
	}

	var lines []string
	lines = append(lines,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#e2e8f0"}).
			Render(fmt.Sprintf("%-8s %-10s %-22s %-14s %s",
				"PORT", "PID", "PROCESS", "USER", "GIT BRANCH")),
	)

	start := 0
	if m.cursor > 8 {
		start = m.cursor - 4
	}
	end := start + 12
	if end > len(m.procs) {
		end = len(m.procs)
	}
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		p := m.procs[i]
		// Pad BEFORE styling so ANSI escapes don't break column alignment.
		portCol := padRight(fmt.Sprintf("%d", p.Port), 8)
		pidCol := padRight(fmt.Sprintf("%d", p.PID), 10)
		nameCol := padRight(trimWidth(p.Name, 22), 22)
		userCol := padRight(trimWidth(p.User, 14), 14)
		row := portBadge.Render(portCol) + " " + dim.Render(pidCol) +
			" " + nameCol + " " + userCol + " " + gitLabel(p)
		if i == m.cursor {
			lines = append(lines, listItemSel.Render(row))
		} else {
			lines = append(lines, listItem.Render(row))
		}
	}

	body := strings.Join(lines, "\n")

	hints := keyHintLine(
		"↑/↓", "navigate",
		"enter", "inspect",
		"r", "refresh",
		"q", "quit",
	)
	return boxStyle.Render(body) + "\n\n" + hints +
		"\n" + dim.Render(fmt.Sprintf("  %d listening port(s)", len(m.procs)))
}

// padRight pads a string to width with spaces (rune-aware).
func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// gitLabel renders branch + clean/dirty badge.
func gitLabel(p *inspector.Process) string {
	if p.GitBranch == "" {
		return dim.Render("—")
	}
	b := branchClean.Render("⎇ " + trimWidth(p.GitBranch, 18))
	if p.GitDirty {
		b += " " + warnTag.Render("DIRTY")
	} else {
		b += " " + okTag.Render("CLEAN")
	}
	return b
}

// ---------------------------------------------------------------------------
// Detail view.
// ---------------------------------------------------------------------------

func (m Model) renderDetail() string {
	if len(m.focused) == 0 {
		return errorStyle.Render("No process found.")
	}
	p := m.focused[m.focusCursor]
	if p == nil {
		return errorStyle.Render("No process found.")
	}

	var rows []string
	rows = append(rows,
		fmt.Sprintf("%-14s %s", "Port:", portBadge.Render(fmt.Sprintf("%d (%s)", p.Port, protoOf(p)))),
		fmt.Sprintf("%-14s %s", "Process:", m.focusedName(p)),
		fmt.Sprintf("%-14s %s", "PID:", pidBadge.Render(fmt.Sprintf("%d", p.PID))),
		fmt.Sprintf("%-14s %s", "User:", p.User),
	)
	if p.Project != "" {
		rows = append(rows, fmt.Sprintf("%-14s %s", "Project:", projectStyle().Render(p.Project)))
	}
	if p.MemoryMB > 0 || p.CPUPercent > 0 {
		rows = append(rows, fmt.Sprintf("%-14s %.1f MB | %.1f%% CPU", "Resources:", p.MemoryMB, p.CPUPercent))
	}
	if p.LocalAddr != "" {
		rows = append(rows, fmt.Sprintf("%-14s %s", "Address:", p.LocalAddr))
	}
	if p.Container != "" {
		rows = append(rows, fmt.Sprintf("%-14s %s", "Container:", infoTag.Render(p.Container)))
	}
	if p.CWD != "" {
		rows = append(rows, fmt.Sprintf("%-14s %s", "Directory:", trimWidth(p.CWD, 60)))
	}
	if p.GitBranch != "" {
		rows = append(rows, fmt.Sprintf("%-14s %s", "Git Branch:", gitLabel(p)))
	}
	if p.Command != "" {
		rows = append(rows, fmt.Sprintf("%-14s %s", "Command:", trimWidth(p.Command, 66)))
	}

	box := boxStyle.Render(strings.Join(rows, "\n"))

	// Tree summary + causality chain.
	extra := ""
	if treeLine := m.treeSummary(p); treeLine != "" {
		extra = "\n" + dim.Render(treeLine)
	}
	if chain := m.causalityLine(p); chain != "" {
		extra += "\n" + infoTag.Render("WHY") + " " + dim.Render(chain)
	}

	hints := keyHintLine(
		"space/k", "graceful kill",
		"f", "force kill",
		"r", "kill & restart",
		"b", "back",
		"q", "quit",
	)

	multi := ""
	if len(m.focused) > 1 {
		multi = "\n" + dim.Render(fmt.Sprintf("  %d process(es) share this port — use j/k to switch", len(m.focused)))
	}

	return box + extra + multi + "\n\n" + hints
}

func (m Model) focusedName(p *inspector.Process) string {
	name := p.Name
	if name == "" {
		name = trimWidth(p.Command, 40)
	}
	return lipgloss.NewStyle().Bold(true).Render(name)
}

func protoOf(p *inspector.Process) string {
	if p.Protocol == "udp" {
		return "UDP"
	}
	return "TCP"
}

// treeSummary shows descendant count for orphan-prevention awareness.
func (m Model) treeSummary(root *inspector.Process) string {
	snap, err := inspector.AllProcesses()
	if err != nil {
		return ""
	}
	s := inspector.NewSnapshot(snap)
	desc := s.Descendants(root.PID)
	if len(desc) == 0 {
		return fmt.Sprintf("🔗 %d is a leaf process — no children to protect.", root.PID)
	}
	names := make([]string, 0, len(desc))
	for _, d := range desc {
		names = append(names, fmt.Sprintf("%s(%d)", d.Name, d.PID))
	}
	return fmt.Sprintf("🔗 Process tree: %d child process(es) will be terminated first — no orphans left behind.\n   %s",
		len(desc), trimWidth(strings.Join(names, " → "), 70))
}

// causalityLine renders a compact causal chain for the focused process.
func (m Model) causalityLine(root *inspector.Process) string {
	chain, err := ancestry.Build(root.PID)
	if err != nil || len(chain.Nodes) == 0 {
		return ""
	}
	return chain.Text()
}

// ---------------------------------------------------------------------------
// Confirm view.
// ---------------------------------------------------------------------------

func (m Model) renderConfirm() string {
	c := m.confirm
	if c == nil {
		return ""
	}
	p := c.proc

	var b strings.Builder
	b.WriteString(warnTag.Render("SAFETY CONFIRMATION") + "\n\n")
	fmt.Fprintf(&b, "You are about to %s:\n\n", lipgloss.NewStyle().Bold(true).Render(c.action.Label()))
	fmt.Fprintf(&b, "  %s %s\n\n", portBadge.Render(fmt.Sprintf(":%d", p.Port)), p.Name+" (PID "+fmt.Sprint(p.PID)+")")

	if len(c.active) > 0 {
		b.WriteString(dim.Render("The safety shield found these warnings:\n"))
		for _, v := range c.active {
			b.WriteString("  ⚠ " + warnTag.Render(v.Code) + dim.Render(v.Message) + "\n")
		}
		b.WriteString("\n")
	}
	if c.forceUsed && c.force {
		b.WriteString(okTag.Render("FORCE MODE") + dim.Render(" Warning-level protections bypassed. Critical protections remain active.") + "\n\n")
	}

	b.WriteString(keyHintLine(
		"y/enter", "confirm",
		"f", "toggle force",
		"esc", "cancel",
	))
	return boxStyle.Render(b.String())
}

// ---------------------------------------------------------------------------
// Working view.
// ---------------------------------------------------------------------------

func (m Model) renderWorking() string {
	if m.working == nil {
		return ""
	}
	label := m.working.action.Label()
	return boxStyle.Render(
		m.spinner.View() + " " + label + " " +
			fmt.Sprintf(": %d (PID %d)…", m.working.proc.Port, m.working.proc.PID) + "\n\n" +
			dim.Render("Sending SIGTERM first, waiting 1.5s, then SIGKILL only if needed."),
	)
}

// ---------------------------------------------------------------------------
// Result view.
// ---------------------------------------------------------------------------

func (m Model) renderResult() string {
	r := m.result
	if r == nil {
		return ""
	}

	var b strings.Builder
	switch {
	case r.killErr != nil:
		b.WriteString(critTag.Render("BLOCKED") + "\n")
		b.WriteString(dim.Render(r.killErr.Error()) + "\n\n")
	case r.action == ActionRestart && r.restart.NewPID > 0:
		b.WriteString(okTag.Render("RESTARTED") + "\n")
		fmt.Fprintf(&b, "  %s killed gracefully (%s).\n", r.res.Summary(), inspector.FormatDuration(r.res.Elapsed))
		fmt.Fprintf(&b, "  New PID %d started from %s\n", r.restart.NewPID, r.restart.Log)
	case r.res.Graceful:
		b.WriteString(okTag.Render("GRACEFUL") + "\n")
		fmt.Fprintf(&b, "  %s — clean shutdown in %s.\n", r.res.Summary(), inspector.FormatDuration(r.res.Elapsed))
	case !r.res.AllGone:
		b.WriteString(critTag.Render("PARTIAL") + "\n")
		fmt.Fprintf(&b, "  %s\n", r.res.Summary())
	default:
		b.WriteString(warnTag.Render("FORCE-KILLED") + "\n")
		fmt.Fprintf(&b, "  %s — SIGKILL was required for %d process(es) after the 1.5s grace period.\n",
			r.res.Summary(), len(r.res.ForceKilled))
	}
	b.WriteString("\n" + keyHintLine("any key", "back to list", "q", "quit"))

	return boxStyle.Render(b.String())
}

// ---------------------------------------------------------------------------
// Error view.
// ---------------------------------------------------------------------------

func (m Model) renderError() string {
	return errorStyle.Render("⛔ "+fmt.Sprint(m.err)) + "\n\n" +
		keyHintLine("any key", "back", "q", "quit")
}
