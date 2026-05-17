# Aegis-SSH-MCP

Aegis-SSH-MCP is a Go-based MCP gateway that gives AI agents controlled SSH access to remote systems.
Each host is exposed as its own MCP tool, and every command is parsed, validated, normalized into a shell-safe form, and only then executed over SSH.

## Recommended Connection Model

The recommended way to run Aegis is:

- deploy the published GitHub Container Registry image
- expose it over `HTTPS`
- connect to it with MCP over `SSE`
- send a bearer token in the `Authorization` header
- give each host config its own endpoint path and its own bearer token

In practice, the connection is:

```text
GET /mcp/HOST_ALIAS/sse
Authorization: Bearer YOUR_TOKEN
```

The port selects the Aegis instance.
The endpoint path and bearer token together select exactly one host surface.
`HOST_ALIAS` is the sanitized config alias, so `my-server` becomes `/mcp/my-server/sse`.

## What You Get

- One MCP endpoint per host config, such as `/mcp/proxmox-node/sse`
- One SSH tool inside that endpoint, such as `aegis_ssh_proxmox-node`
- Per-host bearer-token isolation for HTTPS SSE clients
- Executable and argument validation before any SSH call is made
- Shell-safe command normalization before execution
- Optional stealth responses
- Optional output redaction
- Structured audit logs on `stderr`
- Hot reload for `configs/*.json` and `rules/*.json`
- Automatic GHCR image publishing from GitHub Actions

## Quick Start

You do not need to build this project from source to deploy it.
The quick-start path is: pull the published image, mount your config paths, and start Docker Compose.

### 1. Create host folders

Create folders anywhere on your host for:

- host configs
- rule files
- SSH keys
- TLS certs

Example:

```text
/opt/aegis/configs
/opt/aegis/rules
/opt/aegis/keys
/opt/aegis/certs
```

### 2. Add a host config

Save a file such as `/opt/aegis/configs/my-server.json`:

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
    "change-me-my-server-token"
  ]
}
```

Important:

- `alias` must be unique
- `key_path` must match the container path, not the host path
- each bearer token must belong to only one host config
- `api_keys` are the bearer tokens your MCP clients will use for this host endpoint
- clients must send them as `Authorization: Bearer <token>`

### 3. Add your SSH key and TLS certs

Expected container paths:

```text
/keys/my-server.pem
/certs/tls.crt
/certs/tls.key
```

Quick self-signed cert example:

```bash
openssl req -x509 -nodes -newkey rsa:2048 -keyout /opt/aegis/certs/tls.key -out /opt/aegis/certs/tls.crt -days 365 -subj "/CN=localhost"
```

### 4. Copy this Docker Compose file

Save this as `docker-compose.yml` and replace the host paths with yours:

```yaml
services:
  aegis-ssh-mcp:
    image: ghcr.io/sparksbenjamin/aegis-ssh-mcp:latest
    pull_policy: always
    container_name: aegis-ssh-mcp
    restart: on-failure:5

    environment:
      AEGIS_TRANSPORT: sse
      AEGIS_CONFIGS_DIR: /configs
      AEGIS_RULES_DIR: /rules
      AEGIS_SSE_ADDR: ":8443"
      AEGIS_SSE_BASE_URL: https://aegis.example.com:8443
      AEGIS_SSE_BASE_PATH: /mcp
      AEGIS_SSE_TLS_CERT_FILE: /certs/tls.crt
      AEGIS_SSE_TLS_KEY_FILE: /certs/tls.key

    ports:
      - "8443:8443"

    volumes:
      - /opt/aegis/configs:/configs
      - /opt/aegis/rules:/rules
      - /opt/aegis/keys:/keys:ro
      - /opt/aegis/certs:/certs:ro
```

### 5. Pull and start it

```bash
docker compose pull
docker compose up -d
docker compose logs -f aegis-ssh-mcp
```

### 6. Connect your MCP client

Use:

```text
URL: https://aegis.example.com:8443/mcp/my-server/sse
Header: Authorization: Bearer change-me-my-server-token
```

That is the deploy path: pull the image, mount your files, and start the container.

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

The repo copy of [docker-compose.yml](docker-compose.yml) uses relative mounts for running from this checkout.
For real deployments, update the volume `source` paths to your host directories.

## Connect Your MCP Client

Recommended SSE deployment:

```text
URL: https://HOST:PORT/mcp/HOST_ALIAS/sse
Header: Authorization: Bearer YOUR_TOKEN
```

Example client config for clients that support SSE plus custom headers:

```json
{
  "mcpServers": {
    "aegis-proxmox": {
      "transport": "sse",
      "url": "https://aegis.example.com:8443/mcp/proxmox-node/sse",
      "headers": {
        "Authorization": "Bearer change-me-proxmox-key"
      }
    },
    "aegis-dell": {
      "transport": "sse",
      "url": "https://aegis.example.com:8443/mcp/dell-r820/sse",
      "headers": {
        "Authorization": "Bearer change-me-dell-key"
      }
    }
  }
}
```

This repo now treats bearer-header auth as the only documented remote auth path.
Query-string tokens are intentionally not part of the deployment guidance.
If you want one agent to reach two boxes, add the MCP twice with two different URLs and two different bearer tokens.

## Tools Exposed

For each file in `configs/`, Aegis exposes:

- `aegis_ssh_<alias>`

It also exposes:

- `aegis_status`

For HTTPS SSE clients, each endpoint is host-scoped and each bearer token is host-scoped.

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
