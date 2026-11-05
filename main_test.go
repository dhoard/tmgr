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
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMain makes the suite environment-agnostic: a developer running the tests
// from inside a tmux pane must get the same results as one running outside.
func TestMain(m *testing.M) {
	original, hadTMUX := os.LookupEnv("TMUX")
	os.Unsetenv("TMUX")
	code := m.Run()
	if hadTMUX {
		os.Setenv("TMUX", original)
	}
	os.Exit(code)
}

// setInsideTmux controls the environment that detectInsideTmux reads.
func setInsideTmux(t *testing.T, inside bool) {
	t.Helper()
	if inside {
		t.Setenv("TMUX", "/tmp/tmux-0/tmgr-test,1234,0")
		return
	}
	t.Setenv("TMUX", "")
}

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

func TestNonEmptyStateHelpOutsideTmux(t *testing.T) {
	setInsideTmux(t, false)
	m := initialModel([]Session{{Name: "main", Windows: 1}})

	view := m.View()
	if !strings.Contains(view, "enter: attach") {
		t.Errorf("generic help expected for non-empty list, got: %q", view)
	}
	if strings.Contains(view, "enter: switch") {
		t.Errorf("help must not advertise switching outside tmux, got: %q", view)
	}
	if strings.Contains(view, "No tmux sessions found.") {
		t.Errorf("empty-state message must not render with sessions, got: %q", view)
	}
}

func TestNonEmptyStateHelpInsideTmux(t *testing.T) {
	setInsideTmux(t, true)
	m := initialModel([]Session{{Name: "main", Windows: 1}})

	view := m.View()
	if !strings.Contains(view, "enter: switch") {
		t.Errorf("help should advertise switch when inside tmux, got: %q", view)
	}
	if strings.Contains(view, "enter: attach") {
		t.Errorf("help must not advertise attach when inside tmux, got: %q", view)
	}
}

func TestEmptyStateHelpInsideTmuxDoesNotAdvertiseActivation(t *testing.T) {
	setInsideTmux(t, true)
	m := initialModel(nil)

	view := m.View()
	if !strings.Contains(view, "n: create • q/esc: quit") {
		t.Errorf("empty-state help expected, got: %q", view)
	}
	if strings.Contains(view, "enter:") {
		t.Errorf("empty-state help must not advertise activation, got: %q", view)
	}
}

// otherModel is a tea.Model that is not the app's model, used to exercise
// attachSelected's defensive type-assertion guard.
type otherModel struct{}

func (otherModel) Init() tea.Cmd                       { return nil }
func (otherModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return nil, nil }
func (otherModel) View() string                        { return "" }

func TestDetectInsideTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-0/tmgr-test,1234,0")
	if !detectInsideTmux() {
		t.Error("expected inside-tmux detection when TMUX is set")
	}

	t.Setenv("TMUX", "")
	if detectInsideTmux() {
		t.Error("expected outside-tmux detection when TMUX is empty")
	}
}

func TestSwitchClientMissingTmuxReturnsError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := switchClient("does-not-matter")
	if err == nil {
		t.Fatal("expected error when tmux is not on PATH")
	}
	if !strings.Contains(err.Error(), "tmux not found") {
		t.Errorf("expected 'tmux not found' in error, got: %v", err)
	}
}

func TestSwitchClientNoCurrentClientIntegration(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	server := fmt.Sprintf("tmgr-it-%d", time.Now().UnixNano())
	private := func(args ...string) (string, error) {
		out, err := exec.Command("tmux", append([]string{"-L", server}, args...)...).Output()
		return strings.TrimSpace(string(out)), err
	}
	if _, err := private("new-session", "-d", "-s", "keep"); err != nil {
		t.Skipf("tmux environment unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", server, "kill-server").Run()
	})

	socketPath, err := private("display-message", "-p", "-t", "keep", "#{socket_path}")
	if err != nil || socketPath == "" {
		t.Skipf("cannot determine private socket path for %q: %v", server, err)
	}
	serverPID, err := private("display-message", "-p", "-t", "keep", "#{pid}")
	if err != nil || serverPID == "" {
		t.Skipf("cannot determine private server pid for %q: %v", server, err)
	}
	sessionID, err := private("display-message", "-p", "-t", "keep", "#{session_id}")
	if err != nil || sessionID == "" {
		t.Skipf("cannot determine session id for %q: %v", server, err)
	}

	// Point tmux at the private server that has no attached client, so the real
	// switch-client command runs and its stderr is captured without a pty.
	t.Setenv("TMUX", fmt.Sprintf("%s,%s,%s", socketPath, serverPID, strings.TrimPrefix(sessionID, "$")))

	err = switchClient("keep")
	if err == nil {
		t.Fatal("expected switch to fail without an attached client")
	}
	if !strings.Contains(err.Error(), "current client") {
		t.Errorf("expected 'current client' in error, got: %v", err)
	}
}

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

