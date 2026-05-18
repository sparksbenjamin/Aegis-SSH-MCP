# Aegis Rule Guide

This guide explains how to create custom rule profiles for Aegis-SSH-MCP and how those rules are applied at runtime.

## What a Rule Profile Does

Each host config points at one rule profile with:

```json
"rule_profile": "readonly-safe"
```

That profile tells Aegis which commands are allowed, which are blocked, and which command shapes are suspicious enough to reject before SSH is attempted.

Rules are stored as JSON files in the `rules/` directory.

## Bundled Profiles

The repo ships with these starter profiles:

- `readonly-safe` for general Linux read-only inspection
- `debian-readonly` for Debian package, service, and host inspection
- `debian-ops` for Debian package updates and service operations
- `ubuntu-readonly` for Ubuntu package, snap, service, and host inspection
- `ubuntu-ops` for Ubuntu package, snap, and service operations
- `rhel-readonly` for RHEL-like package, service, and host inspection
- `rhel-ops` for RHEL-like package updates and service operations
- `proxmox-readonly` for Proxmox VE inventory, cluster, guest, and storage inspection
- `proxmox-ops` for Proxmox VE guest lifecycle operations and service control
- `docker-readonly` for Docker host and container inspection
- `docker-ops` for Docker administration on a Docker host
- `systemd-ops` for service restarts, status checks, and journal access
- `kubernetes-readonly` for safe `kubectl` inspection and log retrieval
- `network-diagnostics` for routing, sockets, DNS, and reachability checks
- `logs-readonly` for `journalctl` and `/var/log` inspection
- `package-readonly` for installed-package and repo metadata queries

## How Aegis Applies Rules

Before Aegis runs any SSH command, it does this:

1. Parse the requested command into an executable and arguments.
2. Allow limited pipelines through safe text filters such as `grep`, `head`, `tail`, `sort`, `uniq`, `wc`, `cut`, and `tr`.
3. Reject shell control features such as redirects, chaining, and command substitution.
4. Rebuild the parsed command into a normalized shell-safe command string.
5. Load the rule profile named in the host config.
6. Apply blacklist checks first.
7. Apply whitelist checks second.
8. Only run the SSH command if all checks pass.

If any check fails, SSH is never attempted.

## Rule File Format

A rule file looks like this:

```json
{
  "profile_name": "readonly-safe",
  "executable_whitelist_regex": [
    "^ls$",
    "^cat$"
  ],
  "arguments_whitelist_regex": [
    "^(|/etc/hostname)$"
  ],
  "whitelist_regex": [
    "^ls(\\s|$)",
    "^cat\\s+/etc/hostname$"
  ],
  "executable_blacklist_regex": [
    "^python$"
  ],
  "arguments_blacklist_regex": [
    "(^|\\s)--privileged(\\s|$)"
  ],
  "blacklist_regex": [
    ";",
    "&&",
    "\\$\\("
  ]
}
```

## Supported Fields

### `profile_name`

Required.
This is the name referenced by `rule_profile` in a host config.

### `executable_whitelist_regex`

Optional.
Regex patterns matched against the parsed executable only.

Example:

```json
"executable_whitelist_regex": [
  "^docker$",
  "^systemctl$"
]
```

Use this as your main allowlist boundary whenever possible.

### `executable_blacklist_regex`

Optional.
Regex patterns matched against the parsed executable only.

Example:

```json
"executable_blacklist_regex": [
  "^python$",
  "^bash$"
]
```

### `arguments_whitelist_regex`

Optional.
Regex patterns matched against the normalized argument string only.

This is useful when the executable is allowed, but only certain flags or subcommands should pass.

Example:

```json
"arguments_whitelist_regex": [
  "^ps\\s+(aux|ax|ef)$"
]
```

### `arguments_blacklist_regex`

Optional.
Regex patterns matched against the normalized argument string only.

Example:

```json
"arguments_blacklist_regex": [
  "(^|\\s)--privileged(\\s|$)",
  "(^|\\s)-v\\s+/:/"
]
```

### `whitelist_regex`

Optional.
Regex patterns matched against the full normalized command string.

Example:

```json
"whitelist_regex": [
  "^docker\\s+ps(\\s|$)",
  "^systemctl\\s+status\\s+docker$"
]
```

This is useful for matching the full command shape after parsing.

### `blacklist_regex`

Optional.
Regex patterns matched against the full normalized command string.

Example:

```json
"blacklist_regex": [
  ";",
  "&&",
  "\\|\\|",
  "\\$\\("
]
```

This is where you block shell chaining, redirects, or dangerous full-command patterns.

## Validation Order

This is the exact order Aegis uses:

1. `executable_blacklist_regex`
2. `arguments_blacklist_regex`
3. `blacklist_regex`
4. `executable_whitelist_regex`
5. `arguments_whitelist_regex`
6. `whitelist_regex`

Important behavior:

- Blacklists always run before whitelists.
- If a whitelist section exists and nothing matches, the command is blocked.
- If a whitelist section is empty or omitted, that layer does not restrict anything.

