package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/midnattsol/docker-sweep/internal/sweep"
)

// PickerItem represents an item in the picker
type PickerItem struct {
	Resource sweep.Resource
	Selected bool
	Disabled bool
}

// PickerModel is a bubbletea model for multi-select
type PickerModel struct {
	items      []PickerItem
	cursor     int
	scrollTop  int
	termWidth  int
	termHeight int
	quitting   bool
	confirmed  bool
	totalSize  int64
}

// NewPicker creates a new picker from sweep results
func NewPicker(result *sweep.Result) PickerModel {
	var items []PickerItem

	// Add containers
	for i := range result.Containers {
		r := &result.Containers[i]
		items = append(items, PickerItem{
			Resource: r,
			Selected: r.IsSuggested(),
			Disabled: r.IsProtected(),
		})
	}

	// Add images
	for i := range result.Images {
		r := &result.Images[i]
		items = append(items, PickerItem{
			Resource: r,
			Selected: r.IsSuggested(),
			Disabled: r.IsProtected(),
		})
	}

	// Add volumes
	for i := range result.Volumes {
		r := &result.Volumes[i]
		items = append(items, PickerItem{
			Resource: r,
			Selected: r.IsSuggested(),
			Disabled: r.IsProtected(),
		})
	}

	// Add networks
	for i := range result.Networks {
		r := &result.Networks[i]
		items = append(items, PickerItem{
			Resource: r,
			Selected: r.IsSuggested(),
			Disabled: r.IsProtected(),
		})
	}

	m := PickerModel{items: items}
	m.updateTotalSize()
	return m
}

func (m *PickerModel) updateTotalSize() {
	var total int64
	for _, item := range m.items {
		if item.Selected && !item.Disabled {
			total += item.Resource.Size()
		}
	}
	m.totalSize = total
}

func (m PickerModel) Init() tea.Cmd {
	return nil
}

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.ensureCursorVisible()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			m.confirmed = true
			return m, tea.Quit

		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.items) - 1
			}
			m.ensureCursorVisible()

		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.items) {
				m.cursor = 0
			}
			m.ensureCursorVisible()

		case "pgup", "ctrl+b":
			m.moveCursorBy(-(m.listViewportHeight() - 1))

		case "pgdown", "ctrl+f":
			m.moveCursorBy(m.listViewportHeight() - 1)

		case "home", "g":
			m.cursor = 0
			m.ensureCursorVisible()

		case "end", "G":
			m.cursor = len(m.items) - 1
			m.ensureCursorVisible()

		case " ":
			// Toggle selection
			if !m.items[m.cursor].Disabled {
				m.items[m.cursor].Selected = !m.items[m.cursor].Selected
				m.updateTotalSize()
			}

		case "a":
			// Select all non-disabled
			for i := range m.items {
				if !m.items[i].Disabled {
					m.items[i].Selected = true
				}
			}
			m.updateTotalSize()

		case "n":
			// Select none
			for i := range m.items {
				m.items[i].Selected = false
			}
			m.updateTotalSize()

		case "s":
			// Select only suggested
			for i := range m.items {
				if !m.items[i].Disabled {
					m.items[i].Selected = m.items[i].Resource.IsSuggested()
				}
			}
			m.updateTotalSize()
		}
	}

	return m, nil
}

func (m PickerModel) View() string {
	var b strings.Builder
	widths := m.computeColumnWidths()
	rows := m.renderRows(widths)

	viewportHeight := m.listViewportHeight()
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	maxTop := len(rows) - viewportHeight
	if maxTop < 0 {
		maxTop = 0
	}
	if m.scrollTop > maxTop {
		m.scrollTop = maxTop
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}

	b.WriteString(RenderHeader())
	b.WriteString(fmt.Sprintf("\n  %s\n", MutedStyle.Render("Select resources to delete:")))
	b.WriteString("\n")

	start := m.scrollTop
	end := m.scrollTop + viewportHeight
	if end > len(rows) {
		end = len(rows)
	}
	for _, row := range rows[start:end] {
		b.WriteString(row + "\n")
	}

	if len(rows) > viewportHeight {
		b.WriteString(fmt.Sprintf("  %s\n", MutedStyle.Render(
			fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(rows)),
		)))
	}

	// Footer with help and stats
	b.WriteString(fmt.Sprintf("\n  %s\n", Divider(60)))

	help := RenderHelp([][2]string{
		{"␣", "toggle"},
		{"pgup/pgdn", "scroll"},
		{"a", "all"},
		{"s", "suggested"},
		{"↵", "confirm"},
		{"q", "quit"},
	})
	b.WriteString(fmt.Sprintf("  %s\n", help))

	// Show space to recover
	if m.totalSize > 0 {
		b.WriteString(fmt.Sprintf("\n  %s %s\n",
			MutedStyle.Render("Space to recover:"),
			SizeStyle.Render("~"+FormatSize(m.totalSize))))
	}

	b.WriteString("\n")

	return b.String()
}

