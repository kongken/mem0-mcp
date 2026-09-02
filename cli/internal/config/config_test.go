package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.BaseURL != DefaultBaseURL {
		t.Errorf("default BaseURL = %q, want %q", c.BaseURL, DefaultBaseURL)
	}
	if c.APIKey != "" {
		t.Errorf("APIKey should be empty, got %q", c.APIKey)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c := &Config{APIKey: "m0sk_secret", BaseURL: "http://mem0:8888", UserID: "alice"}
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != "m0sk_secret" || got.BaseURL != "http://mem0:8888" || got.UserID != "alice" {
		t.Errorf("round trip mismatch: %+v", got)
	}

	// Config file must not be world-readable (it holds a key).
	fi, err := os.Stat(filepath.Join(home, ".mem0", "config.json"))
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perm = %o, want 600", perm)
	}
}

func TestApplyEnv(t *testing.T) {
	t.Setenv("MEM0_API_KEY", "env_key")
	t.Setenv("MEM0_BASE_URL", "https://env.example")
	t.Setenv("MEM0_USER_ID", "env_user")

	c := &Config{APIKey: "file_key", BaseURL: "http://file"}
	c.ApplyEnv()

	if c.APIKey != "env_key" {
		t.Errorf("APIKey = %q, want env value", c.APIKey)
	}
	if c.BaseURL != "https://env.example" {
		t.Errorf("BaseURL = %q, want env value", c.BaseURL)
	}
	if c.UserID != "env_user" {
		t.Errorf("UserID = %q, want env value", c.UserID)
	}
}

func TestApplyEnvDefaultUserFallback(t *testing.T) {
	t.Setenv("MEM0_DEFAULT_USER_ID", "fallback")
	c := &Config{}
	c.ApplyEnv()
	if c.UserID != "fallback" {
		t.Errorf("UserID = %q, want MEM0_DEFAULT_USER_ID fallback", c.UserID)
	}
}

func TestRedactedAndMask(t *testing.T) {
	if got := MaskSecret("m0sk_abcdefghijkl"); got == "m0sk_abcdefghijkl" {
		t.Errorf("MaskSecret returned the full secret: %q", got)
	}
	if got := MaskSecret("m0sk_abcdefghijkl"); !strings.HasPrefix(got, "m0sk") || !strings.HasSuffix(got, "ijkl") {
		t.Errorf("MaskSecret = %q, want prefix+suffix visible", got)
	}
	if got := MaskSecret("short"); got != "********" {
		t.Errorf("MaskSecret(short) = %q, want ********", got)
	}

	c := &Config{APIKey: "m0sk_secretvalue"}
	r := c.Redacted()
	if r.APIKey == c.APIKey {
		t.Errorf("Redacted() leaked the key")
	}
	if c.APIKey != "m0sk_secretvalue" {
		t.Errorf("Redacted() mutated the original")
	}
}
