package cmd

import (
	"fmt"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	searchCmd.Flags().Int("top-k", 0, "maximum number of results")
	searchCmd.Flags().Float64("threshold", 0, "minimum similarity score (0-1)")
	searchCmd.Flags().Bool("explain", false, "include score details")
	searchCmd.Flags().Bool("show-expired", false, "include expired memories")
	searchCmd.Flags().String("filter", "", "comma-separated extra filters: --filter source=bot")
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories with natural language",
	Long: `Runs a semantic search across stored memories.

  mem0 search "what does alice prefer?" --user-id alice
  mem0 search "deploy steps" --top-k 5 --threshold 0.5

Scope identity (--user-id / --agent-id / --run-id) is sent inside the filter
object, matching the modern OSS API.`,
	Args: cobra.ExactArgs(1),
	RunE: runE("search", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		topK, _ := cmd.Flags().GetInt("top-k")
		threshold, _ := cmd.Flags().GetFloat64("threshold")
		explain, _ := cmd.Flags().GetBool("explain")
		showExpired, _ := cmd.Flags().GetBool("show-expired")
		filterRaw, _ := cmd.Flags().GetString("filter")

		extra, err := parseKV(filterRaw)
		if err != nil {
			return err
		}

		ctx, cancel := runContext()
		defer cancel()
		results, err := a.client.Search(ctx, args[0], client.SearchOptions{
			UserID:       a.cfg.UserID,
			AgentID:      a.cfg.AgentID,
			RunID:        a.cfg.RunID,
			Filters:      extra,
			TopK:         topK,
			Threshold:    threshold,
			HasThreshold: cmd.Flags().Changed("threshold"),
			Explain:      explain,
			ShowExpired:  showExpired,
		})
		if err != nil {
			return err
		}

		data := make([]map[string]any, 0, len(results))
		for _, r := range results {
			data = append(data, map[string]any{
				"id":         r.ID,
				"memory":     r.Memory,
				"score":      r.Score,
				"created_at": formatTime(r.CreatedAt),
			})
		}
		return a.finish(len(data), data, func() error {
			switch a.output {
			case "quiet":
				for _, r := range results {
					fmt.Fprintf(a.stdout, "%s\t%.3f\t%s\n", r.ID, r.Score, r.Memory)
				}
				return nil
			case "table":
				rows := make([][]string, 0, len(results))
				for _, r := range results {
					rows = append(rows, []string{
						r.ID, fmt.Sprintf("%.3f", r.Score), shorten(r.Memory, 70), formatTime(r.CreatedAt),
					})
				}
				renderTable(a.stdout, []string{"ID", "Score", "Memory", "Created"}, rows)
				return nil
			default:
				if len(results) == 0 {
					fmt.Fprintln(a.stdout, "No memories found.")
					return nil
				}
				fmt.Fprintf(a.stdout, "Found %d result%s:\n", len(results), plural(len(results), "", "s"))
				for _, r := range results {
					fmt.Fprintf(a.stdout, "  [%.3f] %s\n", r.Score, r.Memory)
					fmt.Fprintf(a.stdout, "         id=%s created=%s\n", r.ID, formatTime(r.CreatedAt))
				}
				return nil
			}
		})
	}),
}
