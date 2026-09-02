// Package cmd implements the mem0 CLI command tree.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/kongken/mem0-mcp/cli/internal/config"
	"github.com/spf13/cobra"
)

// version is overridden at build time with -ldflags "-X ...cmd.version=...".
var version = "0.1.0"

// app holds the resolved runtime state for one invocation.
type app struct {
	cfg     *config.Config
	client  *client.Client
	agent   bool // --agent / --json
	output  string
	timeout time.Duration
	stdout  io.Writer
	stderr  io.Writer
	command string
	started time.Time
}

// App is the resolved runtime state for one invocation.
var App = &app{
	output:  "text",
	timeout: 30 * time.Second,
	stdout:  os.Stdout,
	stderr:  os.Stderr,
}

func (a *app) durationMS() int64 {
	if a.started.IsZero() {
		return 0
	}
	return time.Since(a.started).Milliseconds()
}

// scope returns the identity keys passed on the command line.
func (a *app) scope() map[string]string {
	m := map[string]string{}
	if a.cfg.UserID != "" {
		m["user_id"] = a.cfg.UserID
	}
	if a.cfg.AgentID != "" {
		m["agent_id"] = a.cfg.AgentID
	}
	if a.cfg.RunID != "" {
		m["run_id"] = a.cfg.RunID
	}
	return m
}

// envelope is the agent-mode output shape.
type envelope struct {
	Status     string            `json:"status"`
	Command    string            `json:"command"`
	DurationMs int64             `json:"duration_ms"`
	Scope      map[string]string `json:"scope,omitempty"`
	Count      int               `json:"count,omitempty"`
	Data       any               `json:"data,omitempty"`
}

var rootCmd = &cobra.Command{
	Use:   "mem0",
	Short: "Mem0 CLI — memory layer for AI agents (self-hosted Mem0 OSS API)",
	Long: `Mem0 CLI talks to your self-hosted Mem0 OSS server
(https://github.com/mem0ai/mem0, server/ directory) over its REST API.

Configuration is resolved in this order: flags > environment > ~/.mem0/config.json.
Configure once with ` + "`mem0 init`" + ` or point at an existing setup:

  export MEM0_BASE_URL=http://localhost:8888
  export MEM0_API_KEY=m0sk_xxx

Pass --agent (or --json) anywhere for structured JSON output designed for
AI-agent tool loops.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := loadResolvedConfig(cmd)
		if err != nil {
			return err
		}
		timeout, err := cmd.Flags().GetDuration("timeout")
		if err != nil {
			return err
		}
		agent, _ := cmd.Flags().GetBool("agent")
		jsonOut, _ := cmd.Flags().GetBool("json")
		App.cfg = cfg
		App.client = client.New(cfg.BaseURL, cfg.APIKey, timeout)
		App.timeout = timeout
		App.agent = agent || jsonOut
		if out, _ := cmd.Flags().GetString("output"); out != "" {
			App.output = out
		}
		App.stdout = os.Stdout
		App.stderr = os.Stderr
		return nil
	},
}

// loadResolvedConfig applies flags on top of file+env config.
func loadResolvedConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	cfg.ApplyEnv()

	if v, _ := cmd.Flags().GetString("api-key"); v != "" {
		cfg.APIKey = v
	}
	if v, _ := cmd.Flags().GetString("base-url"); v != "" {
		cfg.BaseURL = v
	}
	if v, _ := cmd.Flags().GetString("user-id"); v != "" {
		cfg.UserID = v
	}
	if v, _ := cmd.Flags().GetString("agent-id"); v != "" {
		cfg.AgentID = v
	}
	if v, _ := cmd.Flags().GetString("run-id"); v != "" {
		cfg.RunID = v
	}
	return cfg, nil
}

// runE wraps every command body: it records timing, routes errors through the
// configured mode (agent JSON vs human text), and prevents cobra from dumping
// usage text on runtime failures.
func runE(name string, fn func(a *app, cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		App.started = time.Now()
		App.command = name
		if err := fn(App, cmd, args); err != nil {
			if App.agent {
				fmt.Fprintln(App.stdout, mustJSON(envelope{
					Status:     "error",
					Command:    name,
					DurationMs: App.durationMS(),
					Data:       map[string]string{"error": err.Error()},
				}))
				return err
			}
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
}

// emitAgent prints a success envelope in agent mode.
func (a *app) emitAgent(count int, data any) error {
	fmt.Fprintln(a.stdout, mustJSON(envelope{
		Status:     "success",
		Command:    a.command,
		DurationMs: a.durationMS(),
		Scope:      a.scope(),
		Count:      count,
		Data:       data,
	}))
	return nil
}

// finish is the common tail for every command: agent mode emits an envelope,
// otherwise the human renderer is called.
func (a *app) finish(count int, data any, render func() error) error {
	if a.agent {
		return a.emitAgent(count, data)
	}
	if a.output == "json" {
		return writeJSON(a.stdout, data)
	}
	return render()
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return `{"status":"error","error":"failed to encode JSON"}`
	}
	return string(b)
}

// writeJSON pretty-prints a value to w.
func writeJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// Execute runs the CLI and returns an honest exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		// Agent-mode errors were already printed as JSON on stdout.
		if !App.agent {
			fmt.Fprintf(App.stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}

// runContext derives a cancellable context tied to the process.
func runContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), App.timeout)
}

// requireAPIKey returns a helpful error when no API key is configured.
func requireAPIKey() error {
	if App.cfg.APIKey == "" {
		return errors.New("no API key configured; run `mem0 init` or set MEM0_API_KEY")
	}
	return nil
}

// isUnknownCommand guards the help fallback for unknown subcommands.
func isUnknownCommand(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown command")
}

func init() {
	rootCmd.PersistentFlags().String("api-key", "", "API key (m0sk_...); overrides MEM0_API_KEY")
	rootCmd.PersistentFlags().String("base-url", "", "Mem0 OSS base URL; overrides MEM0_BASE_URL")
	rootCmd.PersistentFlags().String("user-id", "", "default user_id scope")
	rootCmd.PersistentFlags().String("agent-id", "", "default agent_id scope")
	rootCmd.PersistentFlags().String("run-id", "", "default run_id scope")
	rootCmd.PersistentFlags().Duration("timeout", 30*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&App.output, "output", "text",
		"output format: text, json, table, quiet")
	var agent, jsonOut bool
	rootCmd.PersistentFlags().BoolVar(&agent, "agent", false,
		"structured JSON output for agent tool loops")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false,
		"alias for --agent")
	rootCmd.SetVersionTemplate("mem0 version {{.Version}}\n")
	rootCmd.Version = version

	rootCmd.AddCommand(
		initCmd,
		addCmd,
		searchCmd,
		listCmd,
		getCmd,
		updateCmd,
		deleteCmd,
		importCmd,
		entityCmd,
		configCmd,
		statusCmd,
		versionCmd,
	)
}
