package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// SSEConfig defines the HTTPS SSE listener settings.
type SSEConfig struct {
	Addr        string
	BaseURL     string
	BasePath    string
	TLSCertFile string
	TLSKeyFile  string
	DisableTLS  bool
}

func (c SSEConfig) normalized() SSEConfig {
	cfg := c
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = ":8443"
	}
	if strings.TrimSpace(cfg.BasePath) == "" {
		cfg.BasePath = "/mcp"
	}
	return cfg
}

func (c SSEConfig) validate() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("AEGIS_SSE_BASE_URL is required for SSE transport")
	}
	parsedBaseURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("parse AEGIS_SSE_BASE_URL: %w", err)
	}
	if c.DisableTLS {
		if parsedBaseURL.Scheme != "http" {
			return fmt.Errorf("AEGIS_SSE_BASE_URL must use http:// when AEGIS_SSE_DISABLE_TLS=true")
		}
		return nil
	}
	if parsedBaseURL.Scheme != "https" {
		return fmt.Errorf("AEGIS_SSE_BASE_URL must use https:// when TLS is enabled")
	}
	if strings.TrimSpace(c.TLSCertFile) == "" {
		return fmt.Errorf("AEGIS_SSE_TLS_CERT_FILE is required for SSE transport unless AEGIS_SSE_DISABLE_TLS=true")
	}
	if strings.TrimSpace(c.TLSKeyFile) == "" {
		return fmt.Errorf("AEGIS_SSE_TLS_KEY_FILE is required for SSE transport unless AEGIS_SSE_DISABLE_TLS=true")
	}
	return nil
}

// Start keeps the stdio transport as the backward-compatible default.
func (a *AegisServer) Start() error {
	return a.StartStdio()
}

// StartStdio serves MCP over stdio.
func (a *AegisServer) StartStdio() error {
	go a.runWatcher()
	return mcpserver.ServeStdio(a.mcpSrv)
}

// StartSSE serves MCP over HTTPS, or HTTP when explicitly configured, using one host-scoped endpoint per config file.
func (a *AegisServer) StartSSE(cfg SSEConfig) error {
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return err
	}

	a.mu.RLock()
	hasKeys := len(a.apiKeyAliases) > 0
	a.mu.RUnlock()
	if !hasKeys {
		return fmt.Errorf("SSE transport requires at least one api_keys entry in the host configs")
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           a.wrapSSEAuth(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	a.mu.Lock()
	a.httpSrv = httpSrv
	a.sseCfg = &cfg
	a.mu.Unlock()

	schemeLabel := "HTTPS"
	listen := func() error {
		return httpSrv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
	}
	if cfg.DisableTLS {
		schemeLabel = "HTTP"
		listen = httpSrv.ListenAndServe
		fmt.Fprintln(os.Stderr, "[AEGIS] WARNING: TLS is disabled for SSE transport. Use only in trusted local or lab environments.")
	}

	fmt.Fprintf(os.Stderr, "[AEGIS] %s SSE listening at %s%s/<host-alias>/sse\n", schemeLabel, strings.TrimRight(cfg.BaseURL, "/"), cfg.BasePath)
	go a.runWatcher()

	err := listen()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *AegisServer) wrapSSEAuth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Vary", "Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		alias, next, ok := a.handlerForRequestPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		ctx, err := a.authenticatedContextForRequest(r, alias)
		if err != nil {
			setBearerChallenge(w, err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AegisServer) authenticatedContextForRequest(r *http.Request, requestedAlias string) (context.Context, error) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sessionID != "" {
		a.mu.RLock()
		access, ok := a.sessionAccess[sessionID]
		a.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("invalid session")
		}

		if access.Alias != requestedAlias {
			return nil, fmt.Errorf("session is not authorized for this endpoint")
		}

		if token := extractAPIKeyFromRequest(r); token != "" && token != access.Token {
			return nil, fmt.Errorf("invalid bearer token for session")
		}

		alias, ok := a.aliasForAPIKey(access.Token)
		if !ok {
			return nil, fmt.Errorf("bearer token is no longer authorized")
		}
		if alias != requestedAlias {
			return nil, fmt.Errorf("bearer token is not authorized for this endpoint")
		}

		return withAccessContext(r.Context(), access.Token, alias), nil
	}

	token := extractAPIKeyFromRequest(r)
	if token == "" {
		return nil, fmt.Errorf("missing bearer token")
	}

	alias, ok := a.aliasForAPIKey(token)
	if !ok {
		return nil, fmt.Errorf("invalid bearer token")
	}
	if alias != requestedAlias {
		return nil, fmt.Errorf("bearer token is not authorized for this endpoint")
	}

	return withAccessContext(r.Context(), token, alias), nil
}

func (a *AegisServer) handlerForRequestPath(path string) (string, http.Handler, bool) {
	a.mu.RLock()
	cfg := a.sseCfg
	a.mu.RUnlock()
	if cfg == nil {
		return "", nil, false
	}

	basePath := strings.TrimRight(strings.TrimSpace(cfg.BasePath), "/")
	if basePath == "" {
		basePath = "/mcp"
	}
	if !strings.HasPrefix(path, basePath+"/") {
		return "", nil, false
	}

	remainder := strings.TrimPrefix(path, basePath+"/")
	parts := strings.Split(strings.Trim(remainder, "/"), "/")
	if len(parts) != 2 {
		return "", nil, false
	}
	if parts[1] != "sse" && parts[1] != "message" {
		return "", nil, false
	}

	a.mu.RLock()
	alias, ok := a.pathAliases[parts[0]]
	a.mu.RUnlock()
	if !ok {
		return "", nil, false
	}

	return alias, a.sseHandlerForAlias(alias), true
}

func (a *AegisServer) sseHandlerForAlias(alias string) http.Handler {
	a.mu.RLock()
	if handler, exists := a.sseServers[alias]; exists {
		a.mu.RUnlock()
		return handler
	}
	cfg := a.sseCfg
	a.mu.RUnlock()

	basePath := endpointBasePath(cfg.BasePath, alias)
	handler := mcpserver.NewSSEServer(
		a.mcpSrv,
		mcpserver.WithBaseURL(cfg.BaseURL),
		mcpserver.WithBasePath(basePath),
	)

	a.mu.Lock()
	defer a.mu.Unlock()
	if existing, exists := a.sseServers[alias]; exists {
		return existing
	}
	a.sseServers[alias] = handler
	return handler
}

func setBearerChallenge(w http.ResponseWriter, err error) {
	errorCode := "invalid_token"
	if strings.Contains(strings.ToLower(err.Error()), "missing") {
		errorCode = "invalid_request"
	}

	w.Header().Set(
		"WWW-Authenticate",
		fmt.Sprintf(`Bearer realm="aegis-ssh-mcp", error="%s"`, errorCode),
	)
}
