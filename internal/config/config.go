package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Agent string      `yaml:"agent" mapstructure:"agent"`
	Root  string      `yaml:"root"  mapstructure:"root"`
	Claude ClaudeConfig `yaml:"claude" mapstructure:"claude"`
}

type ClaudeConfig struct {
	Model       string        `yaml:"model"        mapstructure:"model"`
	Timeout     time.Duration `yaml:"timeout"      mapstructure:"timeout"`
	MemoryModel string        `yaml:"memory_model" mapstructure:"memory_model"`
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
			Model:       "claude-opus-4-6",
			Timeout:     5 * time.Minute,
			MemoryModel: "claude-sonnet-4-6",
		},
	}
}

// AgentDir returns the full path to the active agent's directory.
func (c *Config) AgentDir() string {
	return filepath.Join(c.Root, c.Agent)
}
