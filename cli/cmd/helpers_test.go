package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kongken/mem0-mcp/cli/internal/client"
)

func TestParseKV(t *testing.T) {
	m, err := parseKV("importance=high,retries=3,ratio=0.5,ok=true,note=hello")
	if err != nil {
		t.Fatalf("parseKV: %v", err)
	}
	if m["importance"] != "high" {
		t.Errorf("importance = %v", m["importance"])
	}
	if m["retries"] != int64(3) {
		t.Errorf("retries = %v (%T)", m["retries"], m["retries"])
	}
	if m["ratio"] != 0.5 {
		t.Errorf("ratio = %v", m["ratio"])
	}
	if m["ok"] != true {
		t.Errorf("ok = %v", m["ok"])
	}
	if m["note"] != "hello" {
		t.Errorf("note = %v", m["note"])
	}
}

func TestParseKVRejectsBareKey(t *testing.T) {
	if _, err := parseKV("no-value-here"); err == nil {
		t.Fatal("expected error for key without =")
	}
}

func TestParseKVEmpty(t *testing.T) {
	m, err := parseKV("")
	if err != nil {
		t.Fatalf("parseKV(empty): %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestMemoryDataSanitizedShape(t *testing.T) {
	// Verify the agent-envelope memory shape serializes to the documented
	// sanitized fields only.
	user := "alice"
	data := memoryData(client.Memory{ID: "abc", Memory: "hello", UserID: &user})
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	for _, want := range []string{`"id":"abc"`, `"memory":"hello"`, `"user_id":"alice"`} {
		if !strings.Contains(s, want) {
			t.Errorf("envelope missing %s in %s", want, s)
		}
	}
}

type clientMemory = client.Memory
