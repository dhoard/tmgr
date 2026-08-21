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
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCompileSmoke(t *testing.T) {
	_ = model{}
	_ = Session{}
}

// pressKey sends a key message to the model and returns the updated model
func pressKey(m model, key string) tea.Model {
	var (
		updated tea.Model
		_       tea.Cmd
	)
	switch key {
	case "enter":
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	case "esc":
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	case "backspace":
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	default:
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
	return updated
}

// stubCreateFlow replaces the tmux-backed functions for the duration of a test
func stubCreateFlow(t *testing.T, createErr error, refreshed []Session, refreshErr error) {
	t.Helper()
	oldCreate, oldGet := createSessionFn, getSessionsFn
	createSessionFn = func(string) error { return createErr }
	getSessionsFn = func() ([]Session, error) { return refreshed, refreshErr }
	t.Cleanup(func() { createSessionFn, getSessionsFn = oldCreate, oldGet })
}

func TestCreateOpensInputMode(t *testing.T) {
	m := initialModel([]Session{{Name: "main", Windows: 1}})

	m2, ok := pressKey(m, "n").(model)
	if !ok {
		t.Fatal("expected model")
	}
	if !m2.creating {
		t.Error("expected creating mode after pressing n")
	}
	if got := m2.View(); !strings.Contains(got, "New session name:") {
		t.Errorf("view should contain input line, got: %q", got)
	}
	if got := m2.View(); !strings.Contains(got, "enter: create • esc: cancel") {
		t.Errorf("view should contain create help, got: %q", got)
	}
}

func TestCreateTypingAndBackspace(t *testing.T) {
	m := initialModel(nil)
	m.creating = true

	for _, k := range []string{"w", "o", "r", "k"} {
		m, _ = pressKey(m, k).(model)
	}
	if m.newSessionName != "work" {
		t.Errorf("expected name %q, got %q", "work", m.newSessionName)
	}

	m, _ = pressKey(m, "backspace").(model)
	if m.newSessionName != "wor" {
		t.Errorf("expected name %q after backspace, got %q", "wor", m.newSessionName)
	}
}

func TestCreateCancel(t *testing.T) {
	m := initialModel([]Session{{Name: "main"}})
	m, _ = pressKey(m, "n").(model)
	m, _ = pressKey(m, "x").(model)
	m2, ok := pressKey(m, "esc").(model)
	if !ok {
		t.Fatal("expected model")
	}
	if m2.creating || m2.newSessionName != "" {
		t.Errorf("expected create mode cleared, got creating=%v name=%q", m2.creating, m2.newSessionName)
	}
	if m2.quitting {
		t.Error("esc in create mode must cancel, not quit")
	}
}

func TestCreateEmptyNameIsNoop(t *testing.T) {
	called := false
	oldCreate := createSessionFn
	createSessionFn = func(string) error { called = true; return nil }
	t.Cleanup(func() { createSessionFn = oldCreate })

	m := initialModel(nil)
	m.creating = true
	m2, _ := pressKey(m, "enter").(model)

	if !m2.creating {
		t.Error("empty name + enter should stay in create mode")
	}
	if called {
		t.Error("empty name + enter must not invoke tmux")
	}
}

func TestCreateSuccessRefreshesAndSelects(t *testing.T) {
	stubCreateFlow(t, nil, []Session{{Name: "aaa"}, {Name: "work"}}, nil)

	m := initialModel([]Session{{Name: "aaa"}})
	m, _ = pressKey(m, "n").(model)
	for _, k := range []string{"w", "o", "r", "k"} {
		m, _ = pressKey(m, k).(model)
	}
	m2, ok := pressKey(m, "enter").(model)
	if !ok {
		t.Fatal("expected model")
	}

	if m2.creating || m2.newSessionName != "" {
		t.Error("expected create mode cleared after enter")
	}
	if m2.createErr != "" {
		t.Errorf("unexpected error: %q", m2.createErr)
	}
	if len(m2.sessions) != 2 || m2.sessions[1].Name != "work" {
		t.Errorf("expected refreshed list, got %+v", m2.sessions)
	}
	if m2.cursor != 1 {
		t.Errorf("expected cursor on new session (1), got %d", m2.cursor)
	}
}

func TestCreateFailureShowsInlineError(t *testing.T) {
	stubCreateFlow(t, errors.New("duplicate session: work"), nil, nil)

	m := initialModel([]Session{{Name: "main"}})
	m, _ = pressKey(m, "n").(model)
	for _, k := range []string{"w", "o", "r", "k"} {
		m, _ = pressKey(m, k).(model)
	}
	m2, _ := pressKey(m, "enter").(model)

	if m2.creating {
		t.Error("expected return to list mode on failure")
	}
	if m2.createErr == "" {
		t.Error("expected createErr to be set")
	}
	if got := m2.View(); !strings.Contains(got, "Error: duplicate session: work") {
		t.Errorf("view should show inline error, got: %q", got)
	}
	if len(m2.sessions) != 1 || m2.sessions[0].Name != "main" {
		t.Errorf("list must be unchanged on failure, got %+v", m2.sessions)
	}

	// Error clears when reopening create mode
	m3, _ := pressKey(m2, "n").(model)
	if m3.createErr != "" {
		t.Error("expected createErr cleared on n")
	}
}

func TestCreateSessionIntegration(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	name := fmt.Sprintf("tmgr-test-%d", time.Now().UnixNano())
	if err := createSession(name); err != nil {
		t.Skipf("tmux environment unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "kill-session", "-t", name).Run()
	})

	sessions, err := getSessions()
	if err != nil {
		t.Fatalf("getSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.Name == name {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created session %q not found in %v", name, sessions)
	}

	// Duplicate creation must fail so the UI error path is reachable
	if err := createSession(name); err == nil {
		t.Error("expected duplicate session creation to fail")
	}
}

func TestEmptyStateShowsDedicatedHelp(t *testing.T) {
	m := initialModel(nil)

	view := m.View()
	if !strings.Contains(view, "No tmux sessions found.") {
		t.Errorf("view should show the empty-state message, got: %q", view)
	}
	if !strings.Contains(view, "n: create • q/esc: quit") {
		t.Errorf("view should show empty-state help, got: %q", view)
	}
	if strings.Contains(view, "enter: attach") {
		t.Errorf("empty-state help must not advertise attach, got: %q", view)
	}
}

func TestEmptyStateHelpReplacedByCreateMode(t *testing.T) {
	m := initialModel(nil)
	m, _ = pressKey(m, "n").(model)

	view := m.View()
	if !strings.Contains(view, "enter: create • esc: cancel") {
		t.Errorf("create-mode help expected, got: %q", view)
	}
	if strings.Contains(view, "n: create • q/esc: quit") {
		t.Errorf("empty-state help must not render during create mode, got: %q", view)
	}
}

func TestNonEmptyStateKeepsGenericHelp(t *testing.T) {
	m := initialModel([]Session{{Name: "main", Windows: 1}})

	view := m.View()
	if !strings.Contains(view, "enter: attach") {
		t.Errorf("generic help expected for non-empty list, got: %q", view)
	}
	if strings.Contains(view, "No tmux sessions found.") {
		t.Errorf("empty-state message must not render with sessions, got: %q", view)
	}
}

// otherModel is a tea.Model that is not the app's model, used to exercise
// attachSelected's defensive type-assertion guard.
type otherModel struct{}

func (otherModel) Init() tea.Cmd                       { return nil }
func (otherModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return nil, nil }
func (otherModel) View() string                        { return "" }

func TestAttachSessionMissingTmuxReturnsError(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // ensure tmux is not resolvable
	err := attachSession("does-not-matter")
	if err == nil {
		t.Fatal("expected error when tmux is not on PATH")
	}
	if !strings.Contains(err.Error(), "tmux not found") {
		t.Errorf("expected 'tmux not found' in error, got: %v", err)
	}
}

func TestAttachSelectedNoSelection(t *testing.T) {
	old := attachSessionFn
	called := false
	attachSessionFn = func(string) error { called = true; return nil }
	t.Cleanup(func() { attachSessionFn = old })

	m := initialModel([]Session{{Name: "main"}})
	if err := attachSelected(m); err != nil {
		t.Fatalf("expected nil for no selection, got: %v", err)
	}
	if called {
		t.Error("attach must not be called when nothing is selected")
	}
}

func TestAttachSelectedCallsAttachWithName(t *testing.T) {
	old := attachSessionFn
	gotName := ""
	sentinel := errors.New("boom")
	attachSessionFn = func(name string) error { gotName = name; return sentinel }
	t.Cleanup(func() { attachSessionFn = old })

	m := initialModel([]Session{{Name: "dev"}})
	m.selected = &m.sessions[0]

	if err := attachSelected(m); err != sentinel {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
	if gotName != "dev" {
		t.Errorf("expected attach for %q, got %q", "dev", gotName)
	}
}

func TestAttachSelectedNonModelReturnsNil(t *testing.T) {
	old := attachSessionFn
	attachSessionFn = func(string) error {
		t.Error("attach must not be called for a non-model value")
		return nil
	}
	t.Cleanup(func() { attachSessionFn = old })

	if err := attachSelected(otherModel{}); err != nil {
		t.Fatalf("expected nil for non-model, got: %v", err)
	}
}