func (m *PickerModel) moveCursorBy(delta int) {
	if len(m.items) == 0 {
		return
	}
	if delta == 0 {
		delta = 1
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	m.ensureCursorVisible()
}

func (m *PickerModel) listViewportHeight() int {
	height := m.termHeight
	if height <= 0 {
		height = 24
	}

	reserved := 11
	if m.totalSize > 0 {
		reserved++
	}

	viewport := height - reserved
	if viewport < 5 {
		viewport = 5
	}

	return viewport
}

func (m *PickerModel) ensureCursorVisible() {
	if len(m.items) == 0 {
		m.scrollTop = 0
		return
	}

	rowIndex := m.rowIndexForItem(m.cursor)
	viewport := m.listViewportHeight()

	if rowIndex < m.scrollTop {
		m.scrollTop = rowIndex
	}
	if rowIndex >= m.scrollTop+viewport {
		m.scrollTop = rowIndex - viewport + 1
	}

	maxTop := m.totalRows() - viewport
	if maxTop < 0 {
		maxTop = 0
	}
	if m.scrollTop > maxTop {
		m.scrollTop = maxTop
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
}

func (m PickerModel) totalRows() int {
	rows := 0
	currentType := sweep.ResourceType("")
	for _, item := range m.items {
		if item.Resource.Type() != currentType {
			if currentType != "" {
				rows++ // blank separator row between sections
			}
			currentType = item.Resource.Type()
			rows++
		}
		rows++
	}
	return rows
}

func (m PickerModel) rowIndexForItem(itemIndex int) int {
	if itemIndex < 0 {
		return 0
	}
	if itemIndex >= len(m.items) {
		itemIndex = len(m.items) - 1
	}

	row := 0
	currentType := sweep.ResourceType("")
	for i, item := range m.items {
		if item.Resource.Type() != currentType {
			if currentType != "" {
				row++ // blank separator row
			}
			currentType = item.Resource.Type()
			row++
		}
		if i == itemIndex {
			return row
		}
		row++
	}

	return 0
}

func (m PickerModel) renderRows(widths pickerColumnWidths) []string {
	rows := make([]string, 0, m.totalRows())
	currentType := sweep.ResourceType("")

	for i, item := range m.items {
		if item.Resource.Type() != currentType {
			if currentType != "" {
				rows = append(rows, "")
			}
			currentType = item.Resource.Type()
			count := m.countByType(currentType)
			rows = append(rows, fmt.Sprintf("  %s", typeHeader(currentType, count)))
		}

		cursor := "  "
		if i == m.cursor {
			cursor = CursorStyle.Render() + " "
		}

		var checkbox string
		if item.Disabled {
			checkbox = MutedStyle.Render("▢")
		} else if item.Selected {
			checkbox = SuccessStyle.Render("▣")
		} else {
			checkbox = "▢"
		}

		name := item.Resource.DisplayName()
		if i == m.cursor && !item.Disabled {
			name = SelectedStyle.Render(name)
		} else if item.Disabled {
			name = MutedStyle.Render(name)
		} else {
			name = ResourceStyle.Render(name)
		}

		details := item.Resource.Details()
		if item.Disabled {
			details = ProtectedStyle.Render(details)
		} else {
			details = MutedStyle.Render(details)
		}

		size := ""
		if item.Resource.Size() > 0 {
			size = SizeStyle.Render(FormatSize(item.Resource.Size()))
		}

		compose := ""
		if project := sweep.GetComposeProject(item.Resource); project != "" {
			compose = MutedStyle.Render("[" + project + "]")
		}

		line := cursor + checkbox + " " +
			padRight(name, widths.name) + "  " +
			padRight(details, widths.details)

		if widths.size > 0 {
			line += "  " + padLeft(size, widths.size)
		}

		if widths.compose > 0 {
			line += "  " + padRight(compose, widths.compose)
		}

		rows = append(rows, strings.TrimRight(line, " "))
	}

	return rows
}

type pickerColumnWidths struct {
	name    int
	details int
	size    int
	compose int
}

func (m PickerModel) computeColumnWidths() pickerColumnWidths {
	var w pickerColumnWidths

	for _, item := range m.items {
		nameWidth := lipgloss.Width(item.Resource.DisplayName())
		if nameWidth > w.name {
			w.name = nameWidth
		}

		detailsWidth := lipgloss.Width(item.Resource.Details())
		if detailsWidth > w.details {
			w.details = detailsWidth
		}

		sizeText := ""
		if item.Resource.Size() > 0 {
			sizeText = FormatSize(item.Resource.Size())
		}
		sizeWidth := lipgloss.Width(sizeText)
		if sizeWidth > w.size {
			w.size = sizeWidth
		}

		composeText := ""
		if project := sweep.GetComposeProject(item.Resource); project != "" {
			composeText = "[" + project + "]"
		}
		composeWidth := lipgloss.Width(composeText)
		if composeWidth > w.compose {
			w.compose = composeWidth
		}
	}

	return w
}

func padRight(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func padLeft(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

func (m PickerModel) countByType(t sweep.ResourceType) int {
	count := 0
	for _, item := range m.items {
		if item.Resource.Type() == t && !item.Disabled {
			count++
		}
	}
	return count
}

func typeHeader(t sweep.ResourceType, count int) string {
	var icon, name string
	switch t {
	case sweep.TypeContainer:
		icon = "📦"
		name = "Containers"
	case sweep.TypeImage:
		icon = "🖼"
		name = "Images"
	case sweep.TypeVolume:
		icon = "💾"
		name = "Volumes"
	case sweep.TypeNetwork:
		icon = "🌐"
		name = "Networks"
	}

	return fmt.Sprintf("%s %s %s",
		icon,
		BoldStyle.Render(name),
		MutedStyle.Render(fmt.Sprintf("(%d)", count)))
}

// Cancelled returns true if user quit without confirming
func (m PickerModel) Cancelled() bool {
	return m.quitting
}

// SelectedResources returns the selected resources
func (m PickerModel) SelectedResources() []sweep.Resource {
	var selected []sweep.Resource
	for _, item := range m.items {
		if item.Selected && !item.Disabled {
			selected = append(selected, item.Resource)
		}
	}
	return selected
}

// RunPicker runs the interactive picker and returns selected resources
func RunPicker(result *sweep.Result) ([]sweep.Resource, error) {
	m := NewPicker(result)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	fm := finalModel.(PickerModel)
	if fm.Cancelled() {
		return nil, nil // User cancelled
	}

	return fm.SelectedResources(), nil
}
