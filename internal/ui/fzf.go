package ui

import (
	"fmt"
	"path/filepath"
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
			return formatSessionLine(s)
		},
		fzf.WithPreviewWindow(func(i, w, h int) string {
			if i < 0 || i >= len(sessions) {
				return ""
			}
			return formatPreview(sessions[i])
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

func formatSessionLine(s session.Session) string {
	projectName := filepath.Base(s.ProjectPath)
	if projectName == "" || projectName == "." {
		projectName = "(no project)"
	}
	return fmt.Sprintf("%s  %-20s  %s",
		s.ModTime.Format("2006-01-02 15:04"),
		projectName,
		s.Summary)
}

func formatPreview(s session.Session) string {
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
