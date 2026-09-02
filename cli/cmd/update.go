package cmd

import (
	"fmt"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().String("metadata", "", "replace metadata: --metadata importance=high,source=cli")
	updateCmd.Flags().String("expiration-date", "", "YYYY-MM-DD expiry, or pass --clear-expiration")
	updateCmd.Flags().Bool("clear-expiration", false, "clear the expiration date")
}

var updateCmd = &cobra.Command{
	Use:   "update <memory-id> [new-text]",
	Short: "Update a memory",
	Long: `Updates a memory's text and/or metadata. At least one of the new text
or --metadata must be provided.

  mem0 update 3f2a...00 "Alice switched to light mode"
  mem0 update 3f2a...00 --metadata importance=high
  mem0 update 3f2a...00 --expiration-date 2026-12-31`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runE("update", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		id := args[0]
		metadataRaw, _ := cmd.Flags().GetString("metadata")
		expDate, _ := cmd.Flags().GetString("expiration-date")
		clearExp, _ := cmd.Flags().GetBool("clear-expiration")

		var text *string
		if len(args) == 2 {
			t := args[1]
			text = &t
		}
		metadata, err := parseKV(metadataRaw)
		if err != nil {
			return err
		}
		if text == nil && !cmd.Flags().Changed("metadata") && !cmd.Flags().Changed("expiration-date") && !clearExp {
			return fmt.Errorf("nothing to update; provide new text, --metadata, or --expiration-date")
		}

		var expPtr *string
		if !clearExp && expDate != "" {
			expPtr = &expDate
		}

		ctx, cancel := runContext()
		defer cancel()
		m, err := a.client.Update(ctx, id, client.UpdateParams{
			Text:            text,
			Metadata:        metadata,
			SetMetadata:     cmd.Flags().Changed("metadata"),
			ExpirationDate:  expPtr,
			ClearExpiration: clearExp,
		})
		if err != nil {
			return err
		}

		return a.finish(1, memoryData(*m), func() error {
			if a.output == "quiet" {
				fmt.Fprintln(a.stdout, m.ID)
				return nil
			}
			fmt.Fprintf(a.stdout, "Updated %s\n", m.ID)
			fmt.Fprintf(a.stdout, "  %s\n", m.Memory)
			return nil
		})
	}),
}
