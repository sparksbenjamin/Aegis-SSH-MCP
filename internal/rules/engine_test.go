package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateUsesBlacklistBeforeWhitelist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "readonly.json"), []byte(`{
  "profile_name": "readonly-safe",
  "whitelist_regex": ["^ls(\\s|$)"],
  "blacklist_regex": ["rm\\s"]
}`), 0o600)
	if err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	blocked := engine.Validate("readonly-safe", "rm -rf /tmp/demo")
	if blocked.Passed {
		t.Fatal("expected blacklist to block command")
	}

	allowed := engine.Validate("readonly-safe", "ls -la")
	if !allowed.Passed {
		t.Fatalf("expected whitelist to allow command, got %q", allowed.Reason)
	}
}

func TestLoadAllReplacesRemovedProfiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "readonly.json")
	err := os.WriteFile(path, []byte(`{
  "profile_name": "readonly-safe",
  "whitelist_regex": ["^ls(\\s|$)"],
  "blacklist_regex": []
}`), 0o600)
	if err != nil {
		t.Fatalf("write initial rule file: %v", err)
	}

	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove rule file: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("reload engine: %v", err)
	}

	got := engine.ProfileNames()
	if len(got) != 0 {
		t.Fatalf("expected no loaded profiles after removal, got %v", got)
	}
}
