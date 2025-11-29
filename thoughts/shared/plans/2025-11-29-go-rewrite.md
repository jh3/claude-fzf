# claude-fzf Go Rewrite Implementation Plan

## Overview

Rewrite claude-fzf from shell scripts to a single Go binary for significantly faster startup and execution. The new binary will have zero external dependencies (except tmux for tmux integration) by embedding fuzzy finding via go-fzf and using gotmux for tmux operations.

## Current State Analysis

The existing implementation consists of 3 shell scripts:
- `claude-fzf` - Main script (277 lines)
- `claude-fzf-tmux` - Tmux helper functions (72 lines)
- `claude-fzf-preview` - Preview generator (103 lines)

### Performance Bottlenecks
1. **Subprocess spawning** - Each session file requires 5-7 subprocess calls (grep, jq, stat)
2. **Sequential processing** - Files processed one at a time
3. **Cache lookup via grep** - O(n) grep per file lookup
4. **Shell startup overhead** - Bash initialization adds latency

### Key Data Structures

Session files are JSONL in `~/.claude/projects/<encoded-path>/<session-id>.jsonl`:
```json
{"type":"user","message":{"content":"..."},"cwd":"/path/to/project",...}
{"type":"assistant","message":{"content":"..."},...}
{"type":"summary","summary":"..."}
```

Relevant fields to extract:
- `type` - "user", "assistant", "summary", "file-history-snapshot"
- `cwd` - Project directory (from first message with cwd)
- `summary` - Session summary (if present)
- `message.content` - Message text (for fallback summary)
- `gitBranch` - Git branch name (optional)

## Desired End State

A single `claude-fzf` binary (~5-10MB) that:
1. Starts in <50ms (vs current ~500ms+)
2. Lists 100+ sessions in <100ms (vs current 3-5s)
3. Has zero external dependencies for core functionality
4. Maintains all current features (fuzzy search, preview, tmux integration)

### Verification
```bash
# Build and test
go build -o claude-fzf ./cmd/claude-fzf
time ./claude-fzf list  # Should complete in <100ms

# Test tmux integration (inside tmux)
./claude-fzf  # Select session, verify tmux session created
```

## What We're NOT Doing

- Web interface or GUI
- Session creation/deletion (read-only)
- Remote session management
- Plugin system
- Configuration file (keep it simple)

## Implementation Approach

Single Go module with clean package separation:
- `cmd/claude-fzf/` - Main entrypoint with subcommands
- `internal/session/` - Session discovery and parsing
- `internal/cache/` - Mtime-based caching
- `internal/ui/` - go-fzf integration and preview
- `internal/tmux/` - gotmux wrapper

---

## Phase 1: Project Setup and Session Discovery

### Overview
Set up Go module structure and implement core session discovery with parallel file processing.

### Changes Required

#### 1. Initialize Go module
**File**: `go.mod`
```go
module github.com/jh3/claude-fzf

go 1.21

require (
    github.com/koki-develop/go-fzf v0.16.0
    github.com/GianlucaP106/gotmux v0.2.0
)
```

#### 2. Session types
**File**: `internal/session/types.go`
```go
package session

import "time"

// Session represents a Claude Code session
type Session struct {
    ID          string    // UUID from filename
    ProjectPath string    // Decoded from cwd field
    Summary     string    // From summary type or first user message
    ModTime     time.Time // File modification time
    FilePath    string    // Full path to JSONL file
    GitBranch   string    // Optional git branch
    UserMsgCount int      // Number of user messages
    AsstMsgCount int      // Number of assistant messages
}
```

