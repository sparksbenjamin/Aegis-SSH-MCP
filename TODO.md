#### Phase 1: Middleware Hardening (The "Aegis Core")

_Objective: Intercept calls, validate freshness, and generate the unique Trace Identifier._

|**Task**|**Implementation**|**Why (NSA Alignment)**|
|---|---|---|
|**Nonce/Timestamp Store**|Implement a short-lived (60s TTL) in-memory cache (e.g., Redis or `go-cache`) to track used nonces and request timestamps.|**Non-Replayability:** Prevents attackers from capturing and re-submitting an MCP tool call.|
|**Freshness Check**|The middleware rejects any request where `timestamp` is outside a 30s drift window or `nonce` already exists in the store.|**Integrity:** Ensures the request is current and unique; stops "lazy" replay attacks.|
|**UUID Generation**|On a successful validation, generate a cryptographically strong UUID (`Aegis-Req-ID`).|**Correlation:** Creates the anchor for the entire audit trail from LLM to System Call.|

#### Phase 2: Execution Propagation (The "Bridge")

_Objective: Pass the identity of the request from your Docker container to the target environment._

|**Task**|**Implementation**|**Why (NSA Alignment)**|
|---|---|---|
|**Env Variable Injection**|**SSH:** Pass `AEGIS_REQ=<UUID>` via the `ssh` command prefix (e.g., `AEGIS_REQ=... ssh host "cmd"`). **API:** Pass as an HTTP header `X-Aegis-Req-ID: <UUID>`.|**Traceability:** Links the agent’s high-level command to the low-level system action without custom code on the target.|
|**Context Logging**|Log the `UUID` alongside the "Agent Intent" (User prompt/Tool goal) in your `docker logs` output.|**Accountability:** Provides the _reasoning_ for the action, which system logs alone cannot provide.|

#### Phase 3: Infrastructure Visibility (The "Target Setup")

_Objective: Configure existing infrastructure to capture the metadata without deploying custom agents._

|**Task**|**Implementation**|**Why (NSA Alignment)**|
|---|---|---|
|**SSH Configuration**|Update `/etc/ssh/sshd_config` on targets to `AcceptEnv AEGIS_REQ`.|**Zero-Bespoke-Integration:** Allows the SSH protocol to securely carry the metadata tag into the shell environment.|
|**`auditd` Rule Updates**|Add an audit rule (e.g., `-a always,exit -F arch=b64 -S execve -k aegis_audit`) to capture process execution and the `AEGIS_REQ` variable.|**Forensics:** Bridges the semantic gap; proves _what_ the machine actually did in response to the specific request.|

#### Phase 4: Validation & Hardening

_Objective: Verify that the security controls actually work as intended._

|**Task**|**Implementation**|**Why (NSA Alignment)**|
|---|---|---|
|**Replay Simulation**|Attempt to manually replay a captured SSH command (with the same Nonce/Timestamp) against the Aegis middleware. Verify rejection.|**Verification:** Proves the anti-replay mechanism is functional.|
|**Log Correlation Test**|Execute a command, then search the central log aggregator for the `UUID`. Confirm it appears in both the Aegis Docker log and the target's `auditd` log.|**Completeness:** Ensures full audit coverage across the entire stack.|