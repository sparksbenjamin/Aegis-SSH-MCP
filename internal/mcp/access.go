package mcp

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"aegis-ssh-mcp/internal/config"
)

type accessContextKey struct{}

type accessContext struct {
	APIKey         string
	AllowedAliases map[string]struct{}
}

func buildAPIKeyIndex(cfgs []*config.HostConfig) map[string]map[string]struct{} {
	index := make(map[string]map[string]struct{})
	for _, cfg := range cfgs {
		for _, key := range cfg.APIKeys {
			if _, exists := index[key]; !exists {
				index[key] = make(map[string]struct{})
			}
			index[key][cfg.Alias] = struct{}{}
		}
	}
	return index
}

func cloneAliasSet(in map[string]struct{}) map[string]struct{} {
	if len(in) == 0 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(in))
	for alias := range in {
		out[alias] = struct{}{}
	}
	return out
}

func withAccessContext(ctx context.Context, apiKey string, aliases map[string]struct{}) context.Context {
	return context.WithValue(ctx, accessContextKey{}, accessContext{
		APIKey:         apiKey,
		AllowedAliases: cloneAliasSet(aliases),
	})
}

func accessFromContext(ctx context.Context) (accessContext, bool) {
	access, ok := ctx.Value(accessContextKey{}).(accessContext)
	return access, ok
}

func visibleAliasesForContext(ctx context.Context, aliases []string) []string {
	access, ok := accessFromContext(ctx)
	if !ok {
		out := append([]string(nil), aliases...)
		sort.Strings(out)
		return out
	}

	visible := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		if _, allowed := access.AllowedAliases[alias]; allowed {
			visible = append(visible, alias)
		}
	}
	sort.Strings(visible)
	return visible
}

func aliasAllowedForContext(ctx context.Context, alias string) bool {
	access, ok := accessFromContext(ctx)
	if !ok {
		return true
	}
	_, allowed := access.AllowedAliases[alias]
	return allowed
}

func extractAPIKeyFromRequest(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}
