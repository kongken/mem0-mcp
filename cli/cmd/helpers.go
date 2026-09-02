package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/kongken/mem0-mcp/cli/internal/client"
)

// parseKV parses "key=value,key2=value2" into a map. Values with commas are
// not supported; any unknown segment yields an error.
func parseKV(raw string) (map[string]any, error) {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("invalid key=value pair %q (want e.g. --metadata importance=high)", part)
		}
		out[strings.TrimSpace(key)] = parseScalar(strings.TrimSpace(val))
	}
	return out, nil
}

// parseScalar converts "true"/"42"/"3.5" into JSON-native types, leaving
// everything else as a string.
func parseScalar(v string) any {
	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

// renderTable draws rows with the given header and column formatters.
func renderTable(w interface{ Write([]byte) (int, error) }, header []string, rows [][]string) {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.AppendHeader(toRow(header))
	for _, r := range rows {
		t.AppendRow(toRow(r))
	}
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = true
	t.Style().Options.SeparateHeader = true
	t.Style().Options.SeparateRows = false
	t.SetColumnConfigs([]table.ColumnConfig{})
	t.Render()
}

func toRow(cells []string) table.Row {
	row := make(table.Row, len(cells))
	for i, c := range cells {
		row[i] = c
	}
	return row
}

// formatTime renders a timestamp compactly.
func formatTime(t *client.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

// firstKey returns the first metadata key/value pair, for compact tables.
func metadataSummary(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	k := keys[0]
	return fmt.Sprintf("%s=%v", k, m[k])
}

// shorten truncates long text for table cells.
func shorten(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
