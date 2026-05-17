package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/fsnotify/fsnotify"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"aegis-ssh-mcp/internal/audit"
	"aegis-ssh-mcp/internal/command"
	"aegis-ssh-mcp/internal/config"
	"aegis-ssh-mcp/internal/rules"
	sshexec "aegis-ssh-mcp/internal/ssh"
)

// AegisServer owns the MCP server, the rule engine, the config registry,
// and the file-system watcher.
type AegisServer struct {
	mcpSrv     *server.MCPServer
	ruleEngine *rules.Engine
	logger     *audit.Logger

	watcher    *fsnotify.Watcher
	configsDir string
	rulesDir   string

	mu            sync.RWMutex
	configs       map[string]*config.HostConfig
	apiKeyAliases map[string]map[string]struct{}
	sessionAccess map[string]accessContext
	httpSrv       *http.Server
	stopCh        chan struct{}
	stopOnce      sync.Once
}

// NewAegisServer builds the full server stack and loads the initial config state.
func NewAegisServer(configsDir, rulesDir string, logger *audit.Logger) (*AegisServer, error) {
	ruleEngine, err := rules.NewEngine(rulesDir)
	if err != nil {
		return nil, fmt.Errorf("rule engine init: %w", err)
	}

	hooks := &server.Hooks{}
	mcpSrv := server.NewMCPServer("Aegis-SSH-MCP", "1.0.0", server.WithHooks(hooks))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	for _, dir := range []string{configsDir, rulesDir} {
		if err := watcher.Add(dir); err != nil {
			watcher.Close()
			return nil, fmt.Errorf("watch %s: %w", dir, err)
		}
	}

	a := &AegisServer{
		mcpSrv:        mcpSrv,
		ruleEngine:    ruleEngine,
		logger:        logger,
		watcher:       watcher,
		configsDir:    configsDir,
		rulesDir:      rulesDir,
		configs:       make(map[string]*config.HostConfig),
		apiKeyAliases: make(map[string]map[string]struct{}),
		sessionAccess: make(map[string]accessContext),
		stopCh:        make(chan struct{}),
	}

	hooks.AddAfterListTools(func(ctx context.Context, _ any, _ *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
		if result == nil {
			return
		}

		filtered := make([]mcp.Tool, 0, len(result.Tools))
		for _, tool := range result.Tools {
			if toolVisibleForContext(ctx, tool.Name) {
				filtered = append(filtered, tool)
			}
		}
		result.Tools = filtered
	})

	hooks.AddOnRegisterSession(func(ctx context.Context, session server.ClientSession) {
		access, ok := accessFromContext(ctx)
		if !ok {
			return
		}

		a.mu.Lock()
		a.sessionAccess[session.SessionID()] = access
		a.mu.Unlock()

		go func() {
			<-ctx.Done()
			a.mu.Lock()
			delete(a.sessionAccess, session.SessionID())
			a.mu.Unlock()
		}()
	})

	a.registerIntrospectionTool()

	if err := a.syncConfigs(); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("sync configs: %w", err)
	}

	a.mu.RLock()
	hostCount := len(a.configs)
	a.mu.RUnlock()

	fmt.Fprintf(
		os.Stderr,
		"[AEGIS] Registered %d host tool(s). Active rule profiles: %v\n",
		hostCount,
		ruleEngine.ProfileNames(),
	)

	return a, nil
}

// Stop signals the watcher goroutine to exit and closes the watcher.
func (a *AegisServer) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
		a.mu.RLock()
		httpSrv := a.httpSrv
		a.mu.RUnlock()
		if httpSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(ctx)
		}
		_ = a.watcher.Close()
	})
}

// registerHostTool adds an aegis_ssh_<alias> tool to the MCP server and stores
// the config in the live registry.
func (a *AegisServer) registerHostTool(cfg *config.HostConfig) {
	toolName := "aegis_ssh_" + sanitizeAlias(cfg.Alias)

	a.mu.Lock()
	_, exists := a.configs[cfg.Alias]
	a.configs[cfg.Alias] = cfg
	a.mu.Unlock()

	if exists {
		fmt.Fprintf(os.Stderr, "[AEGIS] Tool updated: %s\n", toolName)
		return
	}

	tool := mcp.NewTool(
		toolName,
		mcp.WithDescription(fmt.Sprintf(
			"[Aegis] Execute a single command on host %q (%s@%s:%d). Rule profile: %q. Commands are validated before execution.",
			cfg.Alias,
			cfg.SSHUser,
			cfg.HostIP,
			cfg.SSHPort,
			cfg.RuleProfile,
		)),
		mcp.WithString(
			"command",
			mcp.Required(),
			mcp.Description(
				"The command to execute on the remote host. Aegis parses it into argv, validates the executable and arguments, then executes a normalized shell-safe form. Shell chaining and expansion features are intentionally blocked or neutralized.",
			),
		),
	)

	a.mcpSrv.AddTool(tool, a.makeToolHandler(cfg.Alias))
	fmt.Fprintf(os.Stderr, "[AEGIS] Tool registered: %s\n", toolName)
}