## What the Rules Match Against

Aegis parses a command into three views:

- executable
- normalized arguments
- normalized full command

Example input:

```text
docker   ps   --format '{{.Names}}'
```

The parser turns that into a normalized form before rules are checked.

That means your regexes should be written for the normalized command shape, not for weird spacing, quoting tricks, or shell expansion behavior.

## Good Authoring Strategy

The safest pattern is:

1. Use `executable_whitelist_regex` to keep the allowed executable set small.
2. Use `arguments_whitelist_regex` to limit flags or subcommands.
3. Use `whitelist_regex` when you need to constrain the full command shape.
4. Use blacklists to catch broad escape hatches and shell operators.

## Example Profiles

### Read-only Linux profile

```json
{
  "profile_name": "my-readonly-linux",
  "executable_whitelist_regex": [
    "^hostname$",
    "^whoami$",
    "^uname$",
    "^df$"
  ],
  "whitelist_regex": [
    "^hostname(\\s|$)",
    "^whoami(\\s|$)",
    "^uname(\\s|$)",
    "^df(\\s|$)"
  ],
  "blacklist_regex": [
    ";",
    "&&",
    "\\|\\|",
    ">",
    "\\$\\("
  ]
}
```

### Docker inspection profile

```json
{
  "profile_name": "my-docker-readonly",
  "executable_whitelist_regex": [
    "^docker$"
  ],
  "whitelist_regex": [
    "^docker\\s+(ps|images|inspect|logs)(\\s|$)",
    "^docker\\s+compose\\s+(ps|logs|config)(\\s|$)"
  ],
  "arguments_blacklist_regex": [
    "(^|\\s)--privileged(\\s|$)",
    "(^|\\s)-v\\s+/:/"
  ],
  "blacklist_regex": [
    ";",
    "&&",
    "\\|\\|",
    "\\$\\("
  ]
}
```

## Common Mistakes

### Making a whitelist too broad

Bad:

```json
"whitelist_regex": [
  "^docker\\s+"
]
```

This allows far more than most people intend.

Better:

```json
"whitelist_regex": [
  "^docker\\s+(ps|images|inspect|logs)(\\s|$)"
]
```

### Forgetting that regexes are exact patterns

If you want to match the whole executable, anchor it:

```json
"executable_whitelist_regex": [
  "^ls$"
]
```

Not:

```json
"executable_whitelist_regex": [
  "ls"
]
```

### Relying only on a blacklist

Blacklists help, but the safest rule sets use a tight whitelist and then layer blacklists on top.

## Hot Reload Behavior

Rule files are hot-reloaded from the `rules/` directory.

That means:

- editing a rule file updates the active rule set
- removing a rule file removes that profile from memory
- invalid JSON or invalid regex patterns cause rule loading to fail

If a rule file fails to load, fix the syntax and save it again.

## Troubleshooting

If a command is blocked:

1. Check which `rule_profile` the host config uses.
2. Look at the Aegis error message. It tells you which layer failed.
3. Compare the requested command to the normalized command shape your regex expects.
4. Tighten the allowlist first, then add blacklist coverage for obvious escape paths.

## Recommended Workflow for New Rules

1. Start from an existing rule file like `readonly-safe.json` or `docker-readonly.json`.
2. Rename `profile_name`.
3. Remove anything you do not want to allow.
4. Test a few expected good commands.
5. Test a few obviously bad commands.
6. Assign the new profile name to the target host config.

If you are building for a mainstream Linux server, the distro starter profiles are a good first base:

- Debian and Ubuntu profiles show `apt`, `dpkg`, `systemctl`, and `journalctl` patterns
- Ubuntu adds `snap` examples
- RHEL profiles show `dnf`, `yum`, `rpm`, and SELinux inspection patterns
- Proxmox profiles show `pvesh`, `pvecm`, `qm`, `pct`, and `pvesm` patterns for virtualization hosts

## Related Files

- [README.md](../README.md)
- [docs/tech-specs/aegis-ssh-mcp-tech-spec.md](tech-specs/aegis-ssh-mcp-tech-spec.md)
- [rules/readonly-safe.json](../rules/readonly-safe.json)
- [rules/debian-readonly.json](../rules/debian-readonly.json)
- [rules/debian-ops.json](../rules/debian-ops.json)
- [rules/ubuntu-readonly.json](../rules/ubuntu-readonly.json)
- [rules/ubuntu-ops.json](../rules/ubuntu-ops.json)
- [rules/rhel-readonly.json](../rules/rhel-readonly.json)
- [rules/rhel-ops.json](../rules/rhel-ops.json)
- [rules/proxmox-readonly.json](../rules/proxmox-readonly.json)
- [rules/proxmox-ops.json](../rules/proxmox-ops.json)
- [rules/systemd-ops.json](../rules/systemd-ops.json)
- [rules/kubernetes-readonly.json](../rules/kubernetes-readonly.json)
