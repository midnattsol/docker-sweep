package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/midnattsol/docker-sweep/internal/sweep"
)

// RenderHeader renders the header for docker sweep.
func RenderHeader() string {
	title := TitleStyle.Render("docker-sweep")
	return fmt.Sprintf("\n  %s\n", title)
}

// RenderSummary renders summary after deletion.
func RenderSummary(deleted int, total int) string {
	content := fmt.Sprintf("Deleted %s of %s resources",
		SuccessStyle.Render(fmt.Sprintf("%d", deleted)),
		BoldStyle.Render(fmt.Sprintf("%d", total)))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(DarkGray).
		Padding(0, 1).
		Render(content)

	return fmt.Sprintf("\n%s\n\n", Indent(box, 2))
}

// RenderError renders an error message.
func RenderError(msg string) string {
	return fmt.Sprintf("\n  %s %s\n\n", CrossStyle.Render(), ErrorStyle.Render(msg))
}

// RenderErrorInline renders an inline error (for loops).
func RenderErrorInline(msg string) string {
	return fmt.Sprintf("%s %s", CrossStyle.Render(), ErrorStyle.Render(msg))
}

// RenderNoResources renders message when no resources are available for deletion.
func RenderNoResources() string {
	return fmt.Sprintf("\n  %s %s\n\n", CheckStyle.Render(), MutedStyle.Render("No resources to delete."))
}

// RenderDryRun renders what would be deleted in dry-run mode.
func RenderDryRun(resources []sweep.Resource) string {
	var s string
	s += fmt.Sprintf("\n  %s\n\n", WarningStyle.Render("Dry run - would delete:"))

	for _, r := range resources {
		s += fmt.Sprintf("    %s %s %s\n",
			CircleStyle.Render(),
			ResourceStyle.Render(r.DisplayName()),
			MutedStyle.Render(fmt.Sprintf("(%s)", r.Type())))
	}

	s += "\n"
	return s
}

// FormatSize formats bytes into human readable string.
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