// syncConfigs rescans the configs directory and applies any adds, updates,
// or removals to the in-memory registry.
func (a *AegisServer) syncConfigs() error {
	cfgs, err := config.ScanConfigDir(a.configsDir)
	if err != nil {
		return err
	}

	apiKeyAliases := buildAPIKeyIndex(cfgs)
	desired := make(map[string]*config.HostConfig, len(cfgs))
	for _, cfg := range cfgs {
		desired[cfg.Alias] = cfg
		a.registerHostTool(cfg)
	}

	removed := make([]string, 0)
	a.mu.Lock()
	for alias := range a.configs {
		if _, keep := desired[alias]; keep {
			continue
		}
		delete(a.configs, alias)
		removed = append(removed, alias)
	}
	a.apiKeyAliases = apiKeyAliases
	a.mu.Unlock()

	for _, alias := range removed {
		a.logger.System(
			"remove_config:"+alias,
			"SYSTEM",
			"config removed during resync - tool will return error on next call",
		)
	}

	return nil
}

func (a *AegisServer) reloadRules() {
	if err := a.ruleEngine.LoadAll(); err != nil {
		a.logger.System("reload_rules", "ERROR", err.Error())
		return
	}

	a.logger.System(
		"reload_rules",
		"SYSTEM",
		fmt.Sprintf("active profiles: %s", strings.Join(a.ruleEngine.ProfileNames(), ", ")),
	)
}

// registerIntrospectionTool adds an aegis_status tool for health checks and debugging.
func (a *AegisServer) registerIntrospectionTool() {
	tool := mcp.NewTool(
		"aegis_status",
		mcp.WithDescription(
			"[Aegis] Returns the current status of the Aegis-SSH-MCP gateway: number of registered hosts, active rule profiles, and server uptime. No SSH connection is made.",
		),
	)

	start := time.Now()
	a.mcpSrv.AddTool(tool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a.mu.RLock()
		aliases := make([]string, 0, len(a.configs))
		for alias := range a.configs {
			aliases = append(aliases, alias)
		}
		a.mu.RUnlock()

		visibleAliases := visibleAliasesForContext(ctx, aliases)
		visibleLabel := strings.Join(visibleAliases, ", ")
		if visibleLabel == "" {
			visibleLabel = "none"
		}

		msg := fmt.Sprintf(
			"Aegis-SSH-MCP v1.0.0\n"+
				"Uptime        : %s\n"+
				"Hosts         : %d (%s)\n"+
				"Rule profiles : %v\n",
			time.Since(start).Round(time.Second),
			len(visibleAliases),
			visibleLabel,
			a.ruleEngine.ProfileNames(),
		)
		return mcp.NewToolResultText(msg), nil
	})
}