#### 3. Session scanner with parallel processing
**File**: `internal/session/scanner.go`
```go
package session

import (
    "bufio"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "sync"
)

const claudeProjectsDir = ".claude/projects"

// Scanner finds and parses Claude sessions
type Scanner struct {
    baseDir string
}

// NewScanner creates a scanner for the default Claude projects directory
func NewScanner() *Scanner {
    home, _ := os.UserHomeDir()
    return &Scanner{baseDir: filepath.Join(home, claudeProjectsDir)}
}

// ScanAll finds all sessions with parallel processing
func (s *Scanner) ScanAll() ([]Session, error) {
    var files []string
    err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return nil // Skip errors
        }
        if !d.IsDir() && strings.HasSuffix(path, ".jsonl") && !strings.HasPrefix(d.Name(), "agent-") {
            files = append(files, path)
        }
        return nil
    })
    if err != nil {
        return nil, err
    }

    // Process files in parallel
    var wg sync.WaitGroup
    results := make(chan Session, len(files))

    for _, f := range files {
        wg.Add(1)
        go func(path string) {
            defer wg.Done()
            if sess, err := parseSessionFile(path); err == nil {
                results <- sess
            }
        }(f)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    var sessions []Session
    for sess := range results {
        sessions = append(sessions, sess)
    }
    return sessions, nil
}
```

#### 4. JSONL parser
**File**: `internal/session/parser.go`
```go
package session

import (
    "bufio"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
)

// jsonLine represents a single line in the JSONL file
type jsonLine struct {
    Type    string `json:"type"`
    Cwd     string `json:"cwd"`
    Summary string `json:"summary"`
    Message struct {
        Content string `json:"content"`
    } `json:"message"`
    GitBranch string `json:"gitBranch"`
}

func parseSessionFile(path string) (Session, error) {
    f, err := os.Open(path)
    if err != nil {
        return Session{}, err
    }
    defer f.Close()

    info, _ := f.Stat()

    sess := Session{
        ID:       strings.TrimSuffix(filepath.Base(path), ".jsonl"),
        FilePath: path,
        ModTime:  info.ModTime(),
    }

    scanner := bufio.NewScanner(f)
    // Increase buffer for long lines
    scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

    var firstUserMsg string

    for scanner.Scan() {
        var line jsonLine
        if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
            continue
        }

        switch line.Type {
        case "user":
            sess.UserMsgCount++
            if sess.ProjectPath == "" && line.Cwd != "" {
                sess.ProjectPath = line.Cwd
            }
            if firstUserMsg == "" && line.Message.Content != "" {
                firstUserMsg = line.Message.Content
            }
        case "assistant":
            sess.AsstMsgCount++
        case "summary":
            if line.Summary != "" {
                sess.Summary = line.Summary
            }
        }

        if line.GitBranch != "" && sess.GitBranch == "" {
            sess.GitBranch = line.GitBranch
        }
    }

    // Use first user message if no summary
    if sess.Summary == "" && firstUserMsg != "" {
        sess.Summary = truncate(firstUserMsg, 60)
    }
    if sess.Summary == "" {
        sess.Summary = "(no summary)"
    }

    return sess, nil
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
```

#### 5. Main entrypoint with list command
**File**: `cmd/claude-fzf/main.go`
```go
package main

import (
    "fmt"
    "os"
    "sort"

    "github.com/jh3/claude-fzf/internal/session"
)

func main() {
    if len(os.Args) > 1 && os.Args[1] == "list" {
        listSessions()
        return
    }

    // Default: run interactive UI (Phase 2)
    fmt.Println("Interactive mode not yet implemented. Use 'list' subcommand.")
}

func listSessions() {
    scanner := session.NewScanner()
    sessions, err := scanner.ScanAll()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error scanning sessions: %v\n", err)
        os.Exit(1)
    }

    // Sort by modification time (newest first)
    sort.Slice(sessions, func(i, j int) bool {
        return sessions[i].ModTime.After(sessions[j].ModTime)
    })

    for _, s := range sessions {
        fmt.Printf("%s|%s|%s|%s\n",
            s.ID,
            s.ModTime.Format("2006-01-02 15:04"),
            s.ProjectPath,
            s.Summary)
    }
}
```

### Success Criteria

#### Automated Verification:
- [x] `go build ./cmd/claude-fzf` compiles without errors
- [x] `go test ./...` passes
- [x] `./claude-fzf list` outputs session data
- [x] `time ./claude-fzf list` completes in <200ms

#### Manual Verification:
- [x] Output matches expected format: `id|timestamp|path|summary`
- [x] All sessions from `~/.claude/projects/` are found
- [x] Agent sessions (agent-*) are excluded

