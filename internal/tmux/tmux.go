package tmux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GianlucaP106/gotmux/gotmux"
	"github.com/jh3/claude-fzf/internal/config"
)

// Manager handles tmux operations
type Manager struct {
	tmux *gotmux.Tmux
}

// New creates a tmux manager
func New() (*Manager, error) {
	t, err := gotmux.DefaultTmux()
	if err != nil {
		return nil, err
	}
	return &Manager{tmux: t}, nil
}

// IsInsideTmux checks if we're running inside tmux
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// SessionExists checks if a tmux session exists
func (m *Manager) SessionExists(name string) bool {
	return m.tmux.HasSession(name)
}

// CreateProjectSession creates a new tmux session with configured windows
// If shellCommand is provided, the first window (claude) runs that command
func (m *Manager) CreateProjectSession(name, projectPath, shellCommand string, windows []config.Window) error {
	sess, err := m.tmux.NewSession(&gotmux.SessionOptions{
		Name:           name,
		StartDirectory: projectPath,
		ShellCommand:   shellCommand,
	})
	if err != nil {
		return err
	}

	// Rename the first window to "claude"
	existingWindows, err := sess.ListWindows()
	if err == nil && len(existingWindows) > 0 {
		existingWindows[0].Rename("claude")
	}

	// Create additional windows from config
	for _, winCfg := range windows {
		_, err := sess.NewWindow(&gotmux.NewWindowOptions{
			WindowName:     winCfg.Name,
			StartDirectory: projectPath,
		})
		if err != nil {
			return err
		}

		// Run command in window if specified (silently via respawn-pane)
		// Wrap command so shell stays alive after command exits
		if winCfg.Command != "" {
			target := fmt.Sprintf("%s:%s", name, winCfg.Name)
			escaped := strings.ReplaceAll(winCfg.Command, "'", "'\\''")
			wrapped := fmt.Sprintf("sh -c '%s; exec \"$SHELL\"'", escaped)
			m.tmux.Command("respawn-pane", "-k", "-t", target, wrapped)
		}
	}

	// Select the claude window
	w, err := sess.GetWindowByName("claude")
	if err == nil {
		w.Select()
	}

	return nil
}

// SwitchToSession switches the client to a session
func (m *Manager) SwitchToSession(name string) error {
	return m.tmux.SwitchClient(&gotmux.SwitchClientOptions{
		TargetSession: name,
	})
}

// SendKeysToWindow sends keys to a specific window and executes them
func (m *Manager) SendKeysToWindow(sessionName, windowName, keys string) error {
	sess, err := m.tmux.GetSessionByName(sessionName)
	if err != nil {
		return err
	}

	w, err := sess.GetWindowByName(windowName)
	if err != nil {
		return fmt.Errorf("window not found: %s", windowName)
	}

	panes, err := w.ListPanes()
	if err != nil || len(panes) == 0 {
		return fmt.Errorf("no panes in window: %s", windowName)
	}

	pane := panes[0]
	if err := pane.SendKeys(keys); err != nil {
		return err
	}
	return pane.SendKeys("Enter")
}

// RespawnWindow kills the current process in a window and runs a new command
// This runs the command directly without visible typing
func (m *Manager) RespawnWindow(sessionName, windowName, command string) error {
	target := fmt.Sprintf("%s:%s", sessionName, windowName)
	_, err := m.tmux.Command("respawn-pane", "-k", "-t", target, command)
	return err
}

// ProjectToSessionName converts a project path to a session name
func ProjectToSessionName(projectPath string) string {
	name := filepath.Base(projectPath)
	if name == "" || name == "." {
		return "claude"
	}
	return name
}
