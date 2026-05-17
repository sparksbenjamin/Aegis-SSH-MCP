# Aegis-SSH-MCP

Aegis-SSH-MCP is a Go-based MCP server that gives AI agents tightly controlled SSH access to remote systems.
Each host is exposed as its own MCP tool, and every command is parsed into argv, validated, normalized into a shell-safe form, and only then executed over SSH.

## What You Get

- One MCP tool per host config, such as `aegis_ssh_proxmox-node`
- Executable and argument validation before any network call is made
- Optional stealth responses for research and honeypot-style testing
- Optional output redaction before command results go back to the model
- Structured JSON audit logs written to `stderr`
- Hot reload for both `configs/*.json` and `rules/*.json`
- Automatic Docker image publishing to `ghcr.io/sparksbenjamin/aegis-ssh-mcp`

## Repo Layout

```text
.
|-- .github/workflows/     CI and container publishing
|-- configs/               Host definitions
|-- docs/tech-specs/       Internal technical notes and handoff docs
|-- internal/
|   |-- audit/             Audit logging
|   |-- command/           Command parsing and shell-safe normalization
|   |-- config/            Host config loading and validation
|   |-- mcp/               MCP server wiring and file watchers
|   |-- rules/             Command rule engine
|   `-- ssh/               SSH execution layer
|-- keys/                  SSH private keys (not committed)
|-- rules/                 Rule profiles
|-- Dockerfile
|-- docker-compose.yml
|-- go.mod
|-- go.sum
`-- main.go
```

## Image Publishing

This repo is set up to build and publish a new Docker image automatically through GitHub Actions.

- Push to `main` -> publishes a fresh `latest` image
- Push a tag like `v1.2.3` -> publishes a versioned image tag
- Every publish also gets a `sha-...` image tag

Published image:

```text
ghcr.io/sparksbenjamin/aegis-ssh-mcp:latest
```

If GHCR package visibility is private after the first publish, switch it to public in GitHub so `docker compose` can pull it without extra auth.

## Quick Start

### 1. Clone the repo

```bash
git clone https://github.com/sparksbenjamin/Aegis-SSH-MCP.git
cd Aegis-SSH-MCP
```

### 2. Add your SSH keys

Put your private keys in `keys/`.

Examples:

```text
keys/proxmox.pem
keys/dell-r820.pem
```

Important:

- Keep private keys out of git.
- Use restrictive permissions where your platform supports them.
- Update each host config so `key_path` matches the path the runtime will see.

For Docker, that normally means paths like `/keys/proxmox.pem`.

### 3. Add or edit host configs

Host configs live in `configs/`.
Two examples are already included:

- `configs/proxmox-node.json`
- `configs/dell-r820.json`

Example host config:

```json
{
  "alias": "my-server",
  "host_ip": "192.168.1.100",
  "ssh_port": 22,
  "ssh_user": "root",
  "auth_method": "key",
  "key_path": "/keys/my-server.pem",
  "rule_profile": "readonly-safe",
  "timeout_seconds": 30
}
```

Required fields:

- `alias`
- `host_ip`
- `ssh_user`
- `auth_method`
- `rule_profile`

Notes:

- `alias` must be unique.
- `auth_method` must be `key` or `password`.
- `key_path` is required for key auth.
- `password` is required for password auth.

### 4. Choose a rule profile

Rule profiles live in `rules/`.
Included examples:

- `rules/readonly-safe.json`
- `rules/docker-readonly.json`
- `rules/docker-ops.json`

Rules work like this:

1. The input is parsed into an executable plus arguments.
2. A blacklist runs first and blocks forbidden executables, arguments, or legacy full-command patterns.
3. A whitelist runs second and only allows approved executables, arguments, and command shapes.
4. The parsed argv is rebuilt into a shell-safe normalized command before execution.

### 5. Run Aegis from the published GitHub image

Pull the latest image:

```bash
docker compose pull
```

Run the service:

```bash
docker compose run --rm -i aegis-ssh-mcp
```

To pin a specific published image version, set `AEGIS_IMAGE_TAG` before running compose.

### 6. Local development

If you want to run from source instead of GHCR:

```bash
go run .
```

When run from the repo root, Aegis automatically uses the local `configs/` and `rules/` folders.

## Connect It To Your MCP Client

Example MCP client configuration using Docker Compose:

```json
{
  "mcpServers": {
    "aegis": {
      "command": "docker",
      "args": [
        "compose",
        "-f",
        "/absolute/path/to/Aegis-SSH-MCP/docker-compose.yml",
        "run",
        "--rm",
        "-i",
        "aegis-ssh-mcp"
      ]
    }
  }
}
```

Example MCP client configuration using a local Go run:

```json
{
  "mcpServers": {
    "aegis": {
      "command": "go",
      "args": ["run", "."],
      "cwd": "/absolute/path/to/Aegis-SSH-MCP"
    }
  }
}
```

## Tools Exposed

For each file in `configs/`, Aegis exposes:

- `aegis_ssh_<alias>`

It also exposes:

- `aegis_status`

## Security Notes

- Aegis uses single-command SSH sessions. It does not open an interactive shell.
- If a command fails validation, SSH is never attempted.
- Shell operators and expansion tricks are not passed through as-is. Aegis executes a normalized command rebuilt from parsed argv.
- If `host_key_fingerprint` is empty, Aegis falls back to insecure host key verification. That is okay for a lab, not for production.
- If `redaction_enabled` is true, matching output is replaced with `[REDACTED]`.
- If `stealth_mode` is true, blocked commands can return a fake response instead of an explicit error.

## Hot Reload

- Editing `configs/*.json` updates the live host registry.
- Editing `rules/*.json` reloads the rule engine.
- Removing a host config leaves the old tool name visible until the MCP client refreshes, but calls to it will fail safely.

## Development

```bash
go test ./...
go build ./...
```

## Internal Notes

The detailed technical handoff document lives here:

- [Tech spec](docs/tech-specs/aegis-ssh-mcp-tech-spec.md)