---

## Phase 2: Caching Layer

### Overview
Add mtime-based caching to avoid re-parsing unchanged files on subsequent runs.

### Changes Required

#### 1. Cache types and storage
**File**: `internal/cache/cache.go`
```go
package cache

import (
    "encoding/gob"
    "os"
    "path/filepath"
    "time"

    "github.com/jh3/claude-fzf/internal/session"
)

const cacheFileName = "sessions.cache"

// Entry stores cached session data with its file mtime
type Entry struct {
    ModTime time.Time
    Session session.Session
}

// Cache manages session metadata caching
type Cache struct {
    path    string
    entries map[string]Entry // keyed by file path
}

// New creates or loads a cache
func New() (*Cache, error) {
    home, _ := os.UserHomeDir()
    cacheDir := filepath.Join(home, ".cache", "claude-fzf")
    os.MkdirAll(cacheDir, 0755)

    c := &Cache{
        path:    filepath.Join(cacheDir, cacheFileName),
        entries: make(map[string]Entry),
    }
    c.load()
    return c, nil
}

func (c *Cache) load() {
    f, err := os.Open(c.path)
    if err != nil {
        return
    }
    defer f.Close()
    gob.NewDecoder(f).Decode(&c.entries)
}

// Save persists the cache to disk
func (c *Cache) Save() error {
    f, err := os.Create(c.path)
    if err != nil {
        return err
    }
    defer f.Close()
    return gob.NewEncoder(f).Encode(c.entries)
}

// Get retrieves a cached session if mtime matches
func (c *Cache) Get(path string, mtime time.Time) (session.Session, bool) {
    entry, ok := c.entries[path]
    if !ok || !entry.ModTime.Equal(mtime) {
        return session.Session{}, false
    }
    return entry.Session, true
}

// Set stores a session in the cache
func (c *Cache) Set(path string, mtime time.Time, sess session.Session) {
    c.entries[path] = Entry{ModTime: mtime, Session: sess}
}

// Prune removes entries for files that no longer exist
func (c *Cache) Prune(validPaths map[string]bool) {
    for path := range c.entries {
        if !validPaths[path] {
            delete(c.entries, path)
        }
    }
}
```

#### 2. Update scanner to use cache
**File**: `internal/session/scanner.go` (updated)
```go
// ScanAllCached finds sessions using cache for unchanged files
func (s *Scanner) ScanAllCached(cache *cache.Cache) ([]Session, error) {
    var files []fileInfo
    err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return nil
        }
        if !d.IsDir() && strings.HasSuffix(path, ".jsonl") && !strings.HasPrefix(d.Name(), "agent-") {
            info, _ := d.Info()
            files = append(files, fileInfo{path: path, modTime: info.ModTime()})
        }
        return nil
    })
    if err != nil {
        return nil, err
    }

    // Track valid paths for cache pruning
    validPaths := make(map[string]bool)

    var wg sync.WaitGroup
    results := make(chan Session, len(files))

    for _, f := range files {
        validPaths[f.path] = true

        // Check cache first
        if cached, ok := cache.Get(f.path, f.modTime); ok {
            results <- cached
            continue
        }

        // Parse in parallel
        wg.Add(1)
        go func(fi fileInfo) {
            defer wg.Done()
            if sess, err := parseSessionFile(fi.path); err == nil {
                cache.Set(fi.path, fi.modTime, sess)
                results <- sess
            }
        }(f)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    var sessions []Session
    for sess := range results {
        sessions = append(sessions, sess)
    }

    cache.Prune(validPaths)
    return sessions, nil
}

type fileInfo struct {
    path    string
    modTime time.Time
}
```

### Success Criteria

#### Automated Verification:
- [x] `go build ./cmd/claude-fzf` compiles
- [x] `go test ./internal/cache/...` passes
- [x] First run creates `~/.cache/claude-fzf/sessions.cache`
- [x] Second run is faster than first (use `time` command)

