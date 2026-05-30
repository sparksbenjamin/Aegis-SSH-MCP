# Aegis-SSH-MCP

![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go)
![MCP compatible](https://img.shields.io/badge/MCP-Compatible-blue?style=flat-square)
![Docker supported](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker)
![Docker publish workflow status](https://img.shields.io/github/actions/workflow/status/sparksbenjamin/Aegis-SSH-MCP/docker-publish.yml?style=flat-square)
![MIT license](https://img.shields.io/github/license/sparksbenjamin/Aegis-SSH-MCP?style=flat-square)
![Go module version](https://img.shields.io/github/go-mod/go-version/sparksbenjamin/Aegis-SSH-MCP?style=flat-square)

**Give AI agents safe, limited SSH access without handing them a shell.**

Aegis-SSH-MCP is a small MCP-native bridge that lets an AI agent run **approved commands** on Linux hosts over SSH.

It is built for people who want agentic infrastructure workflows, but do **not** want to give an AI unrestricted terminal access.

Aegis sits between your MCP client and your servers. It checks every requested command against your rules, opens a short-lived SSH session only when the command is allowed, returns the result, and disconnects.

<img width="1536" height="1024" alt="image" src="https://github.com/user-attachments/assets/6e4b1046-e1ec-4d9e-88d5-434c4c71b07e" />

Aegis does not replace SSH, Linux permissions, sudo, or host hardening. It helps you keep those controls in charge while giving MCP clients a safer way to interact with real systems.

Quick links:

- [Quick start](#quick-start)
- [How it works](#how-it-works)
- [Core concepts](#core-concepts)
- [Security model](#security-model)
- [Client example: LibreChat](#client-example-librechat)
- [Documentation](#documentation)

## What problem does Aegis solve?

AI agents are useful when they can inspect logs, check services, look at containers, or run routine operational commands.

The dangerous version of that is simple:

> Give the agent SSH access and hope it behaves.

Aegis takes a safer approach:

> Give the agent a narrow MCP tool that can only run commands you have approved.

That means an agent can do things like check Docker status, read logs, or run diagnostics without receiving a persistent shell, a pseudo-terminal, SSH agent forwarding, or hidden session state.

## Good fit

Aegis is useful when you want to:

- connect an MCP client to Linux hosts over SSH
- let agents run a small set of operational commands
- keep command access host-scoped and rule-based
- audit what the agent tried to do
- preserve your existing SSH, Linux, sudo, and host security model

## What Aegis is not

Aegis is intentionally narrow.

It is not a replacement for SSH, Linux permissions, sudo, IAM, RBAC, host hardening, or human judgment.

It does not:

- give agents a persistent shell
- create hidden session state
- provide full OS-level sandboxing
- approve commands through a human workflow
- turn MCP into a full infrastructure automation platform

That is by design.

Aegis does one job:

**It gives an MCP client a controlled, auditable way to run approved SSH commands -- and leaves the rest of your security model intact.**

## Quick start

The recommended way to run Aegis is with the included [`docker-compose.yml`](docker-compose.yml).

Prerequisites:

- Docker Compose
- an MCP client with SSE support
- a reachable Linux host
- an SSH key or password for a least-privileged remote user

By default, Aegis exposes MCP over SSE at:

```text
http://localhost:8443
```

Starter rule profiles are included in [`rules/`](rules/). Keep or copy those profiles when deploying; the quick start only requires you to add host configs and SSH credentials.

### 1. Create the local folders

Create these folders next to `docker-compose.yml` if they do not already exist:

```text
./configs
./keys
./certs # only needed if you enable HTTPS
```

The repository already includes `./rules` with starter rule profiles.

### 2. Add an SSH key

Place the SSH private key Aegis should use in `keys/`.

Keep the key permissions strict and use a dedicated, least-privileged SSH user where possible.

### 3. Add a host config

Create `configs/docker.json`:

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

The important parts are:

- `alias`: the friendly name for this host
- `ssh_user`: the Linux user Aegis connects as
- `key_path`: the private key path inside the container
- `rule_profile`: the command rules this host uses
- `host_key_fingerprint`: pins the SSH host key
- `api_keys`: bearer tokens allowed to reach this host endpoint

### 4. Start Aegis

```bash
docker compose pull
docker compose up -d
docker compose logs -f aegis-ssh-mcp
```

### 5. Connect your MCP client

Use this SSE endpoint:

```text
http://localhost:8443/mcp/docker/sse
```

Send the bearer token configured in `configs/docker.json`:

```text
Authorization: Bearer change-me-docker-key
```

You can test reachability with curl:

```bash
curl -i -N \
  -H "Authorization: Bearer change-me-docker-key" \
  http://localhost:8443/mcp/docker/sse
```

A valid token should return `200 OK` and keep the SSE stream open.

### Optional: build from source

```bash
git clone https://github.com/sparksbenjamin/Aegis-SSH-MCP.git
cd Aegis-SSH-MCP
go build -o aegis-ssh-mcp .
```

## How it works

For each command request, Aegis follows the same basic flow:

1. The MCP client asks Aegis to run a command.
2. Aegis checks the bearer token for that host endpoint.
3. Aegis parses the command.
4. Aegis rejects unsafe shell behavior such as chaining, redirects, and command substitution.
5. Aegis checks the command against the host's assigned rule profile.
6. If the command is allowed, Aegis opens a fresh non-interactive SSH session.
7. Aegis runs the command, captures the result, logs the attempt, and disconnects.

No persistent shell is handed to the agent.

<details>
<summary>Architecture view</summary>

```text
+-------------------+
| MCP Client / LLM  |
| Claude / OpenAI   |
| LibreChat / SSE   |
+---------+---------+
          |
          | MCP over HTTP/SSE or stdio
          |
+---------v---------+
| Aegis-SSH-MCP     |
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

## Core concepts

### Configs can be fixed hosts or dynamic profiles

Each JSON file in [`configs/`](configs/) describes either one fixed remote host or one dynamic SSH profile.

A fixed host config creates:

- one MCP endpoint
- one host-scoped SSH tool
- one assigned rule profile
- one bearer-token boundary for SSE

For example, a host with alias `docker` becomes:

```text
/mcp/docker/sse
```

If one agent needs access to two hosts, add Aegis twice in the MCP client: one endpoint and one token per host alias.

A dynamic profile uses the same rule and SSH execution engine, but the MCP tool call supplies the host:

```json
{
  "config_type": "dynamic",
  "alias": "linux-dynamic",
  "ssh_user": "ops",
  "auth_method": "key",
  "key_path": "/keys/linux-dynamic.pem",
  "rule_profile": "readonly-safe",
  "api_keys": [
    "change-me-linux-dynamic-key"
  ]
}
```

That profile creates `aegis_ssh_linux-dynamic` with two required arguments:

```json
{
  "host": "192.168.1.42",
  "command": "uptime"
}
```

### Rules decide what can run

Each host points to a rule profile:

```json
"rule_profile": "docker-readonly"
```

Rule profiles live in [`rules/`](rules/) and define which command shapes are allowed or blocked before SSH is attempted.

Starter profiles include:

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

### Validation happens before SSH

Aegis validates commands before it connects to the remote host.

The validation flow is:

1. Parse the command into executable and arguments.
2. Reject shell control features such as redirects, chaining, and command substitution.
3. Allow only a limited set of safe pipeline filters.
4. Apply executable, argument, and full-command blacklist checks.
5. Apply executable, argument, and full-command whitelist checks.
6. Attempt SSH only if the command passes validation.

## Security model

Aegis uses defense in depth. It is not one magic security layer; it is several smaller boundaries working together.

| Boundary | What protects it | Who owns it |
| :--- | :--- | :--- |
| Client to Aegis | Bearer token per host alias, optional TLS/HTTPS | Aegis / operator |
| Aegis runtime | Shell-less distroless container running as nonroot | Aegis |
| Command checks | Parsing, shell feature rejection, argument checks, restricted pipeline filters | Aegis |
| Aegis to host | Short-lived SSH sessions and host fingerprint pinning | Aegis / operator |
| Remote host | Linux permissions, sudoers, auditd, journald, host hardening | Host operator |

Recommended production posture:

- use dedicated least-privileged SSH users
- pin SSH host keys with `host_key_fingerprint`
- use narrow rule profiles first
- enable TLS or run behind a trusted reverse proxy
- rotate bearer tokens regularly
- collect Aegis logs centrally
- keep sudo policy explicit and minimal

For the deeper threat model, see [`docs/security.md`](docs/security.md).

## Optional host settings

Host configs can also include:

- `stealth_mode`: returns a normal-looking fake response for blocked commands
- `fake_response`: custom response used when `stealth_mode` is enabled
- `redaction_enabled`: masks matching output before results are returned
- `redaction_patterns`: regex patterns used for output redaction
- `host_key_fingerprint`: recommended SSH host key pinning

## Client example: LibreChat

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

## Documentation

The README is meant to help you understand and try Aegis quickly. The deeper docs are here:

- [`docs/security.md`](docs/security.md): threat model, validation logic, pipeline handling, container hardening, and SSH session behavior
- [`docs/config.md`](docs/config.md): host config fields, hot reload behavior, aliases, API keys, and key paths
- [`docs/rules.md`](docs/rules.md): rule profile design, whitelists, blacklists, argument constraints, and starter profiles
- [`docs/FAQ.md`](docs/FAQ.md): common questions and operational notes
- [`docs/tech-specs/aegis-ssh-mcp-tech-spec.md`](docs/tech-specs/aegis-ssh-mcp-tech-spec.md): deeper implementation details
- [`docs/readme-authoring.md`](docs/readme-authoring.md): README authoring guidance for this repo

## Screenshots

<details>
<summary>Open screenshots</summary>

![Aegis project screenshot showing the README hero presentation and key project messaging](https://github.com/user-attachments/assets/6af3ab55-ca74-4a38-9ffa-799ce545a9f4)
![Aegis project screenshot showing a longer walkthrough of project details and configuration content](https://github.com/user-attachments/assets/034e35ff-b53b-4619-9c32-c9bc8cd20704)

</details>

## Project status

Aegis has an early-stage API and an operational runtime.

Current capabilities include:

- MCP over HTTP/SSE
- MCP over stdio
- multi-host configuration
- rule-based command validation
- audit logging
- SSH key authentication
- password authentication
- host fingerprint pinning
- ephemeral per-request SSH execution
- hardened shell-less distroless container runtime
- optional output redaction
- hot reload for config and rule changes

## Support and contributions

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
