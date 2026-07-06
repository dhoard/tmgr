package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// version is set via ldflags at build time
var version = "dev"

// Session represents a tmux session
type Session struct {
	Name   string
	Windows int
	Attached bool
}

// Styles for the TUI
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#FAFAFA")).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(0, 1)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
)

// model represents the Bubble Tea model
type model struct {
	sessions  []Session
	cursor    int
	selected  *Session
	quitting  bool
	renaming  bool
	newName   string
	err       error
}

// Initial model
func initialModel(sessions []Session) model {
	return model{
		sessions: sessions,
		cursor:   0,
	}
}

// Init initializes the Bubble Tea model
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles user input and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle rename mode
		if m.renaming {
			switch msg.String() {
			case "esc":
				m.renaming = false
				m.newName = ""
				return m, nil

			case "enter":
				if m.newName != "" {
					err := renameSession(m.sessions[m.cursor].Name, m.newName)
					if err != nil {
						m.err = err
					} else {
						m.sessions[m.cursor].Name = m.newName
					}
				}
				m.renaming = false
				m.newName = ""
				return m, nil

			case "backspace":
				if len(m.newName) > 0 {
					m.newName = m.newName[:len(m.newName)-1]
				}
				return m, nil

			default:
				s := msg.String()
				if len(s) == 1 {
					m.newName += s
				}
				return m, nil
			}
		}

		// Normal mode
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}

		case "enter", " ":
			if len(m.sessions) > 0 {
				m.selected = &m.sessions[m.cursor]
				return m, tea.Quit
			}

		case "r":
			if len(m.sessions) > 0 {
				m.renaming = true
				m.newName = ""
			}
		}
	}

	return m, nil
}

// View renders the TUI
func (m model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	if len(m.sessions) == 0 {
		return helpStyle.Render("No tmux sessions found.\n")
	}

	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("tmux session manager"))
	s.WriteString("\n\n")

	// Session list
	for i, session := range m.sessions {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		attachedIndicator := "  "
		if session.Attached {
			attachedIndicator = " •"
		}

		sessionInfo := fmt.Sprintf("%s%-20s %d windows%s", cursor, session.Name, session.Windows, attachedIndicator)

		if m.cursor == i {
			s.WriteString(selectedStyle.Render(sessionInfo))
		} else if session.Attached {
			s.WriteString(dimStyle.Render(sessionInfo))
		} else {
			s.WriteString(normalStyle.Render(sessionInfo))
		}
		s.WriteString("\n")
	}

	// Rename input
	if m.renaming {
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("  Rename session to: %s█", m.newName))
		s.WriteString("\n")
	}

	// Help text
	s.WriteString("\n")
	if m.renaming {
		s.WriteString(helpStyle.Render("  enter: confirm • esc: cancel"))
	} else {
		s.WriteString(helpStyle.Render("  ↑/↓/j/k: navigate • enter: attach • r: rename • q/esc: quit"))
	}
	s.WriteString("\n")

	return s.String()
}

// renameSession renames a tmux session
func renameSession(oldName, newName string) error {
	cmd := exec.Command("tmux", "rename-session", "-t", oldName, newName)
	return cmd.Run()
}

// getSessions retrieves tmux sessions
func getSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_windows}:#{session_attached}")
	output, err := cmd.Output()
	if err != nil {
		// tmux exits with status 1 if no sessions exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("failed to run tmux: %w", err)
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return []Session{}, nil
	}

	sessions := make([]Session, 0, 8)
	for _, line := range strings.Split(lines, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}

		name := parts[0]
		windows, _ := strconv.Atoi(parts[1])
		attached := parts[2] == "1"

		sessions = append(sessions, Session{
			Name:     name,
			Windows:  windows,
			Attached: attached,
		})
	}

	return sessions, nil
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("tmgr %s\n", version)
		os.Exit(0)
	}

	sessions, err := getSessions()
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(sessions))
	m, err := p.Run()
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}

	// Check if a session was selected
	if finalModel, ok := m.(model); ok && finalModel.selected != nil {
		// Attach to the selected tmux session
		attachCmd := exec.Command("tmux", "attach-session", "-t", finalModel.selected.Name)
		attachCmd.Stdin = os.Stdin
		attachCmd.Stdout = os.Stdout
		attachCmd.Stderr = os.Stderr

		if err := attachCmd.Run(); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error attaching to session: %v", err)))
			os.Exit(1)
		}
	}
}
