# Aegis-SSH-MCP Technical Notes

This document is the working technical handoff for Aegis-SSH-MCP.
It should be updated whenever architecture, behavior, deployment steps, or repo layout change.

## Purpose

Aegis-SSH-MCP is a local MCP server that exposes approved SSH actions on remote infrastructure to an AI client.
Its main job is to act as a command firewall:

1. Receive a tool call from an MCP client.
2. Map that tool call to a configured host.
3. Validate the requested command against a named rule profile.
4. Only if validation passes, open a single SSH session and run the command.
5. Return output to the caller and write an audit record to `stderr`.

## Current Repo Layout

```text
.
|-- .github/
|   `-- workflows/
|       `-- docker-publish.yml
|-- configs/
|   |-- dell-r820.json
|   `-- proxmox-node.json
|-- docs/
|   `-- tech-specs/
|       `-- aegis-ssh-mcp-tech-spec.md
|-- internal/
|   |-- audit/
|   |   `-- logger.go
|   |-- config/
|   |   |-- loader.go
|   |   `-- loader_test.go
|   |-- mcp/
|   |   `-- server.go
|   |-- rules/
|   |   |-- engine.go
|   |   `-- engine_test.go
|   `-- ssh/
|       `-- executor.go
|-- keys/
|   `-- .gitkeep
|-- rules/
|   |-- docker-ops.json
|   |-- docker-readonly.json
|   `-- readonly-safe.json
|-- .dockerignore
|-- .gitignore
|-- Dockerfile
|-- docker-compose.yml
|-- go.mod
|-- go.sum
`-- main.go
```

## Core Runtime Flow

### Startup

`main.go`:

1. Creates the audit logger.
2. Resolves config and rule directories.
3. Builds the Aegis server.
4. Starts stdio-based MCP serving.
5. Listens for `SIGINT` and `SIGTERM` for graceful shutdown.

Directory resolution behavior:

- Use `AEGIS_CONFIGS_DIR` or `AEGIS_RULES_DIR` if set.
- Otherwise prefer local `configs/` and `rules/` when running from the repo root.
- Otherwise fall back to `/configs` and `/rules` for container usage.

### MCP Layer

`internal/mcp/server.go` owns:

- MCP server creation
- dynamic tool registration
- config registry state
- file watching for config and rule changes
- the `aegis_status` tool

Tool naming:

- Host alias `proxmox-node` becomes `aegis_ssh_proxmox-node`
- Non-alphanumeric characters other than `_` and `-` are normalized to `_`

### Command Handling

Per host tool call:

1. Read the `command` argument.
2. Look up the live host config by alias.
3. Validate the command with the configured rule profile.
4. If blocked:
   - log a failed audit record
   - return a block message, or a fake response if stealth mode is enabled
5. If allowed:
   - execute a single SSH command
   - apply output redaction if enabled
   - log the result
   - return combined stdout/stderr text

## Package Responsibilities

### `internal/config`

Responsibilities:

- Parse host config JSON files
- Validate required fields
- Apply default `ssh_port` of `22`
- Apply default `timeout_seconds` of `30`
- Scan the `configs/` directory
- Reject duplicate host aliases

Important behavior:

- Bad config files are skipped with a warning during directory scans.
- Duplicate aliases fail the full scan because tool name collisions are unsafe.

### `internal/rules`

Responsibilities:

- Parse rule profile JSON files
- Compile whitelist and blacklist regexes
- Validate commands
- Reload the entire rule set from disk

Validation order:

1. Blacklist first
2. Whitelist second

Important behavior:

- `LoadAll()` rebuilds the in-memory rule set from disk instead of merging into the old state.
- Removing a rule file and reloading removes that profile from memory.
- Duplicate profile names fail the load.

### `internal/ssh`

Responsibilities:

- Build SSH client config
- Load key-based or password-based auth
- Enforce restrictive private key permissions
- Optionally verify host key fingerprints
- Run one non-interactive SSH command
- Capture stdout, stderr, and exit code
- Apply redaction before returning output

Important behavior:

- No interactive shell is opened.
- The implementation uses one `ssh.Session.Run()` call per request.
- If `host_key_fingerprint` is empty, insecure host-key verification is used.

### `internal/audit`

Responsibilities:

- Write structured JSON audit logs to `stderr`
- Serialize concurrent writes safely
- Emit both command and system events

Audit record fields include:

- timestamp
- agent_alias
- command_requested
- validation_result
- validation_reason
- output_summary
- exit_code
- duration_ms
- stealth_mode

