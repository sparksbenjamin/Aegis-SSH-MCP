# Aegis-SSH-MCP

Aegis-SSH-MCP is a Go-based MCP gateway that gives AI agents controlled SSH access to remote systems.
Each host is exposed as its own MCP tool, and every command is parsed, validated, normalized into a shell-safe form, and only then executed over SSH.

## Recommended Connection Model

The recommended way to run Aegis is:

- deploy the published GitHub Container Registry image
- expose it over `HTTPS`
- connect to it with MCP over `SSE`
- send a bearer token in the `Authorization` header
- use an `api_keys` entry in each host config to control which tools a client can see

In practice, the connection is:

```text
GET /mcp/sse
Authorization: Bearer YOUR_TOKEN
```

The port selects the Aegis instance.
The bearer token selects which host tools that client can use.

## What You Get

- One MCP tool per host config, such as `aegis_ssh_proxmox-node`
- Per-token tool filtering for HTTPS SSE clients
- Executable and argument validation before any SSH call is made
- Shell-safe command normalization before execution
- Optional stealth responses
- Optional output redaction
- Structured audit logs on `stderr`
- Hot reload for `configs/*.json` and `rules/*.json`
- Automatic GHCR image publishing from GitHub Actions

## Quick Start

### 1. Clone the repo

```bash
git clone https://github.com/sparksbenjamin/Aegis-SSH-MCP.git
cd Aegis-SSH-MCP
```

### 2. Add SSH keys

Place private keys in `keys/`.

Examples:

```text
keys/proxmox.pem
keys/dell-r820.pem
```

Keep them out of git.

### 3. Add or edit host configs

Host configs live in `configs/`.
Each host can define one or more `api_keys`.
These values are used as accepted bearer tokens.
If the same token appears on multiple hosts, that token will see all of those host tools.

Example:

```json
{
  "alias": "my-server",
  "host_ip": "192.168.1.100",
  "ssh_port": 22,
  "ssh_user": "root",
  "auth_method": "key",
  "key_path": "/keys/my-server.pem",
  "rule_profile": "readonly-safe",
  "timeout_seconds": 30,
  "api_keys": [
    "change-me-my-server-key",
    "change-me-shared-ops-key"
  ]
}
```

Important notes:

- `alias` must be unique
- `auth_method` must be `key` or `password`
- `key_path` is required for key auth
- `password` is required for password auth
- `api_keys` are optional for local stdio use, but required if you want HTTPS SSE access
- for HTTPS SSE, clients must send those values as `Authorization: Bearer <token>`

### 4. Choose a rule profile

Rule profiles live in `rules/`.
Included examples:

- `rules/readonly-safe.json`
- `rules/docker-readonly.json`
- `rules/docker-ops.json`

Rules validate the executable, arguments, and legacy full-command patterns before SSH is attempted.

### 5. Add TLS certificates

The HTTPS SSE deployment expects:

```text
certs/tls.crt
certs/tls.key
```

For a quick local certificate:

```bash
openssl req -x509 -nodes -newkey rsa:2048 -keyout certs/tls.key -out certs/tls.crt -days 365 -subj "/CN=localhost"
```

For real deployments, use a certificate that matches the hostname in `AEGIS_SSE_BASE_URL`.

### 6. Start the published container

Pull the image:

```bash
docker compose pull
```

Start the service:

```bash
docker compose up -d
```

Follow logs if needed:

```bash
docker compose logs -f aegis-ssh-mcp
```

By default, `docker-compose.yml` pulls:

```text
ghcr.io/sparksbenjamin/aegis-ssh-mcp:latest
```

Set `AEGIS_IMAGE_TAG` if you want to pin a specific published version.

## Docker Compose Settings

The compose file is already set up to pull from GHCR and run the HTTPS SSE transport.

Most important settings:

- `AEGIS_SSE_PORT`
- `AEGIS_SSE_BASE_URL`
- `AEGIS_IMAGE_TAG`

Example `.env`:

```dotenv
AEGIS_SSE_PORT=8443
AEGIS_SSE_BASE_URL=https://aegis.example.com:8443
AEGIS_IMAGE_TAG=latest
```

If you change the port, update `AEGIS_SSE_BASE_URL` to match it.

## Connect Your MCP Client

Recommended SSE deployment:

```text
URL: https://HOST:PORT/mcp/sse
Header: Authorization: Bearer YOUR_TOKEN
```

Example client config for clients that support SSE plus custom headers:

```json
{
  "mcpServers": {
    "aegis": {
      "transport": "sse",
      "url": "https://aegis.example.com:8443/mcp/sse",
      "headers": {
        "Authorization": "Bearer change-me-shared-ops-key"
      }
    }
  }
}
```

This repo now treats bearer-header auth as the only documented remote auth path.
Query-string tokens are intentionally not part of the deployment guidance.

## Tools Exposed

For each file in `configs/`, Aegis exposes:

- `aegis_ssh_<alias>`

It also exposes:

- `aegis_status`

For HTTPS SSE clients, the visible tool list is filtered by the bearer token used for that session.

## Local Development

Local stdio mode still works for development:

```bash
go run .
```

When run from the repo root, Aegis automatically uses the local `configs/` and `rules/` folders.

## Security Notes

- Aegis runs one non-interactive SSH command per request
- If validation fails, SSH is never attempted
- Raw shell strings are not executed directly
- Commands are rebuilt into a normalized shell-safe form before execution
- If `host_key_fingerprint` is empty, host verification falls back to insecure mode
- If `redaction_enabled` is true, matching output is replaced with `[REDACTED]`
- If `stealth_mode` is true, blocked commands can return a fake response

## Development Checks

```bash
go test ./...
go build -buildvcs=false ./...
```

## Technical Handoff

The detailed living tech spec is here:

- [docs/tech-specs/aegis-ssh-mcp-tech-spec.md](docs/tech-specs/aegis-ssh-mcp-tech-spec.md)
