# Aegis-SSH-MCP

![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go)
![MCP compatible](https://img.shields.io/badge/MCP-Compatible-blue?style=flat-square)
![Docker supported](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker)
![Docker publish workflow status](https://img.shields.io/github/actions/workflow/status/sparksbenjamin/Aegis-SSH-MCP/docker-publish.yml?style=flat-square)
![MIT license](https://img.shields.io/github/license/sparksbenjamin/Aegis-SSH-MCP?style=flat-square)
![Go module version](https://img.shields.io/github/go-mod/go-version/sparksbenjamin/Aegis-SSH-MCP?style=flat-square)

A thin MCP-native SSH bridge for AI agents.

Aegis exposes host-scoped MCP tools backed by SSH, with rule-based validation, ephemeral per-request execution, and audit logging. It is designed to work with existing Linux permissions, sudo policy, and host security rather than replace them.

Quick links:

- [Quick start](#quick-start)
- [How it works](#how-it-works)
- [Configuration](#configuration)
- [Connect a client](#connect-a-client)
- [Security model](#security-model)
- [Docs](#docs)
- [Support](#support)

## Quick start

The checked-in [docker-compose.yml](docker-compose.yml) is the recommended deployment path. It runs Aegis over SSE on `http://localhost:8443` by default and ships with starter rule profiles in [`rules/`](rules/).

1. Create or populate these folders next to `docker-compose.yml`:

```text
./configs
./rules
./keys
./certs   # only if you want HTTPS
```

2. Put an SSH private key in `keys/` and keep its permissions strict.

3. Add a host config in `configs/docker.json`:

```json
{
  "alias": "docker",
  "host_ip": "192.168.1.10",
  "ssh_port": 22,
  "ssh_user": "ops",
  "auth_method": "key",
  "key_path": "/keys/docker_ed25519",
  "rule_profile": "docker-readonly",
  "timeout_seconds": 30,
  "host_key_fingerprint": "SHA256:replace-this-with-your-real-host-key",
  "api_keys": [
    "change-me-docker-key"
  ]
}
```

4. Start the service:

```bash
docker compose pull
docker compose up -d
docker compose logs -f aegis-ssh-mcp
```

5. Connect your MCP client to:

```text
http://localhost:8443/mcp/docker/sse
Authorization: Bearer change-me-docker-key
```

If you want HTTPS, uncomment the TLS lines in [docker-compose.yml](docker-compose.yml) and set `AEGIS_SSE_BASE_URL` to your real external address.

<details>
<summary>Build from source</summary>

```bash
git clone https://github.com/sparksbenjamin/Aegis-SSH-MCP.git
cd Aegis-SSH-MCP
go build -o aegis-ssh-mcp .
```

</details>

## Why it exists

Most AI infrastructure tooling tends toward one of two extremes:

- raw shell access with very little control
- a heavy replacement platform for existing SSH and Linux workflows

Aegis is the middle path:

> Keep SSH authoritative.
> Keep Linux authoritative.
> Keep infrastructure ownership where it already belongs.

It gives MCP clients a controlled way to reach real hosts without turning Aegis into a replacement for your existing security model.

## What it does

- Creates one host-scoped MCP endpoint per host config
- Exposes one host-scoped SSH tool plus `aegis_status` for that endpoint
- Validates commands against rule profiles before SSH is attempted
- Opens a fresh SSH session for every request
- Supports both `sse` and `stdio` transport
- Logs command attempts for audit visibility
- Supports host key pinning, bearer tokens, and optional output redaction
- Hot-reloads config and rule changes from disk

## What it does not do

- Replace SSH, Linux permissions, or sudo policy
- Provide OS-level sandboxing or full IAM-style RBAC
- Keep persistent remote shells, PTYs, or hidden session state
- Support SSH agent forwarding or `~/.ssh/config`
- Pause commands for human approval
- Act as a full autonomous infrastructure platform

## How it works

For each allowed request, Aegis parses the command, validates it against the assigned rule profile, opens a new non-interactive SSH session, runs the command, returns the output, and disconnects immediately.

<details>
<summary>Architecture view</summary>

```text
+-------------------+
| MCP Client / LLM  |
| Claude / OpenAI   |
| LibreChat / SSE   |
+---------+---------+
          |
          | MCP (HTTP/SSE or stdio)
          |
+---------v---------+
|  Aegis-SSH-MCP    |
|-------------------|
| Bearer Auth       |
| Rule Validation   |
| Audit Logging     |
| Host Isolation    |
| Ephemeral SSH     |
+---------+---------+
          |
          | Standard SSH
          |
+---------v---------+
| Remote Linux Host |
|-------------------|
| SSH Permissions   |
| sudo Policies     |
| auditd/journald   |
| Host Security     |
+-------------------+
```

</details>

## Configuration

### Host configs

One JSON file in [`configs/`](configs/) equals one remote host.

- One host config = one MCP endpoint
- One host config = one host-scoped SSH tool
- One host config = one assigned rule profile
- One host config = one bearer-token boundary for SSE

Use simple aliases such as `web-01` or `docker`. The alias becomes part of the endpoint path and tool name.

<details>
<summary>Optional host fields</summary>

- `stealth_mode`: returns a fake normal-looking response for blocked commands
- `fake_response`: custom fake response when `stealth_mode` is on
- `redaction_enabled`: applies regex-based output masking before results are returned
- `redaction_patterns`: regex list used for output redaction
- `host_key_fingerprint`: strongly recommended host key pinning

</details>

### Rule profiles

Each host config points to one rule profile with:

```json
"rule_profile": "docker-readonly"
```

Rule profiles live in [`rules/`](rules/) and decide which command shapes are allowed or blocked before SSH is attempted.

<details>
<summary>Shipped starter profiles</summary>

- `readonly-safe`
- `debian-readonly`
- `debian-ops`
- `ubuntu-readonly`
- `ubuntu-ops`
- `rhel-readonly`
- `rhel-ops`
- `proxmox-readonly`
- `proxmox-ops`
- `docker-readonly`
- `docker-ops`
- `systemd-ops`
- `kubernetes-readonly`
- `network-diagnostics`
- `logs-readonly`
- `package-readonly`

</details>

<details>
<summary>Rule validation model</summary>

Aegis validates commands in this order:

1. Parse the command into executable plus arguments
2. Reject shell control features such as redirects, chaining, and command substitution
3. Allow only a limited set of safe pipeline filters
4. Apply executable, argument, and full-command blacklist checks
5. Apply executable, argument, and full-command whitelist checks
6. Attempt SSH only if all checks pass

</details>

## Connect a client

Each host config becomes its own host-scoped SSE endpoint:

```text
http://YOUR_AEGIS_HOST:8443/mcp/<alias>/sse
```

Clients must send the configured bearer token:

```text
Authorization: Bearer YOUR_TOKEN
```

Quick reachability check:

```bash
curl -i -N \
  -H "Authorization: Bearer change-me-docker-key" \
  http://YOUR_AEGIS_HOST:8443/mcp/docker/sse
```

If the token is valid, Aegis should return `200 OK` and keep the SSE stream open.

If one agent needs access to two different hosts, add Aegis twice in the client with one endpoint URL and one bearer token per host alias.

<details>
<summary>LibreChat example</summary>

```yaml
mcpSettings:
  allowedDomains:
    - "192.168.100.184"

mcpServers:
  aegis-docker:
    type: sse
    url: "http://192.168.100.184:8443/mcp/docker/sse"
    headers:
      Authorization: "Bearer change-me-docker-key"
    timeout: 120000
    initTimeout: 30000
```

</details>

## Security model

Aegis is a controlled SSH execution bridge, not a full security platform.

Security boundaries come from the combination of:

- SSH authentication
- Linux user permissions
- sudo configuration
- host isolation
- command validation rules
- bearer-authenticated MCP access
- audit logging
- ephemeral execution isolation

Operators still own:

- credential management
- host hardening
- least-privilege remote accounts
- network security
- audit retention
- infrastructure segmentation

## Docs

The README stays focused on deployment and first-use. The deeper docs below are still in the repo, but their core purpose is summarized here first.

Authoring rule for this README: [docs/readme-authoring.md](docs/readme-authoring.md)

<details>
<summary>Config guide</summary>

Full guide: [docs/config.md](docs/config.md)

Highlights:

- one config file equals one host
- config changes are hot-reloaded
- aliases become endpoint paths and tool names
- `api_keys` are optional for `stdio` and effectively required for `sse`
- key paths must be container-visible paths such as `/keys/my-server.pem`

</details>

<details>
<summary>Rule guide</summary>

Full guide: [docs/rules.md](docs/rules.md)

Highlights:

- use a tight executable whitelist first
- constrain arguments and full command shapes second
- layer blacklists on top for shell escape paths
- starter profiles for Docker, Debian, Ubuntu, RHEL, Proxmox, Kubernetes, logs, and diagnostics are already included

</details>

<details>
<summary>FAQ</summary>

Full guide: [docs/FAQ.md](docs/FAQ.md)

Highlights:

- no persistent SSH shell is handed to the agent
- Aegis reduces abuse risk, but it is not a remote sandbox
- audit logs are structured and useful for investigation
- output redaction helps, but it is not full data-loss prevention

</details>

<details>
<summary>Technical spec</summary>

Full spec: [docs/tech-specs/aegis-ssh-mcp-tech-spec.md](docs/tech-specs/aegis-ssh-mcp-tech-spec.md)

Use the tech spec when you want the deeper implementation model, transport details, and runtime behavior.

</details>

## Screenshots

<details>
<summary>Open screenshots</summary>

![Aegis project screenshot showing the README hero presentation and key project messaging](https://github.com/user-attachments/assets/6af3ab55-ca74-4a38-9ffa-799ce545a9f4)
![Aegis project screenshot showing a longer walkthrough of project details and configuration content](https://github.com/user-attachments/assets/034e35ff-b53b-4619-9c32-c9bc8cd20704)

</details>

## Status

Aegis is early-stage but operational.

<details>
<summary>Current capabilities</summary>

Current capabilities include:

- MCP over HTTP/SSE
- MCP over stdio
- multi-host configuration
- rule-based validation
- audit logging
- SSH key authentication
- password authentication
- host fingerprint pinning
- ephemeral per-request SSH execution

</details>

## Support

For bugs, setup questions, or operational feedback, open an issue in this repository.

Primary maintainer: [@sparksbenjamin](https://github.com/sparksbenjamin)

Contributions are welcome, especially around:

- MCP client interoperability
- rule validation improvements
- observability
- deployment hardening
- transport support
- testing and validation

Until a dedicated contributing guide lands, opening an issue before a large change is the best way to align on direction.

## License

MIT License
