package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\n[AEGIS] Received signal %s - shutting down.\n", sig)
		srv.Stop()
		os.Exit(0)
	}()

	fmt.Fprintf(os.Stderr, "[AEGIS] Server starting - configs: %s | rules: %s\n", configsDir, rulesDir)
	if err := srv.Start(); err != nil {
		log.Fatalf("[AEGIS] Server exited with error: %v", err)
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
