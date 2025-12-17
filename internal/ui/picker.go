package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jh3/claude-fzf/internal/session"
)

// Action represents what the user wants to do with the selected session
type Action int

const (
	ActionNone Action = iota
	ActionResume
	ActionDelete
	ActionNewProject
)

// Result holds the selected session and action
type Result struct {
	Session     *session.Session
	Action      Action
	ProjectPath string // for ActionNewProject
}

// pickerModel is the bubbletea model for the session picker
type pickerModel struct {
	allSessions      []session.Session
	filteredSessions []session.Session
	cursor           int
	filter           textinput.Model
	showEmpty        bool
	width            int
	height           int
	result           Result
	confirmDelete    bool
	quitting         bool
	newProjectMode   bool     // are we in new project path entry mode?
	projectsDir      string   // base directory from config (may be empty)
	existingDirs     []string // directories in projectsDir (for new project mode)
}

// Styles
var (
	selectedStyle   = lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("229"))
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	previewHeader   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	confirmStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

func newPickerModel(sessions []session.Session, showEmpty bool, projectsDir string) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	m := pickerModel{
		allSessions: sessions,
		filter:      ti,
		showEmpty:   showEmpty,
		width:       80,
		height:      24,
		projectsDir: projectsDir,
	}
	m.applyFilter()
	return m
}

func (m *pickerModel) applyFilter() {
	query := strings.ToLower(m.filter.Value())
	m.filteredSessions = nil

	for _, s := range m.allSessions {
		// Filter by showEmpty
		if !m.showEmpty && s.UserMsgCount == 0 && s.AsstMsgCount == 0 {
			continue
		}

		// Filter by search query
		if query != "" {
			searchText := strings.ToLower(s.ProjectPath + " " + s.Summary + " " + s.GitBranch)
			if !strings.Contains(searchText, query) {
				continue
			}
		}

		m.filteredSessions = append(m.filteredSessions, s)
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filteredSessions) {
		m.cursor = max(0, len(m.filteredSessions)-1)
	}
}

