# Aegis FAQ

This FAQ answers the questions operators usually ask before giving an agent SSH access through Aegis.

Short version:

- Aegis is a controlled SSH gateway, not a full sandbox platform
- it narrows what an agent can do through per-host endpoints, rule profiles, and bearer-token boundaries
- it does not replace host hardening, least-privilege Unix accounts, or network controls

## Who owns the SSH session?

Aegis owns the SSH session.

The MCP client or agent does not get a raw SSH socket, terminal, or shell prompt.
For each allowed tool call, Aegis:

1. parses the requested command
2. validates it against the selected rule profile
3. opens a new SSH connection
4. runs one command in one non-interactive session
5. closes the session

There is no interactive shell handoff to the agent.

## How are keys stored?

SSH keys are stored on the Aegis host and mounted into the container.

Typical pattern:

- host path like `/opt/aegis/keys`
- mounted into the container as `/keys`
- referenced from host configs with `key_path`

Aegis loads the private key from disk when it needs it.
It does not hand the private key to the MCP client.

Important limits:

- Aegis does not encrypt the key file for you
- if you use password auth, the password is stored in the host config JSON as plain text
- key files must have restrictive permissions or Aegis will refuse to use them

## Are commands sandboxed?

Not in the OS-level sandbox sense.

Aegis is a command firewall, not a container sandbox or VM sandbox for remote commands.
It limits command execution by:

- parsing commands into executable plus arguments
- applying allowlist and blocklist rules
- rebuilding a normalized shell-safe command
- running one non-interactive SSH command only

But if you allow a dangerous command in the rule profile, Aegis will still run it.
The real security boundary is the combination of:

- the remote Unix account
- the selected rule profile
- the host itself

## Is sudo allowed?

Only if you allow it.

Aegis does not have a global hardcoded "sudo is always forbidden" switch.
The bundled safer profiles generally block `sudo`, but a custom rule profile could allow it.

Recommendation:

- do not allow `sudo` unless you have a very specific reason
- prefer a dedicated low-privilege service account on the target host

## Are commands logged?

Yes.

Aegis writes structured audit logs for command attempts.
The audit record includes:

- timestamp
- host alias
- command requested
- validation result
- validation reason
- exit code
- duration
- a short stdout summary

By default these logs go to `stderr`, which works well with Docker logging and log shippers.

## Can agents exfiltrate secrets?

Aegis reduces that risk, but it does not eliminate it automatically.

Important truth:

- if a rule profile allows reading sensitive files or commands that print secrets, the agent can ask for that data
- if the remote account already has access to secrets, Aegis cannot change that fact

Aegis helps by:

- narrowing access per host
- narrowing allowed commands with rule profiles
- supporting optional output redaction patterns
- blocking a lot of shell abuse patterns

But output redaction is not a complete data-loss-prevention system.

Best practice:

- use a dedicated least-privilege remote account
- use tight allowlists
- avoid allowing broad file reads
- enable redaction where it makes sense
- treat Aegis as one layer, not the only layer

## Is there RBAC?

Not full RBAC in the IAM sense.

What Aegis has today for remote HTTP/SSE use:

- one endpoint per host config
- one or more bearer tokens per host config
- tool visibility limited to that host endpoint
- command execution limited to that same host

That gives you host-scoped access control.

What it does not currently have:

- users, groups, and roles
- per-command permissions by user identity
- policy inheritance
- time-based approval roles

## Is there approval gating?

Not built in.

Aegis does not currently pause a command and wait for a human to approve it.
If a command matches the rule profile, Aegis runs it.
If it does not match, Aegis blocks it.

If you need approval gating, that has to come from:

- the MCP client
- an external orchestrator
- or a future Aegis feature

## Are commands replayable or auditable?

They are auditable.

Aegis keeps enough information to answer questions like:

- what command was requested
- whether it passed validation
- whether execution failed
- how long it took
- what host it targeted

What it does not currently do:

- store a full signed transcript
- keep a durable command replay queue
- record full stdout and stderr forever

So the answer is:

- auditable: yes
- replayable: not as a built-in workflow

## Is there host isolation?

Yes, for SSE deployments.

Each host config becomes:

- its own endpoint path
- its own SSH tool
- its own bearer-token boundary

Example:

- `/mcp/docker/sse`
- `/mcp/proxmox-node/sse`

A token for one host is not allowed to call another host's endpoint.

Important nuance:

- in `stdio` mode there is no HTTP bearer-token boundary, so all tools are visible to the local client process

## Is there session persistence?

Not for SSH command execution.

Each tool call creates a fresh SSH session and closes it when the command finishes.
There is:

- no interactive terminal
- no PTY
- no shell state carried from one command to the next

For SSE transport, Aegis does keep a temporary in-memory MCP session mapping for auth continuity, but that is not the same thing as a persistent remote shell.

If Aegis restarts:

- those SSE session bindings are lost
- clients reconnect and create new ones

## Are shell injections mitigated?

Yes, to a meaningful degree, but you should think of this as mitigation, not magic.

Aegis does not validate a raw shell string and then execute that exact raw shell string.
Instead it:

- parses the request into tokens
- validates executable and arguments separately
- rebuilds a normalized shell-safe command form
- blocks many common shell chaining and expansion patterns through rules

This is a strong improvement over naive string matching.

Still, the real safety depends on the rule profile.
If you allow broad executables or dangerous argument shapes, you weaken that protection.

## What should I rely on Aegis for?

Rely on Aegis for:

- host-scoped MCP access boundaries
- controlled SSH command execution
- rule-based allow/deny checks
- audit logging
- better protection against shell injection tricks than raw shell bridging

Do not rely on Aegis alone for:

- full endpoint security
- secret isolation
- human approvals
- full RBAC
- remote OS sandboxing

## Related Docs

- [README.md](../README.md)
- [docs/config.md](config.md)
- [docs/rules.md](rules.md)
- [docs/tech-specs/aegis-ssh-mcp-tech-spec.md](tech-specs/aegis-ssh-mcp-tech-spec.md)
