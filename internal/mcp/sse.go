package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	if strings.TrimSpace(c.TLSCertFile) == "" {
		return fmt.Errorf("AEGIS_SSE_TLS_CERT_FILE is required for SSE transport")
	}
	if strings.TrimSpace(c.TLSKeyFile) == "" {
		return fmt.Errorf("AEGIS_SSE_TLS_KEY_FILE is required for SSE transport")
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

// StartSSE serves MCP over HTTPS using SSE with bearer-token-based tool filtering.
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

	sseServer := mcpserver.NewSSEServer(
		a.mcpSrv,
		mcpserver.WithBaseURL(cfg.BaseURL),
		mcpserver.WithBasePath(cfg.BasePath),
	)

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           a.wrapSSEAuth(sseServer),
		ReadHeaderTimeout: 10 * time.Second,
	}

	a.mu.Lock()
	a.httpSrv = httpSrv
	a.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[AEGIS] HTTPS SSE listening at %s%s/sse\n", strings.TrimRight(cfg.BaseURL, "/"), cfg.BasePath)
	go a.runWatcher()

	err := httpSrv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *AegisServer) wrapSSEAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Vary", "Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ctx, err := a.authenticatedContextForRequest(r)
		if err != nil {
			setBearerChallenge(w, err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AegisServer) authenticatedContextForRequest(r *http.Request) (context.Context, error) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId"))
	if sessionID != "" {
		a.mu.RLock()
		access, ok := a.sessionAccess[sessionID]
		a.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("invalid session")
		}

		if key := extractAPIKeyFromRequest(r); key != "" && key != access.APIKey {
			return nil, fmt.Errorf("invalid bearer token for session")
		}

		aliases, ok := a.aliasesForAPIKey(access.APIKey)
		if !ok {
			return nil, fmt.Errorf("bearer token is no longer authorized")
		}

		return withAccessContext(r.Context(), access.APIKey, aliases), nil
	}

	key := extractAPIKeyFromRequest(r)
	if key == "" {
		return nil, fmt.Errorf("missing bearer token")
	}

	aliases, ok := a.aliasesForAPIKey(key)
	if !ok {
		return nil, fmt.Errorf("invalid bearer token")
	}
	return withAccessContext(r.Context(), key, aliases), nil
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
