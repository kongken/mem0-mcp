package cmd

import (
	"fmt"
	"strings"

	"github.com/kongken/mem0-mcp/cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify CLI configuration",
	Long: `Inspects or edits ~/.mem0/config.json.

  mem0 config                      # show current settings
  mem0 config set base_url http://localhost:8888
  mem0 config set user_id alice
  mem0 config unset user_id
  mem0 config path

Keys: api_key, base_url, user_id, agent_id, run_id.`,
	Args: cobra.ArbitraryArgs,
	RunE: runE("config", func(a *app, cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return configShow(a)
		}
		switch args[0] {
		case "show":
			return configShow(a)
		case "path":
			p, err := config.Path()
			if err != nil {
				return err
			}
			if a.agent {
				return a.emitAgent(0, map[string]string{"config_path": p})
			}
			fmt.Fprintln(a.stdout, p)
			return nil
		case "set":
			if len(args) != 3 {
				return fmt.Errorf("usage: mem0 config set <key> <value>")
			}
			return configSet(a, args[1], args[2])
		case "unset":
			if len(args) != 2 {
				return fmt.Errorf("usage: mem0 config unset <key>")
			}
			return configUnset(a, args[1])
		default:
			return fmt.Errorf("unknown config subcommand %q (want show, set, unset, path)", args[0])
		}
	}),
}

func configShow(a *app) error {
	cfg := a.cfg
	if a.agent {
		return a.emitAgent(0, map[string]any{
			"config_path": mustConfigPath(),
			"base_url":    cfg.BaseURL,
			"api_key":     maskKey(cfg.APIKey),
			"user_id":     cfg.UserID,
			"agent_id":    cfg.AgentID,
			"run_id":      cfg.RunID,
		})
	}
	if a.output == "json" {
		return writeJSON(a.stdout, map[string]any{
			"config_path": mustConfigPath(),
			"base_url":    cfg.BaseURL,
			"api_key":     maskKey(cfg.APIKey),
			"user_id":     cfg.UserID,
			"agent_id":    cfg.AgentID,
			"run_id":      cfg.RunID,
		})
	}
	fmt.Fprintf(a.stdout, "config:  %s\n", mustConfigPath())
	fmt.Fprintf(a.stdout, "base_url: %s\n", cfg.BaseURL)
	fmt.Fprintf(a.stdout, "api_key:  %s\n", maskKey(cfg.APIKey))
	fmt.Fprintf(a.stdout, "user_id:  %s\n", orDash(cfg.UserID))
	fmt.Fprintf(a.stdout, "agent_id: %s\n", orDash(cfg.AgentID))
	fmt.Fprintf(a.stdout, "run_id:   %s\n", orDash(cfg.RunID))
	return nil
}

func configSet(a *app, key, value string) error {
	cfg := a.cfg
	switch strings.ToLower(key) {
	case "api_key", "api-key":
		cfg.APIKey = value
	case "base_url", "base-url":
		cfg.BaseURL = strings.TrimRight(value, "/")
	case "user_id", "user-id":
		cfg.UserID = value
	case "agent_id", "agent-id":
		cfg.AgentID = value
	case "run_id", "run-id":
		cfg.RunID = value
	default:
		return fmt.Errorf("unknown config key %q (want api_key, base_url, user_id, agent_id, run_id)", key)
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	if a.agent {
		return a.emitAgent(0, map[string]any{"key": key, "value": value, "saved": true})
	}
	fmt.Fprintf(a.stdout, "Set %s = %s\n", key, value)
	return nil
}

func configUnset(a *app, key string) error {
	cfg := a.cfg
	switch strings.ToLower(key) {
	case "api_key", "api-key":
		cfg.APIKey = ""
	case "base_url", "base-url":
		cfg.BaseURL = config.DefaultBaseURL
	case "user_id", "user-id":
		cfg.UserID = ""
	case "agent_id", "agent-id":
		cfg.AgentID = ""
	case "run_id", "run-id":
		cfg.RunID = ""
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	if a.agent {
		return a.emitAgent(0, map[string]any{"key": key, "saved": true})
	}
	fmt.Fprintf(a.stdout, "Unset %s\n", key)
	return nil
}

func mustConfigPath() string {
	p, err := config.Path()
	if err != nil {
		return "~/.mem0/config.json"
	}
	return p
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
