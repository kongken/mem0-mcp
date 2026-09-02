package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	addCmd.Flags().String("role", "user", "role for plain-text input (user/assistant)")
	addCmd.Flags().String("metadata", "", "comma-separated metadata: --metadata importance=high,source=cli")
	addCmd.Flags().String("expiration-date", "", "YYYY-MM-DD expiry for the extracted memories")
	addCmd.Flags().Bool("infer", true, "extract facts from the text (default true)")
	addCmd.Flags().String("memory-type", "", "memory type, e.g. core")
	addCmd.Flags().String("prompt", "", "custom extraction prompt")
	addCmd.Flags().String("file", "", "read input from a file (JSON messages or plain text)")
	addCmd.Flags().String("json-messages", "", "raw JSON array of {role,content} messages")
}

var addCmd = &cobra.Command{
	Use:   "add [text]",
	Short: "Add a memory",
	Long: `Stores a memory. Input can come from a positional argument, a file
(--file), a raw JSON message array (--json-messages), or stdin when piped.

  mem0 add "Alice prefers dark mode" --user-id alice
  echo "I use vim" | mem0 add --user-id alice
  mem0 add --json-messages '[{"role":"user","content":"prefers Go"}]' --user-id alice
  mem0 add --file notes.json --user-id alice

The Mem0 OSS server requires at least one of user_id / agent_id / run_id.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runE("add", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		role, _ := cmd.Flags().GetString("role")
		metadataRaw, _ := cmd.Flags().GetString("metadata")
		expDate, _ := cmd.Flags().GetString("expiration-date")
		infer, _ := cmd.Flags().GetBool("infer")
		memType, _ := cmd.Flags().GetString("memory-type")
		promptStr, _ := cmd.Flags().GetString("prompt")
		file, _ := cmd.Flags().GetString("file")
		jsonMessages, _ := cmd.Flags().GetString("json-messages")

		if a.cfg.UserID == "" && a.cfg.AgentID == "" && a.cfg.RunID == "" {
			return fmt.Errorf("at least one of --user-id / --agent-id / --run-id is required")
		}

		messages, err := gatherMessages(role, args, file, jsonMessages, cmd)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return fmt.Errorf("no input provided; pass text, --file, --json-messages, or pipe stdin")
		}

		metadata, err := parseKV(metadataRaw)
		if err != nil {
			return err
		}

		ctx, cancel := runContext()
		defer cancel()
		results, err := a.client.Add(ctx, messages, client.AddOptions{
			UserID:         a.cfg.UserID,
			AgentID:        a.cfg.AgentID,
			RunID:          a.cfg.RunID,
			Metadata:       metadata,
			ExpirationDate: expDate,
			Infer:          &infer,
			MemoryType:     memType,
			Prompt:         promptStr,
		})
		if err != nil {
			return err
		}

		data := make([]map[string]any, 0, len(results))
		for _, r := range results {
			data = append(data, map[string]any{"id": r.ID, "memory": r.Memory, "event": r.Event})
		}
		return a.finish(len(data), data, func() error {
			switch a.output {
			case "quiet":
				for _, r := range results {
					fmt.Fprintln(a.stdout, r.ID)
				}
				return nil
			default:
				fmt.Fprintf(a.stdout, "Added %d memor%s:\n",
					len(results), plural(len(results), "y", "ies"))
				for _, r := range results {
					fmt.Fprintf(a.stdout, "  %s  %s\n", r.ID, shorten(r.Memory, 100))
				}
				return nil
			}
		})
	}),
}

// gatherMessages assembles the message list from the various input sources.
func gatherMessages(role string, args []string, file, jsonMessages string, cmd *cobra.Command) ([]client.Message, error) {
	haveSource := func() bool {
		return len(args) > 0 || file != "" || jsonMessages != ""
	}

	if jsonMessages != "" {
		var msgs []client.Message
		if err := json.Unmarshal([]byte(jsonMessages), &msgs); err != nil {
			return nil, fmt.Errorf("parse --json-messages: %w", err)
		}
		return msgs, nil
	}

	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		return parseFileInput(data, role)
	}

	if len(args) > 0 {
		return []client.Message{{Role: role, Content: args[0]}}, nil
	}

	// stdin is used when piped and nothing else was given.
	fi, _ := os.Stdin.Stat()
	if !isStdinCharDevice(fi) {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		if strings.TrimSpace(string(data)) != "" {
			return parseFileInput(data, role)
		}
	}
	_ = haveSource
	return nil, nil
}

func parseFileInput(data []byte, role string) ([]client.Message, error) {
	trimmed := strings.TrimSpace(string(data))
	var msgs []client.Message
	if err := json.Unmarshal([]byte(trimmed), &msgs); err == nil && len(msgs) > 0 {
		return msgs, nil
	}
	return []client.Message{{Role: role, Content: trimmed}}, nil
}

func isStdinCharDevice(fi os.FileInfo) bool {
	return fi != nil && fi.Mode()&os.ModeCharDevice != 0
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
