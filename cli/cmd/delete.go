package cmd

import (
	"fmt"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	deleteCmd.Flags().Bool("all", false, "delete all memories in the given scope")
}

var deleteCmd = &cobra.Command{
	Use:   "delete <memory-id>...",
	Short: "Delete one or more memories",
	Long: `Deletes memories by ID, or with --all removes every memory in a scope
(user/agent/run). Deleting a scope is irreversible.

  mem0 delete 3f2a...00 8b1c...11
  mem0 delete --all --user-id alice           # wipe Alice's memories
  mem0 delete --all --agent-id support-bot`,
	Args: cobra.ArbitraryArgs,
	RunE: runE("delete", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		all, _ := cmd.Flags().GetBool("all")

		ctx, cancel := runContext()
		defer cancel()

		if all {
			if len(args) > 0 {
				return fmt.Errorf("--all does not take memory IDs; scope with --user-id/--agent-id/--run-id")
			}
			scope := client.Scope{UserID: a.cfg.UserID, AgentID: a.cfg.AgentID, RunID: a.cfg.RunID}
			if scope.UserID == "" && scope.AgentID == "" && scope.RunID == "" {
				return fmt.Errorf("--all requires a scope (--user-id / --agent-id / --run-id)")
			}
			if err := a.client.DeleteAll(ctx, scope); err != nil {
				return err
			}
			return a.finish(0, map[string]any{
				"deleted": "scope",
				"scope":   a.scope(),
			}, func() error {
				fmt.Fprintf(a.stdout, "Deleted all memories for %s\n", formatScope(a.scope()))
				return nil
			})
		}

		if len(args) == 0 {
			return fmt.Errorf("provide at least one memory ID (or use --all to wipe a scope)")
		}

		deleted := make([]string, 0, len(args))
		for _, id := range args {
			if err := a.client.Delete(ctx, id); err != nil {
				return fmt.Errorf("delete %s: %w", id, err)
			}
			deleted = append(deleted, id)
		}
		return a.finish(len(deleted), deleted, func() error {
			for _, id := range deleted {
				fmt.Fprintf(a.stdout, "Deleted %s\n", id)
			}
			return nil
		})
	}),
}

func formatScope(scope map[string]string) string {
	out := ""
	for _, k := range []string{"user_id", "agent_id", "run_id"} {
		if v, ok := scope[k]; ok && v != "" {
			if out != "" {
				out += ", "
			}
			out += k + "=" + v
		}
	}
	if out == "" {
		return "(none)"
	}
	return out
}
