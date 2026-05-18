# Aegis Config Guide

This guide explains the JSON files in `configs/` and what each option does.

The important model is:

- one `.json` file = one remote host
- one host config = one MCP endpoint
- one host config = one host-scoped SSH tool
- one host config = one rule profile assignment
- one host config = one set of bearer tokens for that one host

## What a Host Config Controls

Each host config tells Aegis:

- which machine to connect to
- which SSH credential method to use
- which rule profile to enforce
- which bearer tokens can reach that host's MCP endpoint
- whether blocked commands should return normal errors or fake stealth responses
- whether returned output should be redacted

## Where Config Files Live

Host config files live in the `configs/` directory.

Examples:

- `configs/proxmox-node.json`
- `configs/dell-r820.json`

Only `.json` files are loaded from that directory.

## How Config Files Are Applied

Aegis watches the `configs/` directory and hot-reloads changes.

That means:

- creating a new config file creates a new host surface
- updating a config file updates that host without restarting Aegis
- deleting a config file removes that host from the live registry

Important behavior:

- one bad config file is skipped with a warning if it fails normal field validation
- duplicate host aliases fail the config sync
- duplicate endpoint aliases after sanitizing also fail the config sync
- duplicate bearer tokens across different hosts also fail the config sync

## Minimal Example

Key-based SSH example:

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
  "host_key_fingerprint": "SHA256:replace-this-with-your-real-host-key",
  "api_keys": [
    "change-me-my-server-token"
  ]
}
```

Password-based SSH example:

```json
{
  "alias": "lab-box",
  "host_ip": "192.168.1.55",
  "ssh_port": 22,
  "ssh_user": "ubuntu",
  "auth_method": "password",
  "password": "replace-me",
  "rule_profile": "readonly-safe",
  "timeout_seconds": 30,
  "host_key_fingerprint": "SHA256:replace-this-with-your-real-host-key",
  "api_keys": [
    "change-me-lab-token"
  ]
}
```

## Supported Fields

### `alias`

Required.

This is the human name for the host inside Aegis.

It is used for:

- the host identity inside the config registry
- the MCP endpoint path
- the SSH tool name

Example:

```json
"alias": "my-server"
```

That becomes:

- endpoint path: `/mcp/my-server/sse`
- tool name: `aegis_ssh_my-server`

Important behavior:

- `alias` must be unique across all config files
- Aegis also creates a sanitized endpoint alias for URLs and tool names
- spaces and punctuation are converted to `_`
- uppercase letters are lowercased

Examples:

- `Prod Box` becomes `prod_box`
- `web-01` stays `web-01`

Recommendation:

- use lowercase letters, digits, and hyphens only

That avoids collisions like:

- `prod box`
- `prod_box`

Those two would both sanitize to the same endpoint alias.

### `host_ip`

Required.

This is the SSH target hostname or IP address.

Example:

```json
"host_ip": "192.168.1.100"
```

### `ssh_port`

Optional.

Default:

```json
22
```

This is the TCP port used for SSH.

Example:

```json
"ssh_port": 2222
```

### `ssh_user`

Required.

This is the remote SSH username.

Example:

```json
"ssh_user": "root"
```

### `auth_method`

Required.

Allowed values:

- `key`
- `password`

Example:

```json
"auth_method": "key"
```

Behavior:

- if `auth_method` is `key`, `key_path` is required
- if `auth_method` is `password`, `password` is required

Recommendation:

- prefer `key` in production
- use `password` only when needed

### `key_path`

Required when `auth_method` is `key`.

This must be the path inside the container, not the host path.

Example:

```json
"key_path": "/keys/my-server.pem"
```

Important behavior:

- Aegis checks the key file permissions before using it
- the key file must not be group-readable or world-readable
- expected permissions are effectively `0600` or `0400`

### `password`

Required when `auth_method` is `password`.

Example:

```json
"password": "replace-me"
```

Important behavior:

- this is stored as plain text in the config file
- prefer key-based auth whenever possible

### `rule_profile`

Required.

This must match the `profile_name` of a rule file in `rules/`.

Example:

```json
"rule_profile": "readonly-safe"
```

If the rule profile does not exist, commands for that host will not validate correctly.

See also:

- [docs/rules.md](rules.md)

### `timeout_seconds`

Optional.

Default:

```json
30
```

This controls the SSH network timeout used when connecting to the host.

Example:

```json
"timeout_seconds": 45
```

Use a larger value if the host is slow to accept SSH connections.

### `stealth_mode`

Optional.

Default:

```json
false
```

When enabled, blocked commands return a fake normal-looking response instead of an Aegis block error.

Example:

```json
"stealth_mode": true
```

Important behavior:

- this only affects validation failures
- it does not hide real SSH connection or execution errors
- audit logs still record the blocked event

### `fake_response`

Optional.

Default behavior:

- if `stealth_mode` is on and `fake_response` is empty, Aegis returns `bash: command not found`

Example:

```json
"fake_response": "permission denied"
```

This is only used when:

- `stealth_mode` is `true`
- a command is blocked by the rule engine

### `redaction_enabled`

Optional.

Default:

```json
false
```

When enabled, Aegis applies the regexes in `redaction_patterns` to stdout and stderr before returning output to the MCP client.

Example:

```json
"redaction_enabled": true
```

Important behavior:

- redaction happens after the command runs
- redaction protects returned output, not the remote machine itself

### `redaction_patterns`

Optional.

This is a list of regex patterns used to replace matching output with `[REDACTED]`.

Example:

```json
"redaction_patterns": [
  "(?i)password[=:\\s]+\\S+",
  "(?i)token[=:\\s]+\\S+"
]
```

Important behavior:

- patterns only apply when `redaction_enabled` is `true`
- invalid regex patterns are skipped
- broad patterns can hide more output than you expect

### `host_key_fingerprint`

Optional, but strongly recommended.

Example:

```json
"host_key_fingerprint": "SHA256:abc123replace-this"
```

This pins the remote SSH host key.

Important behavior:

- if this value is set, Aegis verifies the remote host key exactly
- if this value is empty, Aegis logs a warning and falls back to insecure host-key verification

Recommendation:

- set this in production

### `api_keys`

Optional for `stdio`, but effectively required for `sse`.

This is the list of bearer tokens allowed to access this host's MCP endpoint.

Example:

```json
"api_keys": [
  "change-me-my-server-token",
  "optional-second-token-for-the-same-host"
]
```

Important behavior:

- Aegis trims whitespace around each token
- blank entries are removed
- duplicate entries inside the same config are deduplicated
- a token can belong to only one host config
- SSE startup requires at least one configured token somewhere in the config set
- if a host has no `api_keys`, nothing can authenticate to that host's SSE endpoint

Clients must send these values as:

```text
Authorization: Bearer YOUR_TOKEN
```

## Endpoint Mapping

Each host config creates one host-scoped endpoint.

Example:

```json
"alias": "my-server"
```

Produces:

```text
http://localhost:8443/mcp/my-server/sse
```

And if the alias needs sanitizing:

```json
"alias": "Prod Box"
```

Produces:

```text
http://localhost:8443/mcp/prod_box/sse
```

## Client Connection Examples

For a host config like:

```json
{
  "alias": "docker",
  "api_keys": [
    "change-me-docker-key"
  ]
}
```

the client should connect to:

```text
http://localhost:8443/mcp/docker/sse
```

and send:

```text
Authorization: Bearer change-me-docker-key
```

### Quick Reachability Check

```bash
curl -i -N \
  -H "Authorization: Bearer change-me-docker-key" \
  http://localhost:8443/mcp/docker/sse
