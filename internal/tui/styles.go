package tui

import "github.com/charmbracelet/lipgloss"

// Theme — clean, professional, terminal-native.
var (
	// Palette.
	cyan      = lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#22d3ee"}
	green     = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#4ade80"}
	yellow    = lipgloss.AdaptiveColor{Light: "#a16207", Dark: "#facc15"}
	red       = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f87171"}
	muted     = lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#64748b"}
	gray      = lipgloss.AdaptiveColor{Light: "#94a3b8", Dark: "#475569"}
	borderCol = lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#334155"}

	// Components.
	titleStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true).
			MarginLeft(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(muted).
			MarginLeft(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderCol).
			Padding(0, 1)

	listItem = lipgloss.NewStyle().PaddingLeft(2)

	listItemSel = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true).
			PaddingLeft(1)

	dim = lipgloss.NewStyle().Foreground(gray)

	okTag = lipgloss.NewStyle().
		Foreground(green).
		Bold(true).
		Background(lipgloss.AdaptiveColor{Light: "#dcfce7", Dark: "#14532d"}).
		Padding(0, 1).
		MarginRight(1)

	warnTag = lipgloss.NewStyle().
		Foreground(yellow).
		Bold(true).
		Background(lipgloss.AdaptiveColor{Light: "#fef9c3", Dark: "#713f12"}).
		Padding(0, 1).
		MarginRight(1)

	critTag = lipgloss.NewStyle().
		Foreground(red).
		Bold(true).
		Background(lipgloss.AdaptiveColor{Light: "#fee2e2", Dark: "#7f1d1d"}).
		Padding(0, 1).
		MarginRight(1)

	infoTag = lipgloss.NewStyle().
		Foreground(cyan).
		Bold(true).
		Background(lipgloss.AdaptiveColor{Light: "#cffafe", Dark: "#155e75"}).
		Padding(0, 1).
		MarginRight(1)

	keyHint = lipgloss.NewStyle().
		Foreground(cyan).
		Bold(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(muted).
			MarginTop(1).
			MarginLeft(1)

	helpLine = lipgloss.NewStyle().
			Foreground(gray).
			MarginLeft(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	branchClean = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	portBadge = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true)

	pidBadge = lipgloss.NewStyle().
			Foreground(muted)
)

// KeyHint renders " [X] label" pairs.
func keyHintLine(pairs ...string) string {
	var out string
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			out += "  "
		}
		out += keyHint.Render(pairs[i]) + " " + dim.Render(pairs[i+1])
	}
	return helpLine.Render(out)
}

// lipglossSpinnerStyle styles the progress spinner.
func lipglossSpinnerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cyan).Bold(true)
}

// projectStyle highlights the project name — the emotional hook of the tool.
func projectStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(cyan).
		Bold(true).
		Background(lipgloss.AdaptiveColor{Light: "#cffafe", Dark: "#155e75"}).
		Padding(0, 1)
}
