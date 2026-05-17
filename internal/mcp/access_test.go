package mcp

import (
	"context"
	"net/http/httptest"
	"testing"

	"aegis-ssh-mcp/internal/config"
)

func TestBuildAPIKeyIndexAggregatesAliasesByKey(t *testing.T) {
	t.Parallel()

	cfgs := []*config.HostConfig{
		{Alias: "proxmox-node", APIKeys: []string{"shared-key", "ops-key"}},
		{Alias: "dell-r820", APIKeys: []string{"shared-key"}},
	}

	index := buildAPIKeyIndex(cfgs)

	if len(index["shared-key"]) != 2 {
		t.Fatalf("expected shared-key to authorize two aliases, got %d", len(index["shared-key"]))
	}
	if _, ok := index["ops-key"]["proxmox-node"]; !ok {
		t.Fatal("expected ops-key to authorize proxmox-node")
	}
}

func TestVisibleAliasesForContextFiltersAccess(t *testing.T) {
	t.Parallel()

	ctx := withAccessContext(context.Background(), "demo-key", map[string]struct{}{
		"b": {},
	})

	got := visibleAliasesForContext(ctx, []string{"c", "b", "a"})
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("unexpected visible aliases: %v", got)
	}
}

func TestExtractAPIKeyFromRequestSupportsQueryAndHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "https://example.com/mcp/sse?apiKey=query-key", nil)
	if got := extractAPIKeyFromRequest(req); got != "query-key" {
		t.Fatalf("expected query key, got %q", got)
	}

	req = httptest.NewRequest("GET", "https://example.com/mcp/sse", nil)
	req.Header.Set("Authorization", "Bearer bearer-key")
	if got := extractAPIKeyFromRequest(req); got != "bearer-key" {
		t.Fatalf("expected bearer key, got %q", got)
	}
}