```

### LibreChat Example

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

If you want one client or agent to access multiple hosts, add one MCP entry per host alias and token.

## Recommended Authoring Pattern

For most deployments:

1. use one config file per host
2. use a short lowercase alias like `web-01`
3. prefer `auth_method: "key"`
4. set a real `host_key_fingerprint`
5. assign a narrow `rule_profile`
6. give each host its own unique bearer token
7. enable redaction only when you know what output patterns you want to mask

## Common Mistakes

### Using host paths instead of container paths

Bad:

```json
"key_path": "C:\\Users\\me\\keys\\my-server.pem"
```

Better:

```json
"key_path": "/keys/my-server.pem"
```

The config must use the path visible inside the container.

### Reusing the same token across hosts

Bad:

```json
"api_keys": ["shared-token"]
```

for multiple different host configs.

Each host should have its own token set.

### Forgetting that aliases become endpoint paths

This works:

```json
"alias": "web-01"
```

This is legal but less predictable:

```json
"alias": "Web 01 (Prod)"
```

Use simple aliases unless you have a strong reason not to.

## Related Files

- [README.md](../README.md)
- [docs/rules.md](rules.md)
- [docs/tech-specs/aegis-ssh-mcp-tech-spec.md](tech-specs/aegis-ssh-mcp-tech-spec.md)