#### Manual Verification:
- [x] Cache file is created after first run
- [x] Modifying a session file causes re-parse of that file only
- [x] Deleting a session file removes it from cache

---

## Phase 3: Interactive UI with go-fzf

### Overview
Implement the interactive fuzzy finder using go-fzf with preview support.

### Changes Required

#### 1. UI package
**File**: `internal/ui/fzf.go`
```go
package ui

import (
    "fmt"
    "strings"

    "github.com/koki-develop/go-fzf"
    "github.com/jh3/claude-fzf/internal/session"
)

// SelectSession presents an interactive fuzzy finder
func SelectSession(sessions []session.Session) (*session.Session, error) {
    if len(sessions) == 0 {
        return nil, fmt.Errorf("no sessions found")
    }

    f, err := fzf.New(
        fzf.WithPrompt("Claude Sessions > "),
        fzf.WithInputPosition(fzf.InputPositionTop),
        fzf.WithLimit(1),
    )
    if err != nil {
        return nil, err
    }

    idxs, err := f.Find(
        sessions,
        func(i int) string {
            s := sessions[i]
            return fmt.Sprintf("%s  %s  %s",
                s.ModTime.Format("2006-01-02 15:04"),
                s.ProjectPath,
                s.Summary)
        },
        fzf.WithPreviewWindow(func(i, w, h int) string {
            if i < 0 || i >= len(sessions) {
                return ""
            }
            return formatPreview(sessions[i], h)
        }),
    )
    if err != nil {
        return nil, err
    }
    if len(idxs) == 0 {
        return nil, nil // User cancelled
    }

    return &sessions[idxs[0]], nil
}

func formatPreview(s session.Session, height int) string {
    var b strings.Builder

    b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
    b.WriteString(fmt.Sprintf("Session: %s\n", s.ID))
    b.WriteString(fmt.Sprintf("Project: %s\n", s.ProjectPath))
    b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

    if s.Summary != "" && s.Summary != "(no summary)" {
        b.WriteString(fmt.Sprintf("Summary:\n%s\n\n", s.Summary))
    }

    if s.GitBranch != "" {
        b.WriteString(fmt.Sprintf("Branch: %s\n", s.GitBranch))
    }

    b.WriteString(fmt.Sprintf("Messages: %d user / %d assistant\n", s.UserMsgCount, s.AsstMsgCount))
    b.WriteString(fmt.Sprintf("Last modified: %s\n", s.ModTime.Format("2006-01-02 15:04:05")))

    return b.String()
}
```

#### 2. Update main to use interactive UI
**File**: `cmd/claude-fzf/main.go` (updated)
```go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "sort"

    "github.com/jh3/claude-fzf/internal/cache"
    "github.com/jh3/claude-fzf/internal/session"
    "github.com/jh3/claude-fzf/internal/ui"
)

func main() {
    if len(os.Args) > 1 {
        switch os.Args[1] {
        case "list":
            listSessions()
            return
        case "preview":
            if len(os.Args) > 2 {
                showPreview(os.Args[2])
            }
            return
        case "clear-cache":
            clearCache()
            return
        case "-h", "--help":
            printHelp()
            return
        }
    }

    runInteractive()
}

func runInteractive() {
    c, _ := cache.New()
    scanner := session.NewScanner()

    sessions, err := scanner.ScanAllCached(c)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    c.Save()

    // Sort by modification time
    sort.Slice(sessions, func(i, j int) bool {
        return sessions[i].ModTime.After(sessions[j].ModTime)
    })

    selected, err := ui.SelectSession(sessions)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    if selected == nil {
        return // User cancelled
    }

    // Resume the session
    resumeSession(selected)
}

func resumeSession(s *session.Session) {
    // Change to project directory if it exists
    if s.ProjectPath != "" {
        os.Chdir(s.ProjectPath)
    }

    // Exec claude --resume
    cmd := exec.Command("claude", "--resume", s.ID)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}
```

### Success Criteria

#### Automated Verification:
- [x] `go build ./cmd/claude-fzf` compiles
- [x] Binary runs without errors

