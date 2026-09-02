package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Args:  cobra.NoArgs,
	RunE: runE("version", func(a *app, cmd *cobra.Command, _ []string) error {
		if a.agent {
			return a.emitAgent(0, map[string]string{"version": version})
		}
		fmt.Fprintf(a.stdout, "mem0 version %s\n", version)
		return nil
	}),
}