// stubActivation replaces both activation actions and records which was used.
func stubActivation(t *testing.T, attachErr, switchErr error) (attached, switched *[]string) {
	t.Helper()
	oldAttach, oldSwitch := attachSessionFn, switchClientFn
	attachNames := []string{}
	switchNames := []string{}
	attachSessionFn = func(name string) error {
		attachNames = append(attachNames, name)
		return attachErr
	}
	switchClientFn = func(name string) error {
		switchNames = append(switchNames, name)
		return switchErr
	}
	t.Cleanup(func() { attachSessionFn, switchClientFn = oldAttach, oldSwitch })
	return &attachNames, &switchNames
}

func TestActivateSelectedNoSelection(t *testing.T) {
	attached, switched := stubActivation(t, nil, nil)

	m := initialModel([]Session{{Name: "main"}})
	if err := activateSelected(m); err != nil {
		t.Fatalf("expected nil for no selection, got: %v", err)
	}
	if len(*attached) != 0 || len(*switched) != 0 {
		t.Errorf("no activation expected without a selection, got attach=%v switch=%v", *attached, *switched)
	}
}

func TestActivateSelectedNonModelReturnsNil(t *testing.T) {
	setInsideTmux(t, true)
	attached, switched := stubActivation(t, nil, nil)

	if err := activateSelected(otherModel{}); err != nil {
		t.Fatalf("expected nil for non-model, got: %v", err)
	}
	if len(*attached) != 0 || len(*switched) != 0 {
		t.Errorf("no activation expected for a non-model, got attach=%v switch=%v", *attached, *switched)
	}
}

func TestActivateSelectedOutsideCallsAttach(t *testing.T) {
	setInsideTmux(t, false)
	sentinel := errors.New("boom")
	attached, switched := stubActivation(t, sentinel, nil)

	m := initialModel([]Session{{Name: "dev"}})
	m.selected = &m.sessions[0]

	if err := activateSelected(m); err != sentinel {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
	if len(*attached) != 1 || (*attached)[0] != "dev" {
		t.Errorf("expected attach for %q, got %v", "dev", *attached)
	}
	if len(*switched) != 0 {
		t.Errorf("switch must not be called outside tmux, got %v", *switched)
	}
}

func TestActivateSelectedInsideCallsSwitch(t *testing.T) {
	setInsideTmux(t, true)
	attached, switched := stubActivation(t, nil, nil)

	m := initialModel([]Session{{Name: "dev"}})
	m.selected = &m.sessions[0]

	if err := activateSelected(m); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if len(*switched) != 1 || (*switched)[0] != "dev" {
		t.Errorf("expected switch to %q, got %v", "dev", *switched)
	}
	if len(*attached) != 0 {
		t.Errorf("attach must never be called inside tmux (nested session), got %v", *attached)
	}
}

func TestActivateSelectedInsideSwitchErrorPropagates(t *testing.T) {
	setInsideTmux(t, true)
	sentinel := errors.New("no current client")
	attached, switched := stubActivation(t, nil, sentinel)

	m := initialModel([]Session{{Name: "dev"}})
	m.selected = &m.sessions[0]

	if err := activateSelected(m); err != sentinel {
		t.Fatalf("expected switch error to propagate, got: %v", err)
	}
	if len(*switched) != 1 {
		t.Errorf("expected one switch attempt, got %v", *switched)
	}
	if len(*attached) != 0 {
		t.Errorf("attach must not be attempted after a switch failure, got %v", *attached)
	}
}
