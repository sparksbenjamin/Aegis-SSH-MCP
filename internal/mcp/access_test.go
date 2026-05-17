package mcp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"aegis-ssh-mcp/internal/config"
)

func TestBuildHostAccessStateBuildsSingleHostTokenMap(t *testing.T) {
	t.Parallel()

	cfgs := []*config.HostConfig{
		{Alias: "proxmox-node", APIKeys: []string{"proxmox-token"}},
		{Alias: "dell-r820", APIKeys: []string{"dell-token"}},
	}

	tokenAliases, pathAliases, err := buildHostAccessState(cfgs)
	if err != nil {
		t.Fatalf("build host access state: %v", err)
	}

	if got := tokenAliases["proxmox-token"]; got != "proxmox-node" {
		t.Fatalf("expected proxmox-token to map to proxmox-node, got %q", got)
	}
	if got := pathAliases["dell-r820"]; got != "dell-r820" {
		t.Fatalf("expected endpoint alias to map to dell-r820, got %q", got)
	}
}

func TestBuildHostAccessStateRejectsSharedTokenAcrossHosts(t *testing.T) {
	t.Parallel()

	cfgs := []*config.HostConfig{
		{Alias: "proxmox-node", APIKeys: []string{"shared-token"}},
		{Alias: "dell-r820", APIKeys: []string{"shared-token"}},
	}

	_, _, err := buildHostAccessState(cfgs)
	if err == nil {
		t.Fatal("expected shared token error")
	}
	if !strings.Contains(err.Error(), `bearer token "shared-token" is assigned to both "proxmox-node" and "dell-r820"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildHostAccessStateRejectsEndpointAliasCollisions(t *testing.T) {
	t.Parallel()

	cfgs := []*config.HostConfig{
		{Alias: "prod_box", APIKeys: []string{"prod-token"}},
		{Alias: "prod box", APIKeys: []string{"prod-space-token"}},
	}

	_, _, err := buildHostAccessState(cfgs)
	if err == nil {
		t.Fatal("expected endpoint alias collision error")
	}
	if !strings.Contains(err.Error(), `duplicate MCP endpoint alias "prod_box"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVisibleAliasesForContextFiltersAccess(t *testing.T) {
	t.Parallel()

	ctx := withAccessContext(context.Background(), "demo-token", "b")

	got := visibleAliasesForContext(ctx, []string{"c", "b", "a"})
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("unexpected visible aliases: %v", got)
	}
}

func TestExtractAPIKeyFromRequestSupportsBearerOnly(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "https://example.com/mcp/proxmox-node/sse", nil)
	req.Header.Set("Authorization", "Bearer bearer-key")
	if got := extractAPIKeyFromRequest(req); got != "bearer-key" {
		t.Fatalf("expected bearer key, got %q", got)
	}

	req = httptest.NewRequest("GET", "https://example.com/mcp/proxmox-node/sse?apiKey=query-key", nil)
	if got := extractAPIKeyFromRequest(req); got != "" {
		t.Fatalf("expected query key to be ignored, got %q", got)
	}

	req = httptest.NewRequest("GET", "https://example.com/mcp/proxmox-node/sse", nil)
	req.Header.Set("Authorization", "query-key")
	if got := extractAPIKeyFromRequest(req); got != "" {
		t.Fatalf("expected non-bearer auth header to be ignored, got %q", got)
	}
}

func TestEndpointBasePathScopesByAlias(t *testing.T) {
	t.Parallel()

	if got := endpointBasePath("/mcp", "proxmox-node"); got != "/mcp/proxmox-node" {
		t.Fatalf("unexpected endpoint base path: %q", got)
	}
}
