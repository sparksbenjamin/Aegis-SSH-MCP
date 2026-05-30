package mcp

import (
	"strings"
	"testing"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"

	"aegis-ssh-mcp/internal/config"
)

func TestExecutionConfigForDynamicRequestUsesHostArgument(t *testing.T) {
	t.Parallel()

	cfg := &config.HostConfig{
		ConfigType:  config.ConfigTypeDynamic,
		Alias:       "linux-readonly",
		SSHUser:     "ops",
		AuthMethod:  config.AuthMethodKey,
		KeyPath:     "/keys/ops.pem",
		RuleProfile: "readonly-safe",
	}
	req := mcpsdk.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"host": " 192.168.1.42 ",
	}

	execCfg, targetHost, err := executionConfigForRequest(cfg, req)
	if err != nil {
		t.Fatalf("build execution config: %v", err)
	}

	if targetHost != "192.168.1.42" {
		t.Fatalf("expected trimmed target host, got %q", targetHost)
	}
	if execCfg.HostIP != "192.168.1.42" {
		t.Fatalf("expected execution HostIP to be target host, got %q", execCfg.HostIP)
	}
	if cfg.HostIP != "" {
		t.Fatalf("expected stored dynamic config to remain unchanged, got %q", cfg.HostIP)
	}
}

func TestExecutionConfigForDynamicRequestRequiresHost(t *testing.T) {
	t.Parallel()

	cfg := &config.HostConfig{ConfigType: config.ConfigTypeDynamic}
	req := mcpsdk.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"host": " "}

	_, _, err := executionConfigForRequest(cfg, req)
	if err == nil {
		t.Fatal("expected missing host error")
	}
	if !strings.Contains(err.Error(), "'host' parameter must be a non-empty string") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecutionConfigForFixedHostIgnoresHostArgument(t *testing.T) {
	t.Parallel()

	cfg := &config.HostConfig{
		ConfigType: config.ConfigTypeHost,
		HostIP:     "192.168.1.10",
	}
	req := mcpsdk.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"host": "192.168.1.42",
	}

	execCfg, targetHost, err := executionConfigForRequest(cfg, req)
	if err != nil {
		t.Fatalf("build execution config: %v", err)
	}

	if targetHost != "192.168.1.10" {
		t.Fatalf("expected fixed target host, got %q", targetHost)
	}
	if execCfg.HostIP != "192.168.1.10" {
		t.Fatalf("expected fixed HostIP to remain, got %q", execCfg.HostIP)
	}
}