func (m pickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle delete confirmation mode
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				if len(m.filteredSessions) > 0 {
					// Delete the file
					sess := m.filteredSessions[m.cursor]
					os.Remove(sess.FilePath)

					// Remove from allSessions
					for i, s := range m.allSessions {
						if s.FilePath == sess.FilePath {
							m.allSessions = append(m.allSessions[:i], m.allSessions[i+1:]...)
							break
						}
					}

					// Reapply filter to update filteredSessions
					m.applyFilter()
				}
				m.confirmDelete = false
				return m, nil
			case "n", "N", "esc":
				m.confirmDelete = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			if m.newProjectMode {
				m.newProjectMode = false
				m.filter.SetValue("")
				m.filter.Placeholder = "Filter..."
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.newProjectMode {
				path := m.filter.Value()
				if path != "" {
					m.result = Result{
						Action:      ActionNewProject,
						ProjectPath: m.expandPath(path),
					}
					m.quitting = true
					return m, tea.Quit
				}
				return m, nil
			}
			if len(m.filteredSessions) > 0 {
				m.result = Result{
					Session: &m.filteredSessions[m.cursor],
					Action:  ActionResume,
				}
			}
			m.quitting = true
			return m, tea.Quit

		case "ctrl+n":
			m.newProjectMode = true
			m.filter.SetValue("")
			m.loadExistingDirs()
			if m.projectsDir != "" {
				m.filter.Placeholder = "Project name..."
			} else {
				m.filter.Placeholder = "Path (e.g. ~/projects/my-app)..."
				m.filter.SetValue("~/")
			}
			return m, nil

		case "ctrl+d":
			if len(m.filteredSessions) > 0 {
				m.confirmDelete = true
			}
			return m, nil

		case "ctrl+a":
			m.showEmpty = !m.showEmpty
			m.applyFilter()
			return m, nil

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down":
			if m.cursor < len(m.filteredSessions)-1 {
				m.cursor++
			}
			return m, nil

		case "pgup":
			m.cursor = max(0, m.cursor-10)
			return m, nil

		case "pgdown":
			m.cursor = min(len(m.filteredSessions)-1, m.cursor+10)
			return m, nil

		case "home", "ctrl+home":
			m.cursor = 0
			return m, nil

		case "end", "ctrl+end":
			m.cursor = max(0, len(m.filteredSessions)-1)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.filter.Width = min(40, msg.Width-20)
		return m, nil
	}

	// Handle text input for filtering
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m pickerModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header with filter
	if m.newProjectMode {
		header := "New Project"
		if m.projectsDir != "" {
			header += fmt.Sprintf(" (in %s)", m.projectsDir)
		}
		b.WriteString(fmt.Sprintf("%s %s\n\n", header, m.filter.View()))
	} else {
		emptyIndicator := ""
		if m.showEmpty {
			emptyIndicator = " [+empty]"
		}
		b.WriteString(fmt.Sprintf("Sessions %d/%d%s %s\n\n",
			len(m.filteredSessions), len(m.allSessions), emptyIndicator, m.filter.View()))
	}

	// Calculate layout
	listWidth := m.width / 2
	previewWidth := m.width - listWidth - 3
	listHeight := m.height - 6

	var listLines []string
	var previewLines []string

	if m.newProjectMode {
		// Build existing directories list
		contentWidth := listWidth - 2

		for i := 0; i < len(m.existingDirs) && i < listHeight; i++ {
			line := fixedWidth("  "+m.existingDirs[i], contentWidth+2)
			listLines = append(listLines, line)
		}

		// Pad list to full height
		emptyLine := strings.Repeat(" ", listWidth)
		for len(listLines) < listHeight {
			listLines = append(listLines, emptyLine)
		}

		// Build creation preview
		previewLines = m.formatNewProjectPreview(previewWidth)
	} else {
		// Build session list
		visibleStart := 0
		if m.cursor >= listHeight {
			visibleStart = m.cursor - listHeight + 1
		}

		// Fixed content width (excluding cursor prefix "  " or "> ")
		contentWidth := listWidth - 2

		for i := visibleStart; i < len(m.filteredSessions) && i < visibleStart+listHeight; i++ {
			s := m.filteredSessions[i]
			// Format and pad to fixed width BEFORE applying styles
			line := formatSessionLine(s, contentWidth)
			line = fixedWidth(line, contentWidth)

			if i == m.cursor {
				line = cursorStyle.Render("> ") + selectedStyle.Render(line)
			} else {
				line = "  " + line
			}
			listLines = append(listLines, line)
		}

		// Pad list to full height with fixed-width empty lines
		emptyLine := strings.Repeat(" ", listWidth)
		for len(listLines) < listHeight {
			listLines = append(listLines, emptyLine)
		}

		// Build preview
		if len(m.filteredSessions) > 0 && m.cursor < len(m.filteredSessions) {
			previewLines = formatPreviewLines(m.filteredSessions[m.cursor], previewWidth)
		}
	}

	// Pad preview to full height
	for len(previewLines) < listHeight {
		previewLines = append(previewLines, "")
	}

	// Combine list and preview side by side
	for i := 0; i < listHeight; i++ {
		listLine := listLines[i]
		previewLine := ""
		if i < len(previewLines) {
			previewLine = previewLines[i]
		}
		b.WriteString(fmt.Sprintf("%s │ %s\n", listLine, previewLine))
	}

	// Footer with help
	b.WriteString("\n")
	if m.confirmDelete {
		b.WriteString(confirmStyle.Render("Delete this session? (y/n)"))
	} else if m.newProjectMode {
		b.WriteString(helpStyle.Render("enter: create • esc: cancel"))
	} else {
		b.WriteString(helpStyle.Render("enter: resume • ctrl-d: delete • ctrl-a: toggle empty • ctrl-n: new • esc: quit"))
	}

	return b.String()
}

