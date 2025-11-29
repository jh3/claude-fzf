package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/jh3/claude-fzf/internal/cache"
	"github.com/jh3/claude-fzf/internal/session"
	"github.com/jh3/claude-fzf/internal/tmux"
	"github.com/jh3/claude-fzf/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "list":
			listSessions()
		case "clear-cache":
			clearCache()
		case "-h", "--help":
			printHelp()
		default:
			printHelp()
		}
		return
	}
	runInteractive()
}

func printHelp() {
	fmt.Print(`claude-fzf - Fuzzy search and resume Claude Code sessions

Usage: claude-fzf [command]

Commands:
  (none)        Interactive session picker
  list          Print all sessions (for scripting)
  clear-cache   Clear the session cache
  -h, --help    Show this help

Keybindings (in picker):
  Enter         Resume selected session
  Ctrl-C/Esc    Cancel

Tmux Integration:
  When running inside tmux, selecting a session will:
  - Create a tmux session named after the project (if new)
  - Set up 4 windows: claude, logs, edit, scratch
  - Resume Claude in the claude window
`)
}

func runInteractive() {
	sessions := loadSessions()

	selected, err := ui.SelectSession(sessions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if selected == nil {
		return // User cancelled
	}

	resumeSession(selected)
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
		mgr.CreateProjectSession(sessionName, s.ProjectPath, "")
	}

	mgr.SwitchToSession(sessionName)
	mgr.SendKeysToWindow(sessionName, "claude", claudeCmd)
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

func loadSessions() []session.Session {
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

func listSessions() {
	sessions := loadSessions()
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
