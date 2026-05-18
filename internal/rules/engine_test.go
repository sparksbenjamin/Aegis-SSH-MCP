package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aegis-ssh-mcp/internal/command"
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

	blockedCmd, err := command.Parse("rm -rf /tmp/demo")
	if err != nil {
		t.Fatalf("parse blocked command: %v", err)
	}
	blocked := engine.Validate("readonly-safe", blockedCmd)
	if blocked.Passed {
		t.Fatal("expected blacklist to block command")
	}

	allowedCmd, err := command.Parse("ls -la")
	if err != nil {
		t.Fatalf("parse allowed command: %v", err)
	}
	allowed := engine.Validate("readonly-safe", allowedCmd)
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

func TestValidateSupportsExecutableAndArgumentRules(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "docker.json"), []byte(`{
  "profile_name": "docker-readonly",
  "executable_whitelist_regex": ["^docker$"],
  "arguments_whitelist_regex": ["^(ps|logs)(\\s|$)"],
  "arguments_blacklist_regex": ["(^| )--privileged($| )"]
}`), 0o600)
	if err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	allowedCmd, err := command.Parse(`docker ps --format "{{.Names}}"`)
	if err != nil {
		t.Fatalf("parse allowed command: %v", err)
	}
	if result := engine.Validate("docker-readonly", allowedCmd); !result.Passed {
		t.Fatalf("expected allowed command to pass, got %q", result.Reason)
	}

	blockedExec, err := command.Parse("bash -lc whoami")
	if err != nil {
		t.Fatalf("parse blocked executable command: %v", err)
	}
	if result := engine.Validate("docker-readonly", blockedExec); result.Passed {
		t.Fatal("expected executable whitelist to block bash")
	}

	blockedArg, err := command.Parse("docker logs --privileged")
	if err != nil {
		t.Fatalf("parse blocked args command: %v", err)
	}
	if result := engine.Validate("docker-readonly", blockedArg); result.Passed {
		t.Fatal("expected arguments blacklist to block --privileged")
	}
}

func TestBundledProfilesLoadFromRepoRulesDir(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(filepath.Join("..", "..", "rules"))
	if err != nil {
		t.Fatalf("load bundled profiles: %v", err)
	}

	want := []string{
		"docker-ops",
		"docker-readonly",
		"kubernetes-readonly",
		"logs-readonly",
		"network-diagnostics",
		"package-readonly",
		"readonly-safe",
		"systemd-ops",
	}

	if got := engine.ProfileNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected bundled profiles: got %v want %v", got, want)
	}
}
