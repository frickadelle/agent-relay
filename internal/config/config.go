package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Agent struct {
	Model   string `toml:"model"`
	Timeout string `toml:"timeout"`
	Sandbox string `toml:"sandbox"`
}

type Config struct {
	DefaultTimeout string           `toml:"default_timeout"`
	SessionStorage string           `toml:"session_storage"`
	Agents         map[string]Agent `toml:"agents"`
}

func Dir() string {
	if x := os.Getenv("AGENT_RELAY_CONFIG_DIR"); x != "" {
		return x
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "agent-relay")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agent-relay"
	}
	return filepath.Join(home, ".config", "agent-relay")
}

func Path() string {
	return filepath.Join(Dir(), "config.toml")
}

func Default() *Config {
	return &Config{
		DefaultTimeout: "10m",
		SessionStorage: "json",
		Agents:         map[string]Agent{},
	}
}

func Load() (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Path(), err)
	}
	if cfg.SessionStorage != "json" && cfg.SessionStorage != "sqlite" {
		return nil, fmt.Errorf("invalid session_storage %q (want json or sqlite)", cfg.SessionStorage)
	}
	return cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	f, err := os.Create(Path())
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func (c *Config) Timeout(agent string) (time.Duration, error) {
	d := c.DefaultTimeout
	if a, ok := c.Agents[agent]; ok && a.Timeout != "" {
		d = a.Timeout
	}
	parsed, err := time.ParseDuration(d)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q for %s: %w", d, agent, err)
	}
	return parsed, nil
}

func (c *Config) Sandbox(agent string) string {
	if a, ok := c.Agents[agent]; ok && a.Sandbox != "" {
		return a.Sandbox
	}
	if agent == "codex" {
		return "read-only"
	}
	return ""
}
