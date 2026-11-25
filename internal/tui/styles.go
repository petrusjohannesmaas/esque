package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// warnGlyph marks warnings. It is U+26A0 without the emoji variation
// selector, so terminals draw it one cell wide. Change it to "!" if a terminal
// draws it badly.
const warnGlyph = "⚠"

// okGlyph marks a finished action.
const okGlyph = "✓"

// Layout limits for the frame.
const (
	maxFrameWidth  = 72
	maxFrameHeight = 28
)

// The palette. Each colour has a light-terminal and a dark-terminal value.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#9D94FF"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6E6E6E", Dark: "#8E8E8E"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#D4D4D4", Dark: "#3A3A3A"}
	colAmber  = lipgloss.AdaptiveColor{Light: "#A15C00", Dark: "#F2B53A"}
	colRed    = lipgloss.AdaptiveColor{Light: "#C0262D", Dark: "#FF6B6B"}
	colGreen  = lipgloss.AdaptiveColor{Light: "#1F7A3A", Dark: "#5FD38D"}
)

var (
	frameStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colFaint).
			Padding(0, 2)

	brandStyle  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	crumbStyle  = lipgloss.NewStyle().Foreground(colMuted)
	crumbHere   = lipgloss.NewStyle()
	ruleStyle   = lipgloss.NewStyle().Foreground(colFaint)
	titleStyle  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colMuted)
	accentStyle = lipgloss.NewStyle().Foreground(colAccent)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(colMuted).Faint(true)
	warnStyle   = lipgloss.NewStyle().Foreground(colAmber)
	errorStyle  = lipgloss.NewStyle().Foreground(colRed)
	dangerStyle = lipgloss.NewStyle().Foreground(colRed)
	okStyle     = lipgloss.NewStyle().Foreground(colGreen)

	selectedStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	keyStyle  = lipgloss.NewStyle().Foreground(colAccent)
	descStyle = lipgloss.NewStyle().Foreground(colMuted)

	secretStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAmber).
			Padding(1, 4).
			Bold(true)

	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(1, 3)
)

func init() {
	// https://no-color.org: any non-empty value turns colour off.
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// marker returns the cursor column for a row: an accent "▸" when selected.
func marker(selected bool) string {
	if selected {
		return accentStyle.Render("▸") + " "
	}
	return "  "
}

// wrap word-wraps s to width cells, cutting a word longer than width.
func wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Wrap(s, width, "")
}

// rule returns a faint horizontal line width cells wide.
func rule(width int) string {
	if width <= 0 {
		return ""
	}
	return ruleStyle.Render(strings.Repeat("─", width))
}

// truncate cuts s to width cells, ending in "…" when it cuts.
func truncate(s string, width int) string {
	return ansi.Truncate(s, max(width, 0), "…")
}
