package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadHostConfigAppliesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "host.json")
	err := os.WriteFile(path, []byte(`{
  "alias": "test-host",
  "host_ip": "192.168.1.10",
  "ssh_user": "root",
  "auth_method": "key",
  "key_path": "/keys/test.pem",
  "rule_profile": "readonly-safe"
}`), 0o600)
	if err != nil {
		t.Fatalf("write host config: %v", err)
	}

	cfg, err := LoadHostConfig(path)
	if err != nil {
		t.Fatalf("load host config: %v", err)
	}

	if cfg.SSHPort != 22 {
		t.Fatalf("expected default SSH port 22, got %d", cfg.SSHPort)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Fatalf("expected default timeout 30, got %d", cfg.TimeoutSeconds)
	}
}

func TestScanConfigDirRejectsDuplicateAliases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "one.json")
	second := filepath.Join(dir, "two.json")

	for _, path := range []string{first, second} {
		err := os.WriteFile(path, []byte(`{
  "alias": "dup-host",
  "host_ip": "192.168.1.10",
  "ssh_user": "root",
  "auth_method": "key",
  "key_path": "/keys/test.pem",
  "rule_profile": "readonly-safe"
}`), 0o600)
		if err != nil {
			t.Fatalf("write host config %s: %v", path, err)
		}
	}

	_, err := ScanConfigDir(dir)
	if err == nil {
		t.Fatal("expected duplicate alias error")
	}
	if !strings.Contains(err.Error(), `duplicate host alias "dup-host"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
