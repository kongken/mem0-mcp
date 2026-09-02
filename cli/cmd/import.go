package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kongken/mem0-mcp/cli/internal/client"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Bulk import memories from a JSON file",
	Long: `Imports memories with one POST /memories call per entry. The file is a
JSON array; each entry is either

  {"messages":[{"role":"user","content":"..."}], "user_id":"alice", "metadata":{...}}
  {"text":"plain text", "user_id":"bob"}          # shorthand for one message

Fields recognised per entry: messages (or text), user_id, agent_id, run_id,
metadata, expiration_date. Entries that fail are reported but do not abort
the import.`,
	Args: cobra.ExactArgs(1),
	RunE: runE("import", func(a *app, cmd *cobra.Command, args []string) error {
		if err := requireAPIKey(); err != nil {
			return err
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read %s: %w", args[0], err)
		}
		var entries []map[string]any
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("parse %s (want a JSON array of entries): %w", args[0], err)
		}
		if len(entries) == 0 {
			return fmt.Errorf("no entries in %s", args[0])
		}

		ctx, cancel := runContext()
		defer cancel()

		count := 0
		created := []map[string]any{}
		var failures []string
		for i, entry := range entries {
			opts, messages, err := importEntry(entry, a)
			if err != nil {
				failures = append(failures, fmt.Sprintf("[%d] %v", i+1, err))
				continue
			}
			results, err := a.client.Add(ctx, messages, opts)
			if err != nil {
				failures = append(failures, fmt.Sprintf("[%d] %v", i+1, err))
				continue
			}
			count += len(results)
			for _, r := range results {
				created = append(created, map[string]any{"id": r.ID, "memory": r.Memory})
			}
		}

		return a.finish(count, map[string]any{
			"imported": count,
			"total":    len(entries),
			"failed":   len(failures),
			"created":  created,
		}, func() error {
			fmt.Fprintf(a.stdout, "Imported %d memor%s from %d entr%s",
				count, plural(count, "y", "ies"), len(entries), plural(len(entries), "y", "ies"))
			if len(failures) > 0 {
				fmt.Fprintf(a.stdout, ", %d failed:\n", len(failures))
				for _, f := range failures {
					fmt.Fprintf(a.stdout, "  %s\n", f)
				}
			} else {
				fmt.Fprintln(a.stdout, ".")
			}
			return nil
		})
	}),
}

// importEntry maps one JSON object to an Add call.
func importEntry(entry map[string]any, a *app) (client.AddOptions, []client.Message, error) {
	var messages []client.Message

	if rawMessages, ok := entry["messages"].([]any); ok {
		for _, rm := range rawMessages {
			m, ok := rm.(map[string]any)
			if !ok {
				return client.AddOptions{}, nil, fmt.Errorf("message entry is not an object")
			}
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			if content == "" {
				return client.AddOptions{}, nil, fmt.Errorf("message missing content")
			}
			if role == "" {
				role = "user"
			}
			messages = append(messages, client.Message{Role: role, Content: content})
		}
	} else if text, ok := entry["text"].(string); ok && text != "" {
		messages = []client.Message{{Role: "user", Content: text}}
	}
	if len(messages) == 0 {
		return client.AddOptions{}, nil, fmt.Errorf("entry has neither messages[] nor text")
	}

	str := func(key string) string {
		if v, ok := entry[key].(string); ok {
			return v
		}
		return ""
	}
	metadata, _ := entry["metadata"].(map[string]any)

	opts := client.AddOptions{
		UserID:         firstNonEmpty(str("user_id"), a.cfg.UserID),
		AgentID:        str("agent_id"),
		RunID:          str("run_id"),
		Metadata:       metadata,
		ExpirationDate: str("expiration_date"),
	}
	if opts.UserID == "" && opts.AgentID == "" && opts.RunID == "" {
		return client.AddOptions{}, nil, fmt.Errorf("entry has no user_id/agent_id/run_id and no default scope")
	}
	return opts, messages, nil
}
