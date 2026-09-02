package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var entityCmd = &cobra.Command{
	Use:   "entity",
	Short: "Manage entities (users, agents, runs)",
	Args:  cobra.NoArgs,
	RunE:  runE("entity", entitySummary),
}

var entityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entities and their memory counts",
	Args:  cobra.NoArgs,
	RunE:  runE("entity list", entityList),
}

var entityDeleteCmd = &cobra.Command{
	Use:   "delete <type> <entity-id>",
	Short: "Delete an entity and all its memories (admin key required)",
	Long: `Removes every memory owned by the entity. <type> is one of user, agent,
run. Requires an admin API key.

  mem0 entity delete user alice`,
	Args: cobra.ExactArgs(2),
	RunE: runE("entity delete", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		entityType, entityID := args[0], args[1]
		switch entityType {
		case "user", "agent", "run":
		default:
			return fmt.Errorf("entity type must be user, agent, or run (got %q)", entityType)
		}
		ctx, cancel := runContext()
		defer cancel()
		if err := a.client.DeleteEntity(ctx, entityType, entityID); err != nil {
			return err
		}
		return a.finish(1, map[string]string{
			"type": entityType, "id": entityID, "status": "deleted",
		}, func() error {
			fmt.Fprintf(a.stdout, "Deleted %s entity %q\n", entityType, entityID)
			return nil
		})
	}),
}

func entitySummary(_ *app, cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func entityList(a *app, _ *cobra.Command, _ []string) error {
	if err := requireAPIKey(); err != nil {
		return err
	}
	ctx, cancel := runContext()
	defer cancel()
	entities, err := a.client.ListEntities(ctx)
	if err != nil {
		return err
	}

	data := make([]map[string]any, 0, len(entities))
	for _, e := range entities {
		data = append(data, map[string]any{
			"id": e.ID, "type": e.Type, "total_memories": e.TotalMemories,
			"created_at": formatTime(e.CreatedAt), "updated_at": formatTime(e.UpdatedAt),
		})
	}
	return a.finish(len(data), data, func() error {
		switch a.output {
		case "quiet":
			for _, e := range entities {
				fmt.Fprintf(a.stdout, "%s\t%s\n", e.Type, e.ID)
			}
			return nil
		case "table":
			rows := make([][]string, 0, len(entities))
			for _, e := range entities {
				rows = append(rows, []string{
					e.Type, e.ID, fmt.Sprintf("%d", e.TotalMemories),
					formatTime(e.CreatedAt), formatTime(e.UpdatedAt),
				})
			}
			renderTable(a.stdout, []string{"Type", "ID", "Memories", "Created", "Updated"}, rows)
			return nil
		default:
			if len(entities) == 0 {
				fmt.Fprintln(a.stdout, "No entities found.")
				return nil
			}
			for _, e := range entities {
				fmt.Fprintf(a.stdout, "%-5s %-20s %d memories\n", e.Type, e.ID, e.TotalMemories)
			}
			return nil
		}
	})
}

func init() {
	entityCmd.AddCommand(entityListCmd, entityDeleteCmd)
}
