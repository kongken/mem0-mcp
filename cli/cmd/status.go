package cmd

import (
	"fmt"

	"github.com/kongken/mem0-mcp/cli/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verify the connection to your Mem0 OSS server",
	Long: `Verifies the API key against the configured base URL and prints the
server's effective configuration (LLM, embedder, vector store) plus the
CLI's own settings.`,
	Args: cobra.NoArgs,
	RunE: runE("status", func(a *app, cmd *cobra.Command, _ []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		ctx, cancel := runContext()
		defer cancel()

		serverCfg, err := a.client.ServerConfig(ctx)
		if err != nil {
			return fmt.Errorf("server unreachable: %w", err)
		}
		providers, err := a.client.Providers(ctx)
		if err != nil {
			return fmt.Errorf("fetch providers: %w", err)
		}

		data := map[string]any{
			"base_url":  a.cfg.BaseURL,
			"api_key":   maskKey(a.cfg.APIKey),
			"user_id":   a.cfg.UserID,
			"agent_id":  a.cfg.AgentID,
			"run_id":    a.cfg.RunID,
			"server":    serverCfg,
			"providers": providers,
		}
		if a.agent {
			return a.emitAgent(0, data)
		}
		if a.output == "json" {
			return writeJSON(a.stdout, data)
		}

		fmt.Fprintf(a.stdout, "Connected to %s — %s\n", a.cfg.BaseURL, green("OK"))
		fmt.Fprintf(a.stdout, "  api key : %s\n", maskKey(a.cfg.APIKey))
		if a.cfg.UserID != "" {
			fmt.Fprintf(a.stdout, "  user_id : %s\n", a.cfg.UserID)
		}
		if v := serverCfg["version"]; v != nil {
			fmt.Fprintf(a.stdout, "  server  : Mem0 OSS (config version %v)\n", v)
		}
		if llm, ok := serverCfg["llm"].(map[string]any); ok {
			fmt.Fprintf(a.stdout, "  llm     : %s\n", fmtProvider(llm))
		}
		if emb, ok := serverCfg["embedder"].(map[string]any); ok {
			fmt.Fprintf(a.stdout, "  embedder: %s\n", fmtProvider(emb))
		}
		if vs, ok := serverCfg["vector_store"].(map[string]any); ok {
			if p, _ := vs["provider"].(string); p != "" {
				fmt.Fprintf(a.stdout, "  vector  : %s\n", p)
			}
		}
		if llms, ok := providers["llm"].([]any); ok && len(llms) > 0 {
			fmt.Fprintf(a.stdout, "  bundled : llm=%v embedder=%v\n", llms, providers["embedder"])
		}
		return nil
	}),
}

func maskKey(k string) string {
	if k == "" {
		return "(none)"
	}
	return config.MaskSecret(k)
}

func fmtProvider(m map[string]any) string {
	cfg, _ := m["config"].(map[string]any)
	if cfg == nil {
		return "?"
	}
	model, _ := cfg["model"].(string)
	provider, _ := m["provider"].(string)
	if model == "" {
		return provider
	}
	return fmt.Sprintf("%s (%s)", provider, model)
}

func green(s string) string {
	return "\x1b[32m" + s + "\x1b[0m"
}
