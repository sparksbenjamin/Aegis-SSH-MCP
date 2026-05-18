# Aegis-SSH-MCP

<p align="center">
  <strong>A thin MCP-native SSH bridge for AI agents.</strong><br/>
  Controlled, host-scoped SSH execution with ephemeral sessions, rule validation, and audit logging.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go" />
  <img src="https://img.shields.io/badge/MCP-Compatible-blue?style=flat-square" />
  <img src="https://img.shields.io/badge/Docker-Supported-2496ED?style=flat-square&logo=docker" />
  <img src="https://img.shields.io/github/actions/workflow/status/sparksbenjamin/Aegis-SSH-MCP/docker-publish.yml?style=flat-square" />
  <img src="https://img.shields.io/github/license/sparksbenjamin/Aegis-SSH-MCP?style=flat-square" />
  <img src="https://img.shields.io/github/go-mod/go-version/sparksbenjamin/Aegis-SSH-MCP?style=flat-square" />
</p>

---

# What Is Aegis-SSH-MCP?

Aegis-SSH-MCP is a lightweight MCP (Model Context Protocol) server that enables AI agents and MCP-compatible clients to execute controlled SSH commands against predefined infrastructure targets.

It is intentionally designed as a **thin operational bridge** between MCP clients and standard SSH infrastructure.

Rather than replacing Linux security, SSH permissions, or existing infrastructure controls, Aegis operates *within* the trust boundaries that already exist on the target machine.

Aegis focuses on:

- Host-scoped command execution
- Rule-constrained operations
- Ephemeral SSH execution
- Audit visibility
- Operational simplicity
- Infrastructure predictability

---

# Why Aegis Exists

AI systems are becoming increasingly capable of assisting with:

- infrastructure diagnostics
- container troubleshooting
- deployment validation
- operational maintenance
- observability workflows
- service management
- internal automation

Most infrastructure-facing AI tooling today tends toward one of two extremes:

## Unsafe Raw Shell Access

Giving AI agents unrestricted shell execution with little isolation or operational control.

## Overengineered Infrastructure Platforms

Replacing existing SSH/Linux controls with entirely new orchestration layers, security models, and abstractions.

Aegis takes a different approach:

> Keep SSH authoritative.  
> Keep Linux authoritative.  
> Keep infrastructure ownership where it already belongs.

Aegis simply exposes controlled SSH execution through MCP.

---

# Core Philosophy

Aegis is intentionally designed around several principles:

- SSH remains the trust boundary
- Linux permissions remain authoritative
- Existing sudo policies remain authoritative
- Existing logging/audit systems remain authoritative
- Infrastructure operators remain in control
- AI access should be explicit and constrained
- Execution should remain isolated and predictable

This creates a system that is:

- operationally transparent
- easier to reason about
- easier to audit
- easier to deploy
- harder to misuse accidentally

---

# Architecture

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

---

# Key Features

## Host-Scoped MCP Tools

Each configured host becomes its own MCP tool.

Example:

```text
aegis_ssh_docker
aegis_ssh_proxmox
aegis_ssh_staging
```

Each host-scoped endpoint also exposes:

```text
aegis_status
```

This creates explicit infrastructure boundaries and avoids unrestricted generic execution surfaces.

---

## Rule-Based Command Validation

Commands can be validated against configurable allow/deny rules before execution.

This enables restriction of:

- destructive commands
- shell chaining
- unsupported utilities
- risky patterns
- unexpected workflows

without replacing native Linux permissions.

Detailed configuration examples are available in:

- [`docs/rules.md`](docs/rules.md)

Bundled examples now include Docker, Kubernetes, and distro-oriented Debian, Ubuntu, and RHEL read-only and ops profiles.

---

## Ephemeral Execution Model

Aegis intentionally avoids persistent SSH sessions.

Each MCP request:

1. validates the command
2. establishes a fresh SSH connection
3. executes the command
4. captures output
5. disconnects immediately

This design intentionally avoids:

- persistent shell state
- terminal multiplexing abuse
- long-lived agent sessions
- hidden remote execution contexts
- interactive session hijacking
- cross-request shell persistence

The tradeoff is slightly higher execution latency in exchange for improved isolation and operational predictability.

This behavior is intentional and considered a core security property of the project.

---

## Standard SSH Authentication

Supports:

- SSH key authentication
- password authentication

Optional:

- SSH host fingerprint pinning

Aegis does not replace existing infrastructure authentication models.

Configuration examples are available in:

- [`docs/config.md`](docs/config.md)

---

## Audit Logging

Aegis logs execution activity for operational visibility and troubleshooting.

Combined with existing infrastructure tooling such as:

- journald
- auditd
- SSH logs
- sudo logs

this provides layered observability without replacing existing host auditing systems.

---

## Multi-Host Infrastructure Support

Manage multiple infrastructure targets from a single MCP server instance.

Each host remains isolated through:

- dedicated configuration
- dedicated MCP tool naming
- independent SSH credentials
- per-host bearer tokens for SSE access

---

## Lightweight Deployment

Aegis is designed for lightweight deployment as:

- a single Go binary
- a Docker container
- an internal operational service

No external database or orchestration layer is required.

---

# What Aegis Does NOT Do

Aegis intentionally avoids becoming:

- a replacement for SSH
- a Linux RBAC system
- a persistent shell runtime
- a terminal multiplexer
- a Kubernetes-style orchestrator
- a PAM replacement
- an autonomous infrastructure agent
- a hidden background execution layer