#### Manual Verification:
- [x] Running `./claude-fzf` shows interactive fuzzy finder
- [x] Typing filters sessions correctly
- [x] Preview pane shows session details
- [x] Pressing Enter resumes the selected session
- [x] Pressing Ctrl-C/Esc exits without action

---

## Phase 4: Tmux Integration with gotmux

### Overview
Add tmux session management using the gotmux library.

### Changes Required

#### 1. Tmux package
**File**: `internal/tmux/tmux.go`
```go
package tmux

import (
    "os"
    "path/filepath"

    "github.com/GianlucaP106/gotmux/gotmux"
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
    sessions, err := m.tmux.ListSessions()
    if err != nil {
        return false
    }
    for _, s := range sessions {
        if s.Name == name {
            return true
        }
    }
    return false
}

// CreateProjectSession creates a new tmux session with 4 windows
func (m *Manager) CreateProjectSession(name, projectPath string) error {
    // Create session with first window named "claude"
    sess, err := m.tmux.NewSession(&gotmux.SessionOptions{
        Name:           name,
        StartDirectory: projectPath,
        WindowName:     "claude",
    })
    if err != nil {
        return err
    }

    // Create additional windows
    for _, winName := range []string{"logs", "edit", "scratch"} {
        _, err := sess.NewWindow(&gotmux.WindowOptions{
            Name:           winName,
            StartDirectory: projectPath,
        })
        if err != nil {
            return err
        }
    }

    // Select the claude window
    windows, _ := sess.ListWindows()
    for _, w := range windows {
        if w.Name == "claude" {
            w.Select()
            break
        }
    }

    return nil
}

// SwitchToSession switches the client to a session
func (m *Manager) SwitchToSession(name string) error {
    sessions, err := m.tmux.ListSessions()
    if err != nil {
        return err
    }
    for _, s := range sessions {
        if s.Name == name {
            return s.SwitchClient()
        }
    }
    return nil
}

// SendKeysToWindow sends keys to a specific window
func (m *Manager) SendKeysToWindow(sessionName, windowName, keys string) error {
    sessions, err := m.tmux.ListSessions()
    if err != nil {
        return err
    }

    for _, s := range sessions {
        if s.Name == sessionName {
            windows, err := s.ListWindows()
            if err != nil {
                return err
            }
            for _, w := range windows {
                if w.Name == windowName {
                    panes, _ := w.ListPanes()
                    if len(panes) > 0 {
                        return panes[0].SendKeys(keys)
                    }
                }
            }
        }
    }
    return nil
}

// ProjectToSessionName converts a project path to a session name
func ProjectToSessionName(projectPath string) string {
    return filepath.Base(projectPath)
}
```

#### 2. Update main for tmux-aware resume
**File**: `cmd/claude-fzf/main.go` (updated resumeSession)
```go
func resumeSession(s *session.Session) {
    if tmux.IsInsideTmux() {
        resumeInTmux(s)
    } else {
        resumeDirectly(s)
    }
}

func resumeInTmux(s *session.Session) {
    mgr, err := tmux.New()
    if err != nil {
        // Fallback to direct resume
        resumeDirectly(s)
        return
    }

    sessionName := tmux.ProjectToSessionName(s.ProjectPath)

    if mgr.SessionExists(sessionName) {
        // Switch to existing session
        mgr.SwitchToSession(sessionName)
        mgr.SendKeysToWindow(sessionName, "claude", fmt.Sprintf("claude --resume %s", s.ID))
    } else {
        // Create new session with 4-window layout
        mgr.CreateProjectSession(sessionName, s.ProjectPath)
        mgr.SwitchToSession(sessionName)
        mgr.SendKeysToWindow(sessionName, "claude", fmt.Sprintf("claude --resume %s", s.ID))
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
```

### Success Criteria

#### Automated Verification:
- [x] `go build ./cmd/claude-fzf` compiles
- [x] `go test ./internal/tmux/...` passes

#### Manual Verification:
- [x] Outside tmux: session resumes directly in terminal
- [x] Inside tmux: new project creates tmux session with 4 windows
- [x] Inside tmux: existing project switches to existing session
- [x] Windows are named: claude, logs, edit, scratch
- [x] Claude command is sent to claude window