// makeToolHandler returns the ToolHandlerFunc for a specific host alias.
func (a *AegisServer) makeToolHandler(alias string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		rawCommand, _ := req.Params.Arguments["command"].(string)
		rawCommand = strings.TrimSpace(rawCommand)
		if rawCommand == "" {
			return mcp.NewToolResultError("'command' parameter must be a non-empty string"), nil
		}

		if !aliasAllowedForContext(ctx, alias) {
			logEntry := audit.Entry{
				AgentAlias:       alias,
				CommandRequested: rawCommand,
				ValidationResult: "FAIL",
				ValidationReason: "API key is not authorized for this host",
				DurationMs:       time.Since(start).Milliseconds(),
			}
			a.logger.Log(logEntry)
			return mcp.NewToolResultError(
				fmt.Sprintf("AEGIS BLOCKED - API key is not authorized for host %q", alias),
			), nil
		}

		parsed, err := command.Parse(rawCommand)
		if err != nil {
			logEntry := audit.Entry{
				AgentAlias:       alias,
				CommandRequested: rawCommand,
				ValidationResult: "FAIL",
				ValidationReason: err.Error(),
				DurationMs:       time.Since(start).Milliseconds(),
			}
			a.logger.Log(logEntry)

			return mcp.NewToolResultError("AEGIS BLOCKED - " + err.Error()), nil
		}

		a.mu.RLock()
		cfg, exists := a.configs[alias]
		a.mu.RUnlock()
		if !exists {
			return mcp.NewToolResultError(
				fmt.Sprintf("host %q has been removed from the Aegis registry", alias),
			), nil
		}

		validation := a.ruleEngine.Validate(cfg.RuleProfile, parsed)
		logEntry := audit.Entry{
			AgentAlias:       alias,
			CommandRequested: rawCommand,
			ValidationResult: "PASS",
			ValidationReason: validation.Reason,
		}

		if !validation.Passed {
			logEntry.ValidationResult = "FAIL"
			logEntry.DurationMs = time.Since(start).Milliseconds()
			a.logger.Log(logEntry)

			if cfg.StealthMode {
				logEntry.StealthMode = true
				fakeResp := cfg.FakeResponse
				if fakeResp == "" {
					fakeResp = "bash: command not found"
				}
				return mcp.NewToolResultText(fakeResp), nil
			}

			return mcp.NewToolResultError("AEGIS BLOCKED - " + validation.Reason), nil
		}

		result, err := sshexec.Execute(cfg, parsed.Normalized)
		if err != nil {
			logEntry.ValidationResult = "EXEC_ERROR"
			logEntry.ValidationReason = err.Error()
			logEntry.DurationMs = time.Since(start).Milliseconds()
			a.logger.Log(logEntry)
			return mcp.NewToolResultError("SSH execution error: " + err.Error()), nil
		}

		summary := result.Stdout
		if len(summary) > 200 {
			summary = summary[:200] + " ...[truncated]"
		}
		logEntry.OutputSummary = summary
		logEntry.ExitCode = result.ExitCode
		logEntry.DurationMs = time.Since(start).Milliseconds()
		a.logger.Log(logEntry)

		return mcp.NewToolResultText(formatOutput(result)), nil
	}
}

func (a *AegisServer) aliasesForAPIKey(key string) (map[string]struct{}, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	aliases, ok := a.apiKeyAliases[key]
	if !ok {
		return nil, false
	}
	return cloneAliasSet(aliases), true
}

// runWatcher processes fsnotify events and reacts to config and rule changes.
func (a *AegisServer) runWatcher() {
	for {
		select {
		case <-a.stopCh:
			return
		case event, ok := <-a.watcher.Events:
			if !ok {
				return
			}
			a.handleFSEvent(event)
		case err, ok := <-a.watcher.Errors:
			if !ok {
				return
			}
			a.logger.System("fsnotify", "ERROR", err.Error())
		}
	}
}

func (a *AegisServer) handleFSEvent(event fsnotify.Event) {
	if filepath.Ext(event.Name) != ".json" {
		return
	}

	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
		time.Sleep(50 * time.Millisecond)
	}

	switch filepath.Clean(filepath.Dir(event.Name)) {
	case filepath.Clean(a.configsDir):
		if err := a.syncConfigs(); err != nil {
			a.logger.System("sync_configs", "ERROR", err.Error())
		}
	case filepath.Clean(a.rulesDir):
		a.reloadRules()
	}
}

// sanitizeAlias converts an alias into a valid MCP tool-name suffix by
// lowercasing and replacing any character that isn't [a-z0-9_-] with '_'.
func sanitizeAlias(alias string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(alias) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func toolNameForAlias(alias string) string {
	return "aegis_ssh_" + sanitizeAlias(alias)
}

func toolVisibleForContext(ctx context.Context, toolName string) bool {
	if toolName == "aegis_status" {
		return true
	}

	access, ok := accessFromContext(ctx)
	if !ok {
		return true
	}

	for alias := range access.AllowedAliases {
		if toolName == toolNameForAlias(alias) {
			return true
		}
	}
	return false
}

// formatOutput assembles a human-readable result string from a command result.
func formatOutput(r *sshexec.Result) string {
	var sb strings.Builder

	if r.Stdout != "" {
		sb.WriteString(r.Stdout)
	}
	if r.Stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[stderr]\n")
		sb.WriteString(r.Stderr)
	}
	if r.ExitCode != 0 {
		sb.WriteString(fmt.Sprintf("\n[exit code: %d]", r.ExitCode))
	}
	if sb.Len() == 0 {
		return "(no output)"
	}
	return sb.String()
}
