package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	getCmd.Flags().Bool("history", false, "show the memory's change history")
}

var getCmd = &cobra.Command{
	Use:   "get <memory-id>",
	Short: "Get a single memory by ID",
	Long: `Retrieves one memory. With --history it shows the memory's edit history
instead of the current state.

  mem0 get 3f2a...00   --user-id alice
  mem0 get 3f2a...00   --history`,
	Args: cobra.ExactArgs(1),
	RunE: runE("get", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		id := args[0]
		history, _ := cmd.Flags().GetBool("history")

		ctx, cancel := runContext()
		defer cancel()

		if history {
			events, err := a.client.History(ctx, id)
			if err != nil {
				return err
			}
			data := make([]map[string]any, 0, len(events))
			for _, ev := range events {
				data = append(data, map[string]any{
					"id":         ev.ID,
					"memory":     ev.Memory,
					"event":      ev.Event,
					"created_at": formatTime(ev.CreatedAt),
				})
			}
			return a.finish(len(data), data, func() error {
				for _, ev := range events {
					fmt.Fprintf(a.stdout, "%s  %-14s %s\n",
						formatTime(ev.CreatedAt), ev.Event, shorten(ev.Memory, 80))
				}
				return nil
			})
		}

		m, err := a.client.Get(ctx, id)
		if err != nil {
			return err
		}
		return a.finish(1, memoryData(*m), func() error {
			fmt.Fprintf(a.stdout, "id:      %s\n", m.ID)
			fmt.Fprintf(a.stdout, "memory:  %s\n", m.Memory)
			fmt.Fprintf(a.stdout, "scope:   %s\n", m.ScopeOwner())
			fmt.Fprintf(a.stdout, "created: %s\n", formatTime(m.CreatedAt))
			fmt.Fprintf(a.stdout, "updated: %s\n", formatTime(m.UpdatedAt))
			if m.ExpirationDate != nil && *m.ExpirationDate != "" {
				fmt.Fprintf(a.stdout, "expires: %s\n", *m.ExpirationDate)
			}
			if len(m.Metadata) > 0 {
				fmt.Fprintf(a.stdout, "metadata: %v\n", m.Metadata)
			}
			return nil
		})
	}),
}
