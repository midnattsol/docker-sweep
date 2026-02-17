package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// SpinnerModel is a bubbletea model for showing a spinner with a message
type SpinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
	done     bool
	err      error
}

// SpinnerDoneMsg signals the spinner should stop
type SpinnerDoneMsg struct {
	Err error
}

// NewSpinner creates a new spinner model
func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Blue)
	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case SpinnerDoneMsg:
		m.done = true
		m.err = msg.Err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m SpinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("  %s %s\n", CrossStyle.Render(), m.message)
		}
		return fmt.Sprintf("  %s %s\n", CheckStyle.Render(), m.message)
	}
	return fmt.Sprintf("  %s %s\n", m.spinner.View(), MutedStyle.Render(m.message))
}

// IsTTY returns true if stdout is a terminal
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// RunWithSpinner executes a function while showing a spinner
// Returns error if the function fails or user cancels
// Falls back to simple text output if not a TTY
func RunWithSpinner(message string, fn func() error) error {
	// Fallback for non-TTY environments
	if !IsTTY() {
		fmt.Printf("  %s %s\n", MutedStyle.Render("●"), MutedStyle.Render(message))
		err := fn()
		if err != nil {
			fmt.Printf("  %s %s\n", CrossStyle.Render(), message)
		} else {
			fmt.Printf("  %s %s\n", CheckStyle.Render(), message)
		}
		return err
	}

	m := NewSpinner(message)

	p := tea.NewProgram(m)

	// Run the function in background
	go func() {
		err := fn()
		p.Send(SpinnerDoneMsg{Err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if user quit
	if fm, ok := finalModel.(SpinnerModel); ok {
		if fm.quitting {
			return fmt.Errorf("cancelled")
		}
		if fm.err != nil {
			return fm.err
		}
	}

	return nil
}

// MultiSpinner handles multiple sequential spinners
type MultiSpinner struct {
	tasks []SpinnerTask
}

type SpinnerTask struct {
	Message string
	Fn      func() error
}

func NewMultiSpinner() *MultiSpinner {
	return &MultiSpinner{}
}

func (ms *MultiSpinner) Add(message string, fn func() error) {
	ms.tasks = append(ms.tasks, SpinnerTask{Message: message, Fn: fn})
}

func (ms *MultiSpinner) Run() error {
	for _, task := range ms.tasks {
		if err := RunWithSpinner(task.Message, task.Fn); err != nil {
			return err
		}
	}
	return nil
}
