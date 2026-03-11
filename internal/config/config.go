package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Agent string `yaml:"agent" mapstructure:"agent"`
	Root  string `yaml:"root"  mapstructure:"root"`
}

func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".ark")
	}
	return filepath.Join(home, ".ark")
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("agent", "noah")
	v.SetDefault("root", DefaultRoot())

	root := DefaultRoot()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(root)

	// Also check old v0.x config location
	home, _ := os.UserHomeDir()
	oldRoot := filepath.Join(home, ".ark", "cubit")
	v.AddConfigPath(oldRoot)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Normalize root — if it's the old path, update to new default
	if cfg.Root == oldRoot {
		cfg.Root = DefaultRoot()
	}
	if cfg.Root == "~/.ark" {
		cfg.Root = DefaultRoot()
	}

	return &cfg, nil
}

func Default(agent string) *Config {
	return &Config{
		Agent: agent,
		Root:  DefaultRoot(),
	}
}

func (c *Config) AgentDir() string {
	return filepath.Join(c.Root, c.Agent)
}

// IsLegacyLayout returns true if the old v0.x layout exists at ~/.ark/cubit/<agent>/.
// Old layout had identity/, queue/, memory/sessions/ subdirectories.
func (c *Config) IsLegacyLayout() bool {
	home, _ := os.UserHomeDir()
	oldRoot := filepath.Join(home, ".ark", "cubit")
	if c.Root == oldRoot {
		return true
	}
	oldAgentDir := filepath.Join(oldRoot, c.Agent)
	_, err := os.Stat(filepath.Join(oldAgentDir, "identity"))
	return err == nil
}

// LegacyAgentDir returns the old v0.x agent directory path.
func (c *Config) LegacyAgentDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ark", "cubit", c.Agent)
}
