# Aegis-SSH-MCP

Aegis-SSH-MCP lets you give an AI agent controlled SSH access to a specific machine through MCP.

The important model is simple:

- one Aegis container
- one exposed port
- one MCP endpoint per host config
- one bearer token per host config

Each endpoint exposes that host's SSH tool and `aegis_status`. Commands are parsed, validated, normalized into a shell-safe form, and only then executed over SSH.

## Quick Start

You do not need to build from source to use Aegis.
The checked-in Docker Compose example defaults to local HTTP so people can start quickly.
If you want HTTPS, the TLS lines are right there to uncomment.

### 1. Create folders on the host

Example:

```text
/opt/aegis/configs
/opt/aegis/rules
/opt/aegis/keys
/opt/aegis/certs   (only if you want HTTPS)
```

### 2. Create a host config

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
  "host_key_fingerprint": "",
  "api_keys": [
    "change-me-my-server-token"
  ]
}
```

What matters here:

- `alias` becomes the MCP endpoint path
- `key_path` must use the container path, not the host path
- each bearer token must belong to only one host config
- clients must send the token as `Authorization: Bearer <token>`
- if `host_key_fingerprint` is empty, Aegis will warn and use insecure host-key verification

### 3. Add your SSH key

Expected container path:

```text
/keys/my-server.pem
```

If you want HTTPS, also add TLS certs:

Expected container paths:

```text
/certs/tls.crt
/certs/tls.key
```

Quick self-signed cert example:

```bash
openssl req -x509 -nodes -newkey rsa:2048 -keyout /opt/aegis/certs/tls.key -out /opt/aegis/certs/tls.crt -days 365 -subj "/CN=localhost"
```

### 4. Save this Docker Compose file

Replace the host paths and hostname with yours:

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
      AEGIS_SSE_BASE_URL: http://localhost:8443
      AEGIS_SSE_BASE_PATH: /mcp
      AEGIS_SSE_DISABLE_TLS: "true"
      # Uncomment for HTTPS:
      # AEGIS_SSE_BASE_URL: https://aegis.example.com:8443
      # AEGIS_SSE_TLS_CERT_FILE: /certs/tls.crt
      # AEGIS_SSE_TLS_KEY_FILE: /certs/tls.key

    ports:
      - "8443:8443"

    volumes:
      - /opt/aegis/configs:/configs
      - /opt/aegis/rules:/rules
      - /opt/aegis/keys:/keys:ro
      # Uncomment for HTTPS:
      # - /opt/aegis/certs:/certs:ro
```

### 5. Start it

```bash
docker compose pull
docker compose up -d
docker compose logs -f aegis-ssh-mcp
```

### 6. Connect your MCP client

If your config alias is `my-server`, the endpoint is:

```text
URL: http://localhost:8443/mcp/my-server/sse
Header: Authorization: Bearer change-me-my-server-token
```

That is the main deployment flow.
If you want HTTPS, uncomment the TLS lines in the Compose file and switch the URL to `https://...`.

## Client Examples

### One host

Local HTTP example:

```json
{
  "mcpServers": {
    "aegis-my-server": {
      "transport": "sse",
      "url": "http://localhost:8443/mcp/my-server/sse",
      "headers": {
        "Authorization": "Bearer change-me-my-server-token"
      }
    }
  }
}
```

### Two hosts

If you want one agent to reach two boxes, add Aegis twice with two different URLs and two different bearer tokens:

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

If you switch the server to HTTPS, change those URLs to `https://...`.

## How Aegis Maps Hosts

- Config alias `my-server` becomes endpoint `/mcp/my-server/sse`
- Each endpoint exposes one host-scoped SSH tool like `aegis_ssh_my-server`
- Each endpoint also exposes `aegis_status`
- One bearer token cannot be reused across different host configs

## Rules and Safety

Rule profiles live in `rules/`.
Included examples:

- `readonly-safe`
- `docker-readonly`
- `docker-ops`

Full rule authoring guide:

- [docs/rules.md](docs/rules.md)

Important behavior:

- commands are validated before SSH is attempted
- raw shell strings are not executed directly
- Aegis rebuilds a normalized shell-safe command before execution
- if validation fails, SSH is never attempted

## Optional Local HTTP Mode

The checked-in Docker Compose example defaults to local or lab HTTP mode:

```text
AEGIS_SSE_DISABLE_TLS=true
AEGIS_SSE_BASE_URL=http://localhost:8443
```

In that mode:

- cert files are not required
- client URLs use `http://...`
- you should not use it on untrusted networks

Example:

```text
URL: http://localhost:8443/mcp/my-server/sse
Header: Authorization: Bearer change-me-my-server-token
```

To switch to HTTPS:

- set `AEGIS_SSE_DISABLE_TLS=false`
- set `AEGIS_SSE_BASE_URL=https://your-host:8443`
- uncomment `AEGIS_SSE_TLS_CERT_FILE` and `AEGIS_SSE_TLS_KEY_FILE`
- uncomment the `/certs` mount

## Docker Compose Notes

The repo copy of [docker-compose.yml](docker-compose.yml) uses relative mounts for running from this checkout.
For real deployments, update the volume source paths to your host directories.

Common settings:

- `AEGIS_SSE_BASE_URL`
- `AEGIS_SSE_PORT`
- `AEGIS_IMAGE_TAG`

Example `.env`:

```dotenv
AEGIS_SSE_PORT=8443
AEGIS_SSE_BASE_URL=http://localhost:8443
AEGIS_SSE_DISABLE_TLS=true
AEGIS_IMAGE_TAG=latest
```

If you change the port, update `AEGIS_SSE_BASE_URL` to match it.

## Local Development

If you want to run from source instead:

```bash
go run .
```

When run from the repo root, Aegis uses the local `configs/` and `rules/` folders automatically.

## Validation

```bash
go test ./...
go build -buildvcs=false ./...
```

## Technical Handoff

The living technical notes are here:

- [docs/tech-specs/aegis-ssh-mcp-tech-spec.md](docs/tech-specs/aegis-ssh-mcp-tech-spec.md)
- [docs/rules.md](docs/rules.md)
