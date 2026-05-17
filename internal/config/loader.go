package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuthMethod is the SSH credential strategy for a host.
type AuthMethod string

const (
	AuthMethodKey      AuthMethod = "key"
	AuthMethodPassword AuthMethod = "password"
)

// HostConfig is the structure of a single JSON file in the configs directory.
type HostConfig struct {
	Alias   string `json:"alias"`
	HostIP  string `json:"host_ip"`
	SSHPort int    `json:"ssh_port"`
	SSHUser string `json:"ssh_user"`

	AuthMethod AuthMethod `json:"auth_method"`
	KeyPath    string     `json:"key_path"`
	Password   string     `json:"password"`

	RuleProfile string `json:"rule_profile"`

	TimeoutSeconds int `json:"timeout_seconds"`

	StealthMode  bool   `json:"stealth_mode"`
	FakeResponse string `json:"fake_response"`

	RedactionEnabled  bool     `json:"redaction_enabled"`
	RedactionPatterns []string `json:"redaction_patterns"`

	HostKeyFingerprint string `json:"host_key_fingerprint"`
}

// Validate normalizes defaults and returns an error if any required field is absent.
func (h *HostConfig) Validate() error {
	if strings.TrimSpace(h.Alias) == "" {
		return fmt.Errorf("alias is required")
	}
	if strings.TrimSpace(h.HostIP) == "" {
		return fmt.Errorf("host_ip is required")
	}
	if strings.TrimSpace(h.SSHUser) == "" {
		return fmt.Errorf("ssh_user is required")
	}
	if h.AuthMethod != AuthMethodKey && h.AuthMethod != AuthMethodPassword {
		return fmt.Errorf(
			"auth_method must be %q or %q, got %q",
			AuthMethodKey,
			AuthMethodPassword,
			h.AuthMethod,
		)
	}
	if h.AuthMethod == AuthMethodKey && strings.TrimSpace(h.KeyPath) == "" {
		return fmt.Errorf("key_path is required when auth_method is %q", AuthMethodKey)
	}
	if h.AuthMethod == AuthMethodPassword && h.Password == "" {
		return fmt.Errorf("password is required when auth_method is %q", AuthMethodPassword)
	}
	if strings.TrimSpace(h.RuleProfile) == "" {
		return fmt.Errorf("rule_profile is required")
	}
	if h.SSHPort == 0 {
		h.SSHPort = 22
	}
	if h.TimeoutSeconds == 0 {
		h.TimeoutSeconds = 30
	}

	return nil
}

// LoadHostConfig reads and validates a single JSON host config file.
func LoadHostConfig(path string) (*HostConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg HostConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation for %s: %w", path, err)
	}

	return &cfg, nil
}

// ScanConfigDir reads all JSON files from dir and returns parsed, validated host configs.
func ScanConfigDir(dir string) ([]*HostConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configs directory %q does not exist", dir)
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var cfgs []*HostConfig
	seenAliases := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		cfg, err := LoadHostConfig(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[AEGIS] WARNING: skipping %s: %v\n", entry.Name(), err)
			continue
		}
		if previous, exists := seenAliases[cfg.Alias]; exists {
			return nil, fmt.Errorf(
				"duplicate host alias %q in %s and %s",
				cfg.Alias,
				previous,
				path,
			)
		}

		seenAliases[cfg.Alias] = path
		cfgs = append(cfgs, cfg)
	}

	return cfgs, nil
}
