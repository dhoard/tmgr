//
// Copyright (c) 2026-present Douglas Hoard
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// version is set via ldflags at build time
var version = "dev"

// Session represents a tmux session
type Session struct {
	Name     string
	Windows  int
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
	sessions       []Session
	cursor         int
	selected       *Session
	quitting       bool
	renaming       bool
	newName        string
	creating       bool
	newSessionName string
	createErr      string
	err            error
}

// Test seams for the tmux-backed functions used in the create flow
var (
	createSessionFn = createSession
	getSessionsFn   = getSessions
)

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

		// Handle create mode
		if m.creating {
			switch msg.String() {
			case "esc":
				m.creating = false
				m.newSessionName = ""
				return m, nil

			case "enter":
				if m.newSessionName == "" {
					return m, nil
				}
				name := m.newSessionName
				m.creating = false
				m.newSessionName = ""
				if err := createSessionFn(name); err != nil {
					m.createErr = err.Error()
					return m, nil
				}
				sessions, err := getSessionsFn()
				if err != nil {
					m.createErr = err.Error()
					return m, nil
				}
				m.createErr = ""
				m.sessions = sessions
				for i, session := range sessions {
					if session.Name == name {
						m.cursor = i
						break
					}
				}
				return m, nil

			case "backspace":
				if len(m.newSessionName) > 0 {
					m.newSessionName = m.newSessionName[:len(m.newSessionName)-1]
				}
				return m, nil

			default:
				s := msg.String()
				if len(s) == 1 {
					m.newSessionName += s
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

		case "n":
			m.creating = true
			m.newSessionName = ""
			m.createErr = ""
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

	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("tmux session manager"))
	s.WriteString("\n\n")

	if len(m.sessions) == 0 {
		s.WriteString(helpStyle.Render("No tmux sessions found."))
		s.WriteString("\n")
	} else {
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
	}

	// Create input
	if m.creating {
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("  New session name: %s█", m.newSessionName))
		s.WriteString("\n")
	} else if m.createErr != "" {
		s.WriteString("\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s", m.createErr)))
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
	} else if m.creating {
		s.WriteString(helpStyle.Render("  enter: create • esc: cancel"))
	} else if len(m.sessions) == 0 {
		s.WriteString(helpStyle.Render("  n: create • q/esc: quit"))
	} else {
		s.WriteString(helpStyle.Render("  ↑/↓/j/k: navigate • enter: attach • r: rename • n: new • q/esc: quit"))
	}
	s.WriteString("\n")

	return s.String()
}

// createSession creates a detached tmux session
func createSession(name string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(output)); msg != "" {
			return fmt.Errorf("%s: %w", msg, err)
		}
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
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

// attachSessionFn is a test seam for the attach step (defaults to the real
// exec-based attachSession).
var attachSessionFn = attachSession

// attachSession replaces the current process with tmux attach-session for
// the named session. It returns only if the exec fails.
func attachSession(name string) error {
	binary, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}

	if err := syscall.Exec(binary, []string{"tmux", "attach-session", "-t", name}, os.Environ()); err != nil {
		return fmt.Errorf("attach to session %q: %w", name, err)
	}
	return nil // unreachable: syscall.Exec replaces the process on success
}

// attachSelected attaches to the selected session, if any. It returns nil
// when no session was selected or when m is not the app's model.
func attachSelected(m tea.Model) error {
	finalModel, ok := m.(model)
	if !ok || finalModel.selected == nil {
		return nil
	}
	return attachSessionFn(finalModel.selected.Name)
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

	// Attach to the selected session, if any. On success this process is
	// replaced by tmux, so this returns only on error.
	if err := attachSelected(m); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}
}
