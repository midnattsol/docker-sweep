package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Soft color palette - easier on the eyes
var (
	// Primary colors (softer tones)
	Purple    = lipgloss.Color("#C084FC") // lavanda/magenta suave
	Pink      = lipgloss.Color("#F0ABFC") // pink suave
	Cyan      = lipgloss.Color("#22D3EE") // cyan claro
	Green     = lipgloss.Color("#34D399") // verde menta
	Yellow    = lipgloss.Color("#FBBF24") // amarillo suave
	Red       = lipgloss.Color("#F87171") // rojo suave
	Blue      = lipgloss.Color("#60A5FA") // azul suave
	Gray      = lipgloss.Color("#9CA3AF")
	DarkGray  = lipgloss.Color("#4B5563")
	LightGray = lipgloss.Color("#D1D5DB")
)

// Styles
var (
	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Blue)

	// Subtitle / info
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(LightGray)

	// Success
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Green)

	// Warning
	WarningStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	// Error
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Red)

	// Muted / dim text
	MutedStyle = lipgloss.NewStyle().
			Foreground(Gray)

	// Bold
	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	// Resource name (container, image, etc.)
	ResourceStyle = lipgloss.NewStyle().
			Foreground(Cyan).
			Bold(true)

	// Checkmark
	CheckStyle = lipgloss.NewStyle().
			Foreground(Green).
			SetString("✓")

	// Cross
	CrossStyle = lipgloss.NewStyle().
			Foreground(Red).
			SetString("✗")

	// Circle (skipped)
	CircleStyle = lipgloss.NewStyle().
			Foreground(Gray).
			SetString("◦")

	// Box styles for header/footer
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DarkGray).
			Padding(0, 1)

	// Divider
	DividerStyle = lipgloss.NewStyle().
			Foreground(DarkGray)

	// Selected item in list
	SelectedStyle = lipgloss.NewStyle().
			Foreground(Purple).
			Bold(true)

	// Cursor
	CursorStyle = lipgloss.NewStyle().
			Foreground(Pink).
			SetString("›")

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(LightGray)

	// Size style
	SizeStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	// Protected style
	ProtectedStyle = lipgloss.NewStyle().
			Foreground(Gray).
			Italic(true)
)

// Divider returns a horizontal divider line
func Divider(width int) string {
	return DividerStyle.Render(strings.Repeat("─", width))
}

// RenderStatsBox renders a stats box with parts joined by separator
func RenderStatsBox(parts []string) string {
	content := strings.Join(parts, MutedStyle.Render(" · "))
	box := BoxStyle.Render(content)
	return "\n" + Indent(box, 2) + "\n"
}

// Indent adds n spaces to each line
func Indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

// KeyStyle renders a keyboard key
var KeyStyle = lipgloss.NewStyle().
	Foreground(Cyan)

// RenderHelp renders a help line with styled keys
// Format: key1 action1 · key2 action2 · ...
func RenderHelp(items [][2]string) string {
	var parts []string
	for _, item := range items {
		key := KeyStyle.Render(item[0])
		action := MutedStyle.Render(item[1])
		parts = append(parts, key+" "+action)
	}
	return strings.Join(parts, MutedStyle.Render("  ·  "))
}
