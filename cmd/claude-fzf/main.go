package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/jh3/claude-fzf/internal/cache"
	"github.com/jh3/claude-fzf/internal/config"
	"github.com/jh3/claude-fzf/internal/session"
	"github.com/jh3/claude-fzf/internal/tmux"
	"github.com/jh3/claude-fzf/internal/ui"
)

var cfg *config.Config

func main() {
	showAll := false
	args := os.Args[1:]

	// Parse flags
	var filtered []string
	for _, arg := range args {
		switch arg {
		case "-a", "--all":
			showAll = true
		default:
			filtered = append(filtered, arg)
		}
	}

	if len(filtered) > 0 {
		switch filtered[0] {
		case "list":
			listSessions(showAll)
		case "clear-cache":
			clearCache()
		case "-h", "--help":
			printHelp()
		default:
			printHelp()
		}
		return
	}
	runInteractive(showAll)
}

func printHelp() {
	fmt.Printf(`claude-fzf - Fuzzy search and resume Claude Code sessions

Usage: claude-fzf [flags] [command]

Commands:
  (none)        Interactive session picker
  list          Print all sessions (for scripting)
  clear-cache   Clear the session cache
  -h, --help    Show this help

Flags:
  -a, --all     Start with empty sessions visible (0 messages)

Keybindings (in picker):
  Enter         Resume selected session
  Ctrl-D        Delete selected session (with confirmation)
  Ctrl-A        Toggle showing empty sessions
  Ctrl-C/Esc    Cancel

Tmux Integration:
  When running inside tmux, selecting a session will:
  - Create a tmux session named after the project (if new)
  - Set up windows: claude (always) + configured windows
  - Resume Claude in the claude window

Configuration:
  Config file: %s

  Example config:
    tmux:
      windows:
        - name: logs
        - name: edit
        - name: tests
          command: npm test -- --watch
`, config.Path())
}

func runInteractive(showAll bool) {
	cfg = config.Load()
	sessions := loadAllSessions()

	result, err := ui.SelectSession(sessions, showAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if result.Session == nil || result.Action != ui.ActionResume {
		return // User cancelled or performed other action
	}

	resumeSession(result.Session)
}

func resumeSession(s *session.Session) {
	if tmux.IsInsideTmux() {
		resumeInTmux(s)
		return
	}
	resumeDirectly(s)
}

func resumeInTmux(s *session.Session) {
	mgr, err := tmux.New()
	if err != nil {
		resumeDirectly(s)
		return
	}

	sessionName := tmux.ProjectToSessionName(s.ProjectPath)
	claudeCmd := fmt.Sprintf("claude --resume %s", s.ID)

	if !mgr.SessionExists(sessionName) {
		// Check if we can repurpose the current session
		if disposable, _ := mgr.IsDisposableSession(); disposable {
			if err := mgr.RepurposeCurrentSession(sessionName, s.ProjectPath, cfg.Tmux.Windows); err != nil {
				fmt.Fprintf(os.Stderr, "Error repurposing session: %v\n", err)
				os.Exit(1)
			}
			// Respawn claude window and select it
			// Wrap command to keep pane alive if claude exits
			wrappedCmd := fmt.Sprintf("cd %q && %s; exec $SHELL", s.ProjectPath, claudeCmd)
			if err := mgr.RespawnWindow(sessionName, "claude", wrappedCmd); err != nil {
				fmt.Fprintf(os.Stderr, "Error respawning window: %v\n", err)
				os.Exit(1)
			}
			if err := mgr.SelectWindow(sessionName, "claude"); err != nil {
				fmt.Fprintf(os.Stderr, "Error selecting window: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Create a new session
		if err := mgr.CreateProjectSession(sessionName, s.ProjectPath, "", cfg.Tmux.Windows); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tmux session: %v\n", err)
			os.Exit(1)
		}
	}

	if err := mgr.SwitchToSession(sessionName); err != nil {
		fmt.Fprintf(os.Stderr, "Error switching to session: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.RespawnWindow(sessionName, "claude", claudeCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error respawning window: %v\n", err)
		os.Exit(1)
	}
}

func resumeDirectly(s *session.Session) {
	if s.ProjectPath != "" {
		os.Chdir(s.ProjectPath)
	}

	cmd := exec.Command("claude", "--resume", s.ID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func loadAllSessions() []session.Session {
	c := cache.New()
	scanner := session.NewScanner()

	sessions, err := scanner.ScanAllCached(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning sessions: %v\n", err)
		os.Exit(1)
	}

	c.Save()

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	return sessions
}

func loadSessions(showAll bool) []session.Session {
	sessions := loadAllSessions()

	if !showAll {
		var filtered []session.Session
		for _, s := range sessions {
			if s.UserMsgCount > 0 || s.AsstMsgCount > 0 {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}

	return sessions
}

func listSessions(showAll bool) {
	sessions := loadSessions(showAll)
	for _, s := range sessions {
		fmt.Printf("%s|%s|%s|%s\n",
			s.ID,
			s.ModTime.Format("2006-01-02 15:04"),
			s.ProjectPath,
			s.Summary)
	}
}

func clearCache() {
	c := cache.New()
	if err := c.Clear(); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Cache cleared.")
}
