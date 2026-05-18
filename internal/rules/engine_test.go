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

func TestValidateAllowsPipelinesThroughSafeTextFilters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "systemd.json"), []byte(`{
  "profile_name": "systemd-ops",
  "executable_whitelist_regex": ["^systemctl$"],
  "whitelist_regex": ["^systemctl\\s+list-units(\\s|$)"]
}`), 0o600)
	if err != nil {
		t.Fatalf("write rule file: %v", err)
	}

	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	allowedCmd, err := command.Parse(`systemctl list-units --type=service --state=running | grep -iE 'media|server|streaming|jellyfin|plex'`)
	if err != nil {
		t.Fatalf("parse allowed pipeline: %v", err)
	}
	if result := engine.Validate("systemd-ops", allowedCmd); !result.Passed {
		t.Fatalf("expected allowed pipeline to pass, got %q", result.Reason)
	}

	blockedCmd, err := command.Parse(`systemctl list-units --type=service --state=running | bash`)
	if err != nil {
		t.Fatalf("parse blocked pipeline: %v", err)
	}
	if result := engine.Validate("systemd-ops", blockedCmd); result.Passed {
		t.Fatal("expected disallowed pipeline filter to fail")
	}
}

func TestBundledProfilesLoadFromRepoRulesDir(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(filepath.Join("..", "..", "rules"))
	if err != nil {
		t.Fatalf("load bundled profiles: %v", err)
	}

	want := []string{
		"debian-ops",
		"debian-readonly",
		"docker-ops",
		"docker-readonly",
		"kubernetes-readonly",
		"logs-readonly",
		"network-diagnostics",
		"package-readonly",
		"readonly-safe",
		"rhel-ops",
		"rhel-readonly",
		"systemd-ops",
		"ubuntu-ops",
		"ubuntu-readonly",
	}

	if got := engine.ProfileNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected bundled profiles: got %v want %v", got, want)
	}
}

func TestBundledDistroProfilesRepresentativeCommands(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(filepath.Join("..", "..", "rules"))
	if err != nil {
		t.Fatalf("load bundled profiles: %v", err)
	}

	cases := []struct {
		name    string
		profile string
		command string
		allowed bool
	}{
		{name: "debian readonly apt list", profile: "debian-readonly", command: "apt list --installed", allowed: true},
		{name: "debian readonly journalctl read", profile: "debian-readonly", command: "journalctl -u nginx -n 50", allowed: true},
		{name: "debian readonly blocks journalctl follow", profile: "debian-readonly", command: "journalctl -f", allowed: false},
		{name: "debian readonly blocks install", profile: "debian-readonly", command: "apt install nginx", allowed: false},
		{name: "debian ops allows restart", profile: "debian-ops", command: "systemctl restart nginx", allowed: true},
		{name: "debian ops allows journalctl read", profile: "debian-ops", command: "journalctl -u nginx -n 100", allowed: true},
		{name: "ubuntu readonly allows snap list", profile: "ubuntu-readonly", command: "snap list", allowed: true},
		{name: "ubuntu ops allows snap restart", profile: "ubuntu-ops", command: "snap restart lxd", allowed: true},
		{name: "rhel readonly allows dnf repolist", profile: "rhel-readonly", command: "dnf repolist", allowed: true},
		{name: "rhel readonly blocks install", profile: "rhel-readonly", command: "dnf install nginx", allowed: false},
		{name: "rhel ops allows service restart", profile: "rhel-ops", command: "systemctl restart sshd", allowed: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := command.Parse(tc.command)
			if err != nil {
				t.Fatalf("parse command %q: %v", tc.command, err)
			}

			got := engine.Validate(tc.profile, parsed)
			if got.Passed != tc.allowed {
				t.Fatalf("profile %s command %q: got passed=%v reason=%q want %v", tc.profile, tc.command, got.Passed, got.Reason, tc.allowed)
			}
		})
	}
}