## Config Model

Host config schema:

```json
{
  "alias": "my-server",
  "host_ip": "192.168.1.100",
  "ssh_port": 22,
  "ssh_user": "root",
  "auth_method": "key",
  "key_path": "/keys/my-server.pem",
  "password": "",
  "rule_profile": "readonly-safe",
  "timeout_seconds": 30,
  "stealth_mode": false,
  "fake_response": "",
  "redaction_enabled": false,
  "redaction_patterns": [],
  "host_key_fingerprint": ""
}
```

Operational notes:

- `alias` must be unique across all files in `configs/`
- `rule_profile` must match a profile in `rules/`
- `key_path` should reflect the runtime view of the filesystem
- Docker examples use `/keys/...`
- Local runs may also use local filesystem paths if desired

## Rule Model

Rule profile schema:

```json
{
  "profile_name": "readonly-safe",
  "whitelist_regex": ["^ls(\\s|$)"],
  "blacklist_regex": ["rm\\s"]
}
```

Recommended authoring guidance:

- Keep blacklist patterns broad enough to stop obvious escape hatches
- Keep whitelist patterns intentionally narrow
- Prefer exact command prefixes over permissive wildcard rules
- Treat rule edits as security changes, not simple config changes

## Hot Reload Behavior

Watched directories:

- `configs/`
- `rules/`

Behavior:

- Config changes trigger a full config rescan
- Rule changes trigger a full rule reload
- Updating an existing host config refreshes the live config without re-adding the MCP tool
- Removing a host config deletes it from the in-memory registry

Known limitation:

- MCP tool removal is limited by the upstream MCP server library
- If a host config is removed, the old tool name may still appear until the client refreshes its tool list
- Calls to that tool fail safely because the config no longer exists in memory

## Container Publishing Pipeline

Image registry:

- `ghcr.io/sparksbenjamin/aegis-ssh-mcp`

Workflow:

- `.github/workflows/docker-publish.yml`

Trigger behavior:

- Push to `main` publishes `latest`
- Push a tag like `v1.2.3` publishes that tag
- Every pushed build also gets a `sha-...` tag
- Pull requests build the image for validation but do not publish it

Workflow steps:

1. Check out the repository
2. Set up Go
3. Run `go test ./...`
4. Set up Docker Buildx
5. Log in to GHCR for non-PR events
6. Build a multi-arch image for `linux/amd64` and `linux/arm64`
7. Push the image to GHCR on `main` and version tags

Operational note:

- After the first publish, confirm the GHCR package visibility is public if you want anonymous pulls from `docker compose`

## Deployment Modes

### Local

Command:

```bash
go run .
```

Best for:

- development
- debugging
- local MCP integration tests

### Docker via GHCR

Commands:

```bash
docker compose pull
docker compose run --rm -i aegis-ssh-mcp
```

Compose behavior:

- Uses `ghcr.io/sparksbenjamin/aegis-ssh-mcp:${AEGIS_IMAGE_TAG:-latest}`
- Pulls from GHCR instead of building locally
- `AEGIS_IMAGE_TAG` can pin a specific published image tag

Volume expectations:

- `./configs` -> `/configs`
- `./rules` -> `/rules`
- `./keys` -> `/keys`

Security posture in the container:

- distroless runtime image
- non-root user
- read-only root filesystem
- `cap_drop: ALL`
- `no-new-privileges`
- `tmpfs` for `/tmp`

## Validation Performed In This Session

Completed:

- GitHub Actions workflow added for Docker image publishing to GHCR
- `docker-compose.yml` switched from local build to GHCR pulls
- README updated for the published image flow
- tech spec updated for the new delivery pipeline

Notes:

- The local Go code was not changed in this session
- The GitHub Actions workflow was added but not executed inside this workspace
- GHCR package visibility may need a one-time check after the first publish

## Important Maintenance Rules

When changing this project in future sessions:

1. Update this tech spec if behavior, structure, or deployment steps changed.
2. Keep README focused on deployers and operators.
3. Keep host samples in `configs/` and rule samples in `rules/`.
4. Keep private keys out of git.
5. Re-run `go test ./...` and `go build ./...` before finalizing code changes.
6. Keep the GHCR image reference in `docker-compose.yml` aligned with the repo owner/name.

## Known Follow-Up Opportunities

- Add signed release tags and documented versioning rules
- Add a separate non-publishing CI workflow if branch validation should stay independent from image publishing
- Add tests for MCP tool registration and config reload behavior
- Add example MCP client configs for specific clients if needed