func formatSessionLine(s session.Session, maxWidth int) string {
	projectName := filepath.Base(s.ProjectPath)
	if projectName == "" || projectName == "." {
		projectName = "(no project)"
	}

	date := s.ModTime.Format("01/02 15:04")
	summary := s.Summary
	if summary == "" {
		summary = "(no summary)"
	}

	// Truncate to fit
	line := fmt.Sprintf("%s  %-18s  %s", date, truncate(projectName, 18), summary)
	if len(line) > maxWidth {
		line = line[:maxWidth-1] + "…"
	}
	return line
}

func formatPreviewLines(s session.Session, width int) []string {
	var lines []string

	lines = append(lines, previewHeader.Render("Session: ")+s.ID)
	lines = append(lines, previewHeader.Render("Project: ")+s.ProjectPath)
	lines = append(lines, "")

	if s.Summary != "" && s.Summary != "(no summary)" {
		lines = append(lines, previewHeader.Render("Summary:"))
		// Word wrap summary
		wrapped := wordWrap(s.Summary, width)
		lines = append(lines, wrapped...)
		lines = append(lines, "")
	}

	if s.GitBranch != "" {
		lines = append(lines, previewHeader.Render("Branch: ")+s.GitBranch)
	}

	lines = append(lines, fmt.Sprintf("Messages: %d user / %d assistant", s.UserMsgCount, s.AsstMsgCount))
	lines = append(lines, dimStyle.Render("Modified: "+s.ModTime.Format("2006-01-02 15:04:05")))

	return lines
}

func (m *pickerModel) formatNewProjectPreview(width int) []string {
	var lines []string

	input := m.filter.Value()
	if input == "" {
		lines = append(lines, dimStyle.Render("Enter a project name..."))
		return lines
	}

	fullPath := m.expandPath(input)

	lines = append(lines, previewHeader.Render("Will create:"))
	lines = append(lines, "")
	lines = append(lines, "  "+fullPath)
	lines = append(lines, "")
	lines = append(lines, previewHeader.Render("Actions:"))
	lines = append(lines, "  • Create directory")
	lines = append(lines, "  • Initialize git repo")
	lines = append(lines, "  • Start Claude session")

	// Check if path already exists
	if _, err := os.Stat(fullPath); err == nil {
		lines = append(lines, "")
		lines = append(lines, confirmStyle.Render("⚠ Path already exists!"))
	}

	return lines
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// fixedWidth ensures a string is exactly the given width (truncate or pad)
func fixedWidth(s string, width int) string {
	// Handle runes properly for unicode
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width-1]) + "…"
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}

func wordWrap(s string, width int) []string {
	var lines []string
	words := strings.Fields(s)
	var line string

	for _, word := range words {
		if line == "" {
			line = word
		} else if len(line)+1+len(word) <= width {
			line += " " + word
		} else {
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (m *pickerModel) expandPath(input string) string {
	// If projectsDir is set, prepend it
	if m.projectsDir != "" {
		input = filepath.Join(m.projectsDir, input)
	}

	// Expand ~
	if strings.HasPrefix(input, "~/") {
		home, _ := os.UserHomeDir()
		input = filepath.Join(home, input[2:])
	}

	return input
}

func (m *pickerModel) loadExistingDirs() {
	m.existingDirs = nil

	// Determine which directory to list
	dir := m.projectsDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = home
	}

	// Expand ~ if present
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			m.existingDirs = append(m.existingDirs, entry.Name())
		}
	}
}

// SelectSession runs the interactive picker and returns the result
func SelectSession(sessions []session.Session, showEmpty bool, projectsDir string) (Result, error) {
	if len(sessions) == 0 {
		return Result{}, fmt.Errorf("no sessions found")
	}

	m := newPickerModel(sessions, showEmpty, projectsDir)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}

	result := finalModel.(pickerModel).result
	return result, nil
}
