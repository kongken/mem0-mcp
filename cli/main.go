// Command mem0 is a Go client for the self-hosted Mem0 OSS REST API.
package main

import (
	"os"

	"github.com/kongken/mem0-mcp/cli/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
