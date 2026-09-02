package cmd

import (
	"fmt"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().Int("limit", 0, "maximum number of memories to return (default: server limit)")
	listCmd.Flags().Bool("show-expired", false, "include expired memories")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List memories",
	Long: `Lists stored memories. Without a scope it lists everything (requires an
admin key); pass --user-id / --agent-id / --run-id to scope it.

  mem0 list --user-id alice
  mem0 list --limit 20                          # admin, first 20
  mem0 list --user-id alice --output table`,
	Args: cobra.NoArgs,
	RunE: runE("list", func(a *app, cmd *cobra.Command, _ []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		showExpired, _ := cmd.Flags().GetBool("show-expired")

		ctx, cancel := runContext()
		defer cancel()
		memories, err := a.client.List(ctx, client.Scope{
			UserID:  a.cfg.UserID,
			AgentID: a.cfg.AgentID,
			RunID:   a.cfg.RunID,
		}, limit, showExpired)
		if err != nil {
			return err
		}

		data := make([]map[string]any, 0, len(memories))
		for _, m := range memories {
			data = append(data, memoryData(m))
		}
		return a.finish(len(data), data, func() error {
			switch a.output {
			case "quiet":
				for _, m := range memories {
					fmt.Fprintln(a.stdout, m.ID)
				}
				return nil
			case "table":
				rows := make([][]string, 0, len(memories))
				for _, m := range memories {
					rows = append(rows, []string{
						m.ID, shorten(m.Memory, 70), m.ScopeOwner(),
						formatTime(m.CreatedAt), formatTime(m.UpdatedAt),
					})
				}
				renderTable(a.stdout, []string{"ID", "Memory", "Scope", "Created", "Updated"}, rows)
				return nil
			default:
				if len(memories) == 0 {
					fmt.Fprintln(a.stdout, "No memories found.")
					return nil
				}
				fmt.Fprintf(a.stdout, "Found %d memor%s:\n", len(memories), plural(len(memories), "y", "ies"))
				for _, m := range memories {
					fmt.Fprintf(a.stdout, "  %s  %s\n", m.ID, m.Memory)
					if m.ExpirationDate != nil && *m.ExpirationDate != "" {
						fmt.Fprintf(a.stdout, "         expires=%s\n", *m.ExpirationDate)
					}
				}
				return nil
			}
		})
	}),
}

// memoryData is the sanitized agent-mode shape for a stored memory.
func memoryData(m client.Memory) map[string]any {
	owner := ""
	if m.UserID != nil {
		owner = *m.UserID
	}
	return map[string]any{
		"id":         m.ID,
		"memory":     m.Memory,
		"user_id":    owner,
		"scope":      m.ScopeOwner(),
		"created_at": formatTime(m.CreatedAt),
		"updated_at": formatTime(m.UpdatedAt),
	}
}
