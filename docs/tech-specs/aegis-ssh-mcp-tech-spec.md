# Aegis-SSH-MCP Technical Notes

This is the living technical handoff document for Aegis-SSH-MCP.
Update it whenever behavior, architecture, deployment, or repo layout changes.

## Purpose

Aegis-SSH-MCP exposes approved SSH actions to MCP clients while acting as a command firewall.

Core flow:

1. Receive an MCP tool call.
2. Map that tool call to a configured host alias.
3. Parse the command into executable plus arguments.
4. Validate the parsed command against a rule profile.
5. Rebuild the argv into a shell-safe normalized command string.
6. Execute that normalized command over a single SSH session only if validation passed.
7. Return output and write an audit record.

## Current Architecture

### MCP surface

The server exposes:

- one host tool per config file: `aegis_ssh_<alias>`
- one status tool: `aegis_status`

Primary deployment transport:

- `HTTPS`
- `SSE`
- one Aegis instance per port
- bearer-token-filtered tool visibility per session

Local fallback transport:

- `stdio`

### HTTPS SSE access model

This project now treats HTTPS SSE as the recommended operator-facing deployment model.

Connection pattern:

```text
GET /mcp/sse
Authorization: Bearer YOUR_TOKEN
```

Meaning:

- the port identifies the Aegis service instance
- the bearer token identifies which host tools the client is allowed to see and call

Important behavior:

- a host config can define multiple API keys
- those API-key values are used as accepted bearer tokens
- the same token can be reused across multiple hosts
- tool visibility is the union of all hosts that contain that token
- `aegis_status` stays available to authenticated SSE clients, but only reports visible hosts

### Bearer-token requirement

The official MCP authorization spec for HTTP transports requires:

```text
Authorization: Bearer <access-token>
```

And it explicitly says authorization must be included on every HTTP request.

This repo now aligns its documented deployment model to that standard.

Query-string tokens are not part of the recommended deployment path.

### Session auth behavior

Implementation notes:

- the initial SSE request authenticates with a bearer token
- that token is bound to the generated session ID
- later POSTs to the message endpoint can authorize through that session ID alone
- if a POST includes both a session ID and a token, the token must match the session's stored token
- each POST re-derives the current allowed host set from the stored token

That last point matters: config changes apply to existing SSE sessions without requiring a process restart.

## Repo Layout

```text
.
|-- .github/
|   `-- workflows/
|       `-- docker-publish.yml
|-- certs/
|   `-- .gitkeep
|-- configs/
|   |-- dell-r820.json
|   `-- proxmox-node.json
|-- docs/
|   `-- tech-specs/
|       `-- aegis-ssh-mcp-tech-spec.md
|-- internal/
|   |-- audit/
|   |   `-- logger.go
|   |-- command/
|   |   |-- command.go
|   |   `-- command_test.go
|   |-- config/
|   |   |-- loader.go
|   |   `-- loader_test.go
|   |-- mcp/
|   |   |-- access.go
|   |   |-- access_test.go
|   |   |-- server.go
|   |   `-- sse.go
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

## Runtime Startup

`main.go` now supports transport selection through `AEGIS_TRANSPORT`.

Supported values:

- `stdio`
- `sse`

Default:

- `stdio`

Directory resolution behavior:

- `AEGIS_CONFIGS_DIR` overrides config path
- `AEGIS_RULES_DIR` overrides rule path
- otherwise local `configs/` and `rules/` are preferred from the repo root
- final fallback is `/configs` and `/rules` for container runs

### SSE environment variables

When `AEGIS_TRANSPORT=sse`, these settings matter:

- `AEGIS_SSE_ADDR`
  - default: `:8443`
- `AEGIS_SSE_BASE_URL`
  - required
  - example: `https://aegis.example.com:8443`
- `AEGIS_SSE_BASE_PATH`
  - default: `/mcp`
- `AEGIS_SSE_TLS_CERT_FILE`
  - required
- `AEGIS_SSE_TLS_KEY_FILE`
  - required

If no `api_keys` are configured across the host configs, SSE startup fails intentionally.
Missing or invalid bearer tokens return `401 Unauthorized` with a `WWW-Authenticate: Bearer ...` challenge.

## Package Responsibilities

### `internal/command`

Responsibilities:

- parse shell-style quoting into argv using `github.com/google/shlex`
- reject control characters
- preserve executable, arguments, and normalized full-command views
- rebuild a shell-safe normalized command string

Security intent:

- validation is done against structured argv, not the raw shell string
- execution uses the normalized command, not the original input

### `internal/config`

Responsibilities:

- parse host config JSON files
- validate required fields
- apply default `ssh_port=22`
- apply default `timeout_seconds=30`
- normalize and deduplicate `api_keys`
- scan the `configs/` directory
- reject duplicate aliases

Important behavior:

- invalid configs are skipped with a warning during scans
- duplicate aliases fail the full scan because tool identity collisions are unsafe

### `internal/rules`

Responsibilities:

- parse rule JSON
- compile regex allowlists and blocklists
- validate executables, arguments, and legacy full-command shapes
- reload all rule profiles on disk changes

Validation order:

1. executable blacklist
2. arguments blacklist
3. legacy full-command blacklist
4. executable whitelist
5. arguments whitelist
6. legacy full-command whitelist

### `internal/ssh`

Responsibilities:

- create SSH client config
- load key or password auth
- enforce key file permission checks
- optionally verify host key fingerprints
- execute one non-interactive SSH command
- collect stdout, stderr, exit code
- apply redaction before returning output

### `internal/audit`

Responsibilities:

- emit structured JSON audit logs to `stderr`
- serialize concurrent writes safely
- log command and system events

