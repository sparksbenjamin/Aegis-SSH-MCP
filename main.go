package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"aegis-ssh-mcp/internal/audit"
	aegismcp "aegis-ssh-mcp/internal/mcp"
)

const banner = `
=============================================
           Aegis-SSH-MCP v1.0.0
   Secure MCP Gateway for SSH Infrastructure
=============================================
`

func main() {
	fmt.Fprint(os.Stderr, banner)

	logger := audit.New()
	configsDir := resolveDir("AEGIS_CONFIGS_DIR", "configs", "/configs")
	rulesDir := resolveDir("AEGIS_RULES_DIR", "rules", "/rules")

	srv, err := aegismcp.NewAegisServer(configsDir, rulesDir, logger)
	if err != nil {
		log.Fatalf("[AEGIS] Fatal: failed to initialize server: %v", err)
	}

	transport := strings.ToLower(strings.TrimSpace(os.Getenv("AEGIS_TRANSPORT")))
	if transport == "" {
		transport = "stdio"
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\n[AEGIS] Received signal %s - shutting down.\n", sig)
		srv.Stop()
		if transport == "stdio" {
			os.Exit(0)
		}
	}()

	fmt.Fprintf(
		os.Stderr,
		"[AEGIS] Server starting - transport: %s | configs: %s | rules: %s\n",
		transport,
		configsDir,
		rulesDir,
	)

	err = startServer(srv, transport)
	if err != nil {
		log.Fatalf("[AEGIS] Server exited with error: %v", err)
	}
}

func startServer(srv *aegismcp.AegisServer, transport string) error {
	switch transport {
	case "stdio":
		return srv.StartStdio()
	case "sse":
		return srv.StartSSE(aegismcp.SSEConfig{
			Addr:        envOrDefault("AEGIS_SSE_ADDR", ":8443"),
			BaseURL:     strings.TrimSpace(os.Getenv("AEGIS_SSE_BASE_URL")),
			BasePath:    envOrDefault("AEGIS_SSE_BASE_PATH", "/mcp"),
			TLSCertFile: strings.TrimSpace(os.Getenv("AEGIS_SSE_TLS_CERT_FILE")),
			TLSKeyFile:  strings.TrimSpace(os.Getenv("AEGIS_SSE_TLS_KEY_FILE")),
		})
	default:
		return fmt.Errorf("unsupported AEGIS_TRANSPORT %q", transport)
	}
}

func resolveDir(key, localFallback, containerFallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if info, err := os.Stat(localFallback); err == nil && info.IsDir() {
		return localFallback
	}
	return containerFallback
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
