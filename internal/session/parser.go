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

// ParseFile extracts session data from a JSONL file
func ParseFile(path string) (Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Session{}, err
	}

	sess := Session{
		ID:       strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		FilePath: path,
		ModTime:  info.ModTime(),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var firstUserMsg string

	for scanner.Scan() {
		var line jsonLine
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}

		sess.processLine(line, &firstUserMsg)
	}

	sess.finalizeSummary(firstUserMsg)
	return sess, nil
}

func (s *Session) processLine(line jsonLine, firstUserMsg *string) {
	switch line.Type {
	case "user":
		s.UserMsgCount++
		if s.ProjectPath == "" && line.Cwd != "" {
			s.ProjectPath = line.Cwd
		}
		if *firstUserMsg == "" && line.Message.Content != "" {
			*firstUserMsg = line.Message.Content
		}
	case "assistant":
		s.AsstMsgCount++
	case "summary":
		if line.Summary != "" {
			s.Summary = line.Summary
		}
	}

	if line.GitBranch != "" && s.GitBranch == "" {
		s.GitBranch = line.GitBranch
	}
}

func (s *Session) finalizeSummary(firstUserMsg string) {
	if s.Summary != "" {
		return
	}
	if firstUserMsg != "" {
		s.Summary = truncate(firstUserMsg, 60)
		return
	}
	s.Summary = "(no summary)"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