### `internal/mcp`

Responsibilities:

- create the MCP server
- register host tools
- expose `aegis_status`
- watch config and rule directories
- enforce bearer-token-based tool visibility for SSE sessions
- start stdio or HTTPS SSE serving

Key files:

- `access.go`
  - request bearer-token extraction
  - access context helpers
  - alias visibility helpers
- `server.go`
  - tool registration
  - config reload handling
  - status tool
  - command execution handlers
  - hook registration for filtered `tools/list`
- `sse.go`
  - HTTPS listener setup
  - CORS handling
  - session-aware auth wrapper

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
  "host_key_fingerprint": "",
  "api_keys": [
    "change-me-server-key",
    "change-me-shared-ops-key"
  ]
}
```

Notes:

- `alias` must be unique
- `rule_profile` must match a profile in `rules/`
- `api_keys` are normalized with trimming and de-duplication
- `api_keys` are optional for stdio mode
- `api_keys` are effectively required for HTTPS SSE access because SSE startup requires at least one configured token somewhere in the config set
- for HTTPS SSE, those configured values are expected to be sent as bearer tokens in the `Authorization` header

## Tool Visibility Rules

For unauthenticated local stdio:

- all tools are visible

For authenticated SSE:

- `tools/list` is filtered to the allowed host tools for that token
- authenticated requests are expected to use `Authorization: Bearer <token>`
- `aegis_status` remains visible
- `aegis_status` only lists hosts visible to that token
- direct `tools/call` attempts against unauthorized hosts are blocked even if a client guesses a tool name

This means the auth model is not "security by hidden tool list." Visibility and execution are both enforced.

## Hot Reload Behavior

Watched directories:

- `configs/`
- `rules/`

Behavior:

- config changes trigger a full config rescan
- rule changes trigger a full rules reload
- host config updates refresh live config without duplicating the MCP tool registration
- removed hosts are deleted from the in-memory registry
- token-to-host mappings are rebuilt on every config sync
- active SSE sessions pick up those token mapping changes on the next request

Known limitation:

- upstream MCP tool removal behavior is limited
- if a host config is removed, some clients may keep showing the old tool name until they refresh
- calls still fail safely because the backing config is removed

## Container and Deployment Model

Registry:

- `ghcr.io/sparksbenjamin/aegis-ssh-mcp`

Operator quick-start expectation:

- the primary deployment path is Docker Compose
- operators should be able to deploy from the published GHCR image without building from source
- README quick start should stay copy-paste friendly and center on host-mounted config, rule, key, and cert paths

Compose behavior:

- pulls from GHCR instead of building locally
- defaults to `latest`
- uses HTTPS SSE transport
- mounts:
  - `./configs` -> `/configs`
  - `./rules` -> `/rules`
  - `./keys` -> `/keys`
  - `./certs` -> `/certs`
- publishes the configured SSE port to the host

Current compose env defaults:

- `AEGIS_TRANSPORT=sse`
- `AEGIS_CONFIGS_DIR=/configs`
- `AEGIS_RULES_DIR=/rules`
- `AEGIS_SSE_ADDR=:8443`
- `AEGIS_SSE_BASE_URL=https://localhost:8443`
- `AEGIS_SSE_BASE_PATH=/mcp`
- `AEGIS_SSE_TLS_CERT_FILE=/certs/tls.crt`
- `AEGIS_SSE_TLS_KEY_FILE=/certs/tls.key`

Operator note:

- if you change the port, hostname, or both, update `AEGIS_SSE_BASE_URL` to match the externally reachable HTTPS address

## GitHub Actions Image Publishing

Workflow:

- `.github/workflows/docker-publish.yml`

Behavior:

- push to `main` publishes `latest`
- push a tag like `v1.2.3` publishes that version tag
- each publish also gets a `sha-...` tag
- pull requests build for validation without publishing

Important implementation detail:

- the Dockerfile builder image is aligned to Go 1.23 because `go.mod` requires 1.23
- the build uses `TARGETOS` and `TARGETARCH` so multi-arch GHCR publishing produces correct binaries for `amd64` and `arm64`

## Security Notes

- commands are parsed before validation
- raw command strings are not executed directly
- a normalized shell-safe command is executed instead
- command validation occurs before any SSH connection attempt
- API keys gate tool visibility and tool execution for SSE sessions
- HTTPS SSE requests challenge with `WWW-Authenticate: Bearer ...` when the token is missing or invalid
- if `host_key_fingerprint` is empty, host-key verification is insecure
- TLS is required for the recommended SSE deployment model

## Validation Performed In This Session

Completed:

- added HTTPS SSE transport support
- added bearer-token-based host access control for SSE
- added session-bound access behavior for SSE clients
- added config support for `api_keys`
- updated sample configs with `api_keys`
- updated docker compose for GHCR-based HTTPS SSE deployment
- updated README for the new operator flow
- updated this tech spec for the new architecture

Verification:

- `gofmt -w internal/config/loader.go internal/config/loader_test.go internal/mcp/access.go internal/mcp/access_test.go internal/mcp/server.go internal/mcp/sse.go main.go`
- `go test ./...`
- `go build -buildvcs=false ./...`

Not performed here:

- a live HTTPS container run
- an end-to-end external SSE client handshake against a running container

## Maintenance Rules

When changing this project in future sessions:

1. Update this tech spec when architecture or deployment behavior changes.
2. Keep the README focused on deployers and operators.
3. Keep sample host configs in `configs/`.
4. Keep private keys and certificates out of git.
5. Re-run `go test ./...` and `go build -buildvcs=false ./...`.
6. Keep `docker-compose.yml` pointing at the GHCR image unless there is a deliberate release process change.