---

## Phase 5: Polish and Documentation

### Overview
Add help text, clean up error handling, and update documentation.

### Changes Required

#### 1. Help command
**File**: `cmd/claude-fzf/help.go`
```go
package main

import "fmt"

func printHelp() {
    fmt.Print(`claude-fzf - Fuzzy search and resume Claude Code sessions

Usage: claude-fzf [command]

Commands:
  (none)        Interactive session picker
  list          Print all sessions (for scripting)
  preview ID    Show preview for a session
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

Environment:
  CLAUDE_FZF_DEBUG=1    Enable debug output
`)
}
```

#### 2. Update README.md
Replace shell script references with Go binary usage:
```markdown
## Installation

### From Source (requires Go 1.21+)
```bash
git clone https://github.com/jh3/claude-fzf
cd claude-fzf
go build -o claude-fzf ./cmd/claude-fzf

# Move to PATH
mv claude-fzf /usr/local/bin/
```

### Tmux Keybinding
Add to `~/.tmux.conf`:
```bash
bind-key g new-window "claude-fzf"
```

## Usage
```bash
claude-fzf              # Interactive picker
claude-fzf list         # List all sessions
claude-fzf clear-cache  # Clear cache
```
```

#### 3. Makefile for convenience
**File**: `Makefile`
```makefile
.PHONY: build test clean install

build:
	go build -o claude-fzf ./cmd/claude-fzf

test:
	go test ./...

clean:
	rm -f claude-fzf
	rm -rf ~/.cache/claude-fzf

install: build
	mv claude-fzf /usr/local/bin/
```

### Success Criteria

#### Automated Verification:
- [x] `make build` succeeds
- [x] `make test` passes
- [x] `./claude-fzf --help` shows help text

#### Manual Verification:
- [x] README accurately describes installation and usage
- [x] All features work as documented

---

## File Structure (After Implementation)

```
claude-fzf/
├── cmd/
│   └── claude-fzf/
│       ├── main.go         # Entrypoint and subcommands
│       └── help.go         # Help text
├── internal/
│   ├── session/
│   │   ├── types.go        # Session struct
│   │   ├── scanner.go      # File discovery
│   │   └── parser.go       # JSONL parsing
│   ├── cache/
│   │   └── cache.go        # Mtime-based caching
│   ├── ui/
│   │   └── fzf.go          # go-fzf integration
│   └── tmux/
│       └── tmux.go         # gotmux wrapper
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE
```

---

## Testing Strategy

### Unit Tests
- `internal/session/parser_test.go` - Test JSONL parsing with sample data
- `internal/cache/cache_test.go` - Test cache read/write/prune

### Integration Tests
- Test full flow with real `~/.claude/projects/` data
- Benchmark: `go test -bench=. ./...`

### Manual Testing Steps
1. Build: `make build`
2. List sessions: `./claude-fzf list`
3. Interactive mode: `./claude-fzf`
4. Test tmux (inside tmux): Select session, verify 4-window layout
5. Test outside tmux: Verify direct resume works
6. Test cache: Run twice, verify second run is faster

---

## Performance Targets

| Metric | Shell Script | Go Target |
|--------|-------------|-----------|
| Startup | ~100ms | <10ms |
| List 50 sessions (cold) | 3-5s | <200ms |
| List 50 sessions (cached) | 1-2s | <50ms |
| Memory usage | N/A | <20MB |

---

## Migration Notes

The Go binary replaces these shell scripts:
- `claude-fzf` (main script)
- `claude-fzf-tmux` (tmux helpers)
- `claude-fzf-preview` (preview generator)

Shell integration files (`claude-fzf.plugin.zsh`, `claude-fzf.bash`) can be simplified to just add the binary to PATH, or removed entirely if using tmux keybinding.

---

## References

- go-fzf: https://github.com/koki-develop/go-fzf
- gotmux: https://github.com/GianlucaP106/gotmux
- Current implementation: `claude-fzf`, `claude-fzf-tmux`, `claude-fzf-preview`
