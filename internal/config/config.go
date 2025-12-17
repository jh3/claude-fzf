package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Window defines a tmux window configuration
type Window struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command,omitempty"`
}

// Tmux contains tmux-related configuration
type Tmux struct {
	Windows []Window `yaml:"windows"`
}

// Config holds all configuration options
type Config struct {
	ProjectsDir string `yaml:"projects_dir,omitempty"`
	Tmux        Tmux   `yaml:"tmux"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Tmux: Tmux{
			Windows: []Window{
				{Name: "logs"},
				{Name: "edit"},
				{Name: "scratch"},
			},
		},
	}
}

// configPath returns the path to the config file
func configPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "claude-fzf", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "claude-fzf", "config.yaml")
}

// Load loads config from file, falling back to defaults
func Load() *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}

	return cfg
}

// Path returns the config file path (for help text)
func Path() string {
	return configPath()
}
