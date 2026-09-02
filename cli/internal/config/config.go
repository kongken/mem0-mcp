// Package config loads the CLI configuration from disk and applies
// environment-variable overrides, mirroring the official mem0 CLI precedence:
// flags > environment > config file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBaseURL is where a self-hosted Mem0 OSS server listens by default.
// Override with MEM0_BASE_URL or `mem0 init`.
const DefaultBaseURL = "http://localhost:8888"

// Config holds the CLI-wide settings persisted to ~/.mem0/config.json.
// Field names match the JSON keys used by the official mem0 CLI so configs
// are interchangeable.
type Config struct {
	APIKey  string `json:"api_key,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	RunID   string `json:"run_id,omitempty"`
}

// Path returns the location of the config file (~/.mem0/config.json).
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".mem0", "config.json"), nil
}

// Dir returns the directory holding the config file.
func Dir() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// Load reads the config file. A missing file is not an error: it yields a
// config with just the default base URL.
func Load() (*Config, error) {
	c := &Config{BaseURL: DefaultBaseURL}
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	return c, nil
}

// Save writes the config file with 0600 permissions so API keys are not
// world-readable.
func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(p, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", p, err)
	}
	return nil
}

// ApplyEnv overlays environment variables on top of the loaded config.
// Env vars never clear a value, they only set one, so an env override (e.g.
// MEM0_API_KEY) wins over the file while flags still win over everything
// (applied by the CLI layer).
func (c *Config) ApplyEnv() {
	if v := os.Getenv("MEM0_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("MEM0_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	// MEM0_USER_ID is the official variable; MEM0_DEFAULT_USER_ID is kept for
	// compatibility with the sibling mem0-mcp server.
	if v := os.Getenv("MEM0_USER_ID"); v != "" {
		c.UserID = v
	} else if v := os.Getenv("MEM0_DEFAULT_USER_ID"); v != "" {
		c.UserID = v
	}
	if v := os.Getenv("MEM0_AGENT_ID"); v != "" {
		c.AgentID = v
	}
	if v := os.Getenv("MEM0_RUN_ID"); v != "" {
		c.RunID = v
	}
}

// Redacted returns a copy safe to print, with the API key masked.
func (c *Config) Redacted() Config {
	out := *c
	if out.APIKey != "" {
		out.APIKey = MaskSecret(out.APIKey)
	}
	return out
}

// MaskSecret renders a secret like "m0sk****abcd", keeping only a short
// prefix and suffix visible.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "********" + s[len(s)-4:]
}
