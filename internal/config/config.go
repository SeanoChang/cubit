package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SeanoChang/cubit/internal/claude"
	"github.com/spf13/viper"
)

type Config struct {
	Agent string      `yaml:"agent" mapstructure:"agent"`
	Root  string      `yaml:"root"  mapstructure:"root"`
	Claude ClaudeConfig `yaml:"claude" mapstructure:"claude"`
}

type ClaudeConfig struct {
	Model           string        `yaml:"model"            mapstructure:"model"`
	Timeout         time.Duration `yaml:"timeout"          mapstructure:"timeout"`
	MemoryModel     string        `yaml:"memory_model"     mapstructure:"memory_model"`
	RefreshJournals int           `yaml:"refresh_journals" mapstructure:"refresh_journals"`
	MaxParallel     int           `yaml:"max_parallel"     mapstructure:"max_parallel"`
	PermissionMode  string        `yaml:"permission_mode"  mapstructure:"permission_mode"`
	AllowedTools    []string      `yaml:"allowed_tools"    mapstructure:"allowed_tools"`
	WorkDir         string        `yaml:"work_dir"         mapstructure:"work_dir"`
}

func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".ark", "cubit")
	}
	return filepath.Join(home, ".ark", "cubit")
}

func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("agent", "noah")
	v.SetDefault("root", DefaultRoot())
	v.SetDefault("claude.model", "claude-opus-4-6")
	v.SetDefault("claude.timeout", "5m")
	v.SetDefault("claude.memory_model", "claude-sonnet-4-6")
	v.SetDefault("claude.refresh_journals", 5)
	v.SetDefault("claude.max_parallel", 0)

	// Config file location
	root := DefaultRoot()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(root)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		// No config file is fine — use defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Expand ~ in root path
	if cfg.Root == "~/.ark/cubit" {
		cfg.Root = DefaultRoot()
	}

	return &cfg, nil
}

// Default returns a Config with default values for the given agent.
func Default(agent string) *Config {
	return &Config{
		Agent: agent,
		Root:  DefaultRoot(),
		Claude: ClaudeConfig{
			Model:           "claude-opus-4-6",
			Timeout:         5 * time.Minute,
			MemoryModel:     "claude-sonnet-4-6",
			RefreshJournals: 5,
			MaxParallel:     0,
		},
	}
}

// AgentDir returns the full path to the active agent's directory.
func (c *Config) AgentDir() string {
	return filepath.Join(c.Root, c.Agent)
}

// RunnerOpts returns a claude.RunnerOpts populated from config.
func (c *ClaudeConfig) RunnerOpts() claude.RunnerOpts {
	return claude.RunnerOpts{
		Model:          c.Model,
		PermissionMode: c.PermissionMode,
		AllowedTools:   c.AllowedTools,
		Timeout:        c.Timeout,
		WorkDir:        c.WorkDir,
	}
}

// MemoryRunnerOpts returns RunnerOpts using the memory model instead of the
// primary model. Inherits permission mode, allowed tools, etc. from config.
func (c *ClaudeConfig) MemoryRunnerOpts() claude.RunnerOpts {
	opts := c.RunnerOpts()
	opts.Model = c.MemoryModel
	return opts
}
