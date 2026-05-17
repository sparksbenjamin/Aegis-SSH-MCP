package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"aegis-ssh-mcp/internal/config"
)

type accessContextKey struct{}

type accessContext struct {
	Token string
	Alias string
}

func buildHostAccessState(cfgs []*config.HostConfig) (map[string]string, map[string]string, error) {
	tokenAliases := make(map[string]string)
	pathAliases := make(map[string]string)

	for _, cfg := range cfgs {
		endpointAlias := sanitizeAlias(cfg.Alias)
		if previousAlias, exists := pathAliases[endpointAlias]; exists && previousAlias != cfg.Alias {
			return nil, nil, fmt.Errorf(
				"duplicate MCP endpoint alias %q from %q and %q",
				endpointAlias,
				previousAlias,
				cfg.Alias,
			)
		}
		pathAliases[endpointAlias] = cfg.Alias

		for _, token := range cfg.APIKeys {
			if previousAlias, exists := tokenAliases[token]; exists && previousAlias != cfg.Alias {
				return nil, nil, fmt.Errorf(
					"bearer token %q is assigned to both %q and %q",
					token,
					previousAlias,
					cfg.Alias,
				)
			}
			tokenAliases[token] = cfg.Alias
		}
	}

	return tokenAliases, pathAliases, nil
}

func withAccessContext(ctx context.Context, token, alias string) context.Context {
	return context.WithValue(ctx, accessContextKey{}, accessContext{
		Token: token,
		Alias: alias,
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

	visible := make([]string, 0, 1)
	for _, alias := range aliases {
		if alias == access.Alias {
			visible = append(visible, alias)
			break
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
	return access.Alias == alias
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

func endpointBasePath(basePath, alias string) string {
	base := strings.TrimRight(strings.TrimSpace(basePath), "/")
	if base == "" {
		base = "/mcp"
	}
	return base + "/" + sanitizeAlias(alias)
}