Aegis does NOT:

- bypass Linux permissions
- override sudo policies
- replace audit systems
- maintain persistent SSH sessions
- support SSH agent forwarding
- parse `~/.ssh/config`
- provide OS-level sandboxing
- maintain hidden remote shell state
- provide unrestricted interactive terminal access

These constraints are intentional.

The goal is operational transparency and predictable infrastructure behavior.

---

# Current Capabilities

| Capability | Status |
|---|---|
| MCP over HTTP/SSE | Supported |
| MCP stdio support | Supported |
| Docker deployment | Supported |
| Multi-host configuration | Supported |
| Concurrent requests | Supported |
| Rule-based validation | Supported |
| Audit logging | Supported |
| SSH key authentication | Supported |
| Password authentication | Supported |
| SSH fingerprint pinning | Supported |
| Ephemeral per-request SSH execution | Supported |
| Persistent SSH sessions | Intentionally unsupported |
| Streaming terminal sessions | Intentionally unsupported |
| SSH agent forwarding | Unsupported |
| `~/.ssh/config` support | Unsupported |
| File upload/download tooling | Unsupported |

---

# Example Use Cases

## Infrastructure Diagnostics

```text
"Check disk usage on the Docker host"
"Show nginx logs from staging"
"Verify the Proxmox node is reachable"
```

---

## AI-Assisted Operations

```text
"Restart the failing container"
"Inspect Docker compose state"
"Check systemd service health"
```

---

## Controlled Internal Automation

```text
"Run approved maintenance commands"
"Collect operational telemetry"
"Validate infrastructure state"
```

---

# Documentation

Detailed operational documentation is available in the `/docs` directory.

| Document | Purpose |
|---|---|
| [`docs/config.md`](docs/config.md) | Host configuration, authentication, bearer tokens, deployment options, and runtime configuration |
| [`docs/rules.md`](docs/rules.md) | Rule validation, command restrictions, and execution control behavior |
| [`docs/FAQ.md`](docs/FAQ.md) | Design philosophy, security boundaries, operational behavior, and common questions |

These documents intentionally go deeper into operational behavior and infrastructure constraints than the README alone.

---

# Installation

## Docker Compose (Recommended)

The checked-in [docker-compose.yml](docker-compose.yml) is the recommended deployment path.
It pulls the published GHCR image and mounts local `configs/`, `rules/`, and `keys/` directories into the container.

```bash
docker compose pull
docker compose up -d
docker compose logs -f aegis-ssh-mcp
```

By default, the checked-in Compose example serves local HTTP SSE on:

```text
http://localhost:8443
```

If you want HTTPS, uncomment the TLS lines in [docker-compose.yml](docker-compose.yml) and set `AEGIS_SSE_BASE_URL` to your real external address.

Minimal host folder layout:

```text
./configs
./rules
./keys
./certs   (only if you want HTTPS)
```

Additional deployment notes are available in:

- [`docs/config.md`](docs/config.md)

---

## Build From Source

```bash
git clone https://github.com/sparksbenjamin/Aegis-SSH-MCP.git

cd Aegis-SSH-MCP

go build -o aegis-ssh-mcp .
```

---

# Example Configuration

Hosts are configured through JSON configuration files.

Example:

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

Configured hosts automatically become MCP tools:

```text
aegis_ssh_docker
```

The endpoint also exposes:

```text
aegis_status
```

And for SSE deployments, that same config becomes:

```text
Endpoint: /mcp/docker/sse
Header: Authorization: Bearer change-me-docker-key
```

Additional configuration examples are available in:

- [`docs/config.md`](docs/config.md)

---

# Security Model

Aegis is designed around inherited infrastructure security.

Security boundaries are enforced primarily through:

- SSH authentication
- Linux user permissions
- sudo configuration
- host isolation
- command validation rules
- bearer-authenticated MCP access
- audit logging
- ephemeral execution isolation

Aegis should be treated as:

> a controlled SSH execution bridge

not as a complete infrastructure security platform.

Operators remain responsible for:

- credential management
- host hardening
- sudo policy
- network security
- audit retention
- infrastructure segmentation

---

# Recommended Deployment Scenarios

Aegis works best in:

- homelabs
- internal infrastructure
- trusted operational environments
- AI-assisted DevOps workflows
- diagnostics environments
- lightweight infrastructure automation

---

# Project Goals

The goal of Aegis is not maximum abstraction.

The goal is:

- operational simplicity
- explicit infrastructure boundaries
- MCP-native compatibility
- isolated execution
- minimal hidden behavior
- predictable SSH semantics
- infrastructure transparency

---

# Development Status

Aegis is actively evolving and should currently be considered:

> early-stage but operational

The project already supports:

- live SSH execution
- Docker deployment
- MCP over SSE
- multi-host operation
- rule validation
- audit logging
- isolated ephemeral execution

Additional capabilities will evolve over time as the MCP ecosystem matures.

---

# Contributing

Contributions, issues, and operational feedback are welcome.

Areas of interest include:

- MCP client interoperability
- improved rule validation
- enhanced observability
- deployment hardening
- additional transport support
- operational tooling
- testing and validation

---

# License

MIT License

---

# Final Philosophy

Aegis intentionally stays close to existing infrastructure primitives.

It does not attempt to replace SSH.

It does not attempt to replace Linux security.

It simply makes SSH infrastructure accessible to MCP-compatible AI systems in a more controlled, observable, isolated, and operationally predictable way.
