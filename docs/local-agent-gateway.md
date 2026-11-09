# Experimental local coding client adapter

This development prototype reuses dmz's basic detector to mask selected user-message text in supported model requests from Codex and Claude Code. It runs on the user's device. It is separate from the Chromium extension and does not change the extension's supported scope or release status.

The local gateway and CLI launcher are implemented. Installed CLI binaries have been exercised against synthetic local upstream servers. Desktop applications, real provider responses, account authentication, compaction and remote execution still need qualification. Use fabricated data during evaluation.

## Build and run

Use the Go version required by the repository's `go.mod` (Go 1.27 or later):

```sh
go build -trimpath -o /tmp/dmz-local-gateway ./cmd/dmz
/tmp/dmz-local-gateway version
```

The executable uses the basic detector only. It does not include the trained name scanner or a trained-build address vocabulary. Basic and trained detector coverage are different; no independent validation is established by these integration tests.

With the intended provider credentials already configured in the client environment:

```sh
/tmp/dmz-local-gateway run codex
/tmp/dmz-local-gateway run claude
```

Launcher options go before the client name. Client arguments go after it. For a fabricated non-interactive evaluation:

```sh
/tmp/dmz-local-gateway run codex exec --ephemeral 'Draft a reply to pilot@example.test'
/tmp/dmz-local-gateway run claude -p --no-session-persistence 'Draft a reply to pilot@example.test'
```

Codex API mode requires `OPENAI_API_KEY` and routes to the Responses API. `run --auth chatgpt codex` selects the documented ChatGPT-backend routing path, but that route has not been verified with an account. It is experimental and does not establish availability for any particular plan. Claude retains its existing client authentication. An existing custom upstream or alternate Claude backend is rejected rather than silently rerouted; corporate gateway chaining is not qualified.

The launcher starts an ephemeral `127.0.0.1` listener and generates a random local credential. It changes no existing client configuration files. Codex routing uses process-only overrides. Claude uses a private temporary settings file containing the local routing/token and any explicitly supplied session settings; it is removed on normal exit. Forced termination can leave that temporary file behind, but its local token is useless after the ephemeral listener exits. dmz does not read client authentication stores; it checks ordinary Claude routing settings locally and rejects conflicting routes/custom settings headers. Managed settings can still override the temporary layer and need separate qualification.

The client itself retains its normal behavior, including any local history or memory storage unless disabled through the client's own options. Model choice, approvals and sandbox policy are not changed by dmz. The Codex launch disables request compression and selects the HTTP transport advertised by this adapter; WebSocket behavior is not implemented. On Linux/macOS, cancellation first signals the client for graceful exit; non-interactive launches also clean up their process group. Native Windows process-tree cleanup and terminal behavior need qualification.

## What is masked

The adapter transforms only string values at explicit supported locations:

| Protocol | Eligible text |
| --- | --- |
| Responses | A string `input`, or user-role message content strings and `input_text` blocks inside `input` |
| Messages and token counting | User-role message content strings and `text` blocks inside `messages` |

These locations can contain client-generated user messages as well as typed text. The adapter cannot infer human authorship from a role field alone.

Instructions, system/developer messages, tool definitions, tool arguments, tool results, assistant messages, metadata, images, documents, and other attachments are forwarded unchanged. Signatures, encrypted reasoning, item IDs and bytes outside changed strings are preserved. Responses retain markers; there is no automatic display or tool-input restoration. A filename masked in a user message can therefore lead to an unusable tool argument.

Tool outputs commonly contain file contents. They can reintroduce an identifier that was masked in an earlier prompt. This prototype does not protect an entire coding session or all repository context.

The gateway implements the selected provider's model-request route, optional model listing, Responses compaction and, for Messages, token counting. Compaction applies the same user-text field allowlist and preserves opaque input/output state. This endpoint has synthetic contract tests; automatic compaction in the installed clients still needs qualification. Other routes such as provider file APIs are unsupported. The service is not an arbitrary HTTP proxy. Streaming responses and provider errors pass through; upstream redirects are rejected to avoid sending local credentials to another host. Standard environment proxy settings remain effective for upstream HTTPS connections.

## Mappings and limits

Original values and reverse mappings remain in process memory on this device. The gateway writes no prompt, response or mapping log; its operational status contains counts and fixed reason codes only. The launcher preserves the client's normal output and diagnostics, which can contain conversation data. This is not a defense against another process with access to the user's environment or memory.

Claude scopes mappings by the verified `X-Claude-Code-Session-Id` request header; subagent requests carrying the same root session share its map. Missing or ambiguous identity warns and forwards the request unchanged. Codex currently has one explicitly shared mapping scope per launcher invocation. Do not attach unrelated clients to a manually served Responses instance.

Mappings are discarded on exit or explicit reset. They are not persisted for resume. Resuming history after reset/restart may leave old markers unresolved or cause whole-field masking. Existing unknown/edited marker handling is retained: only markers issued by the current core session bypass detection. A marker-shaped string never disables scanning of adjacent text.

Default resource bounds:

- Four concurrent transformation slots, with up to 30 seconds to acquire a slot during a burst. Timeout forwards the request unchanged with a warning; cancellation cancels its upstream request as well.
- An 8 MiB request parsing limit. Larger requests stream unchanged with a warning.
- A 256 KiB limit per eligible text field.
- A 60-second request-body read deadline on launcher/manual listeners. Streaming responses are not subject to that body-read deadline.
- Up to 16 mapping scopes, each admitting up to 8 MiB of unique scanned text or 32,768 distinct text fields. Identical history replay does not spend that text budget again. At scope capacity the least recently used idle mapping is discarded with a warning so new sessions can still be masked. Returning to an evicted conversation loses its old map. These are input-accounting limits, not exact heap-size guarantees.
- JSON depth at most 256 and at most 100,000 value nodes.

Requests using compressed bodies, malformed/ambiguous JSON, duplicate keys, invalid Unicode or unsupported shapes are forwarded unchanged with fixed warnings. Text/scoping capacity failures and detector-processing failures also warn and continue; other eligible fields may still be masked. A running trained scanner is not part of this profile.

If the gateway process is unreachable, the client gets a connection error. That is a connectivity limitation, distinct from the detector's warn-and-continue policy. There is no silent direct-provider fallback. Exit the launcher and launch the client normally to leave this experimental route.

## Manual service and desktop evaluation

`serve` supports operator-controlled testing. It does not install or change desktop settings. Create a local token without using a provider key as that token:

```sh
export DMZ_LOCAL_TOKEN="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
```

Keep actual provider credentials out of shell history, repository files and reports. Supply them through your existing environment/credential workflow.

For a Responses service using `OPENAI_API_KEY` from its environment:

```sh
/tmp/dmz-local-gateway serve --protocol responses --local-auth bearer --listen 127.0.0.1:43127
```

For a Messages service using `ANTHROPIC_API_KEY` from its environment:

```sh
/tmp/dmz-local-gateway serve --protocol messages --local-auth bearer --listen 127.0.0.1:43128
```

The service consumes the local bearer credential and substitutes its in-memory provider key before forwarding. `--local-auth api-key` accepts the local credential in `x-api-key` instead. Default `header` mode requires `X-Dmz-Token` alongside the client's own provider authentication. A local token is stripped from requests to the provider, including accidental use in a provider-auth field. These manual modes use API keys; they do not convert desktop subscriptions into API access.

Generate fragments for review:

```sh
/tmp/dmz-local-gateway config --address http://127.0.0.1:43127 codex
/tmp/dmz-local-gateway config --address http://127.0.0.1:43128 claude
```

Merge the Codex fragment into the appropriate user-level configuration without duplicating existing TOML tables. Put `model_provider` at top level, before the first `[table]`; appending the entire fragment after an existing table would silently leave the old provider active. The desktop process must inherit `DMZ_LOCAL_TOKEN`; an application launched from a dock/menu may not inherit the shell environment. Retain the selected model. Restart and verify that requests actually reach dmz.

For Claude Desktop, review the third-party inference configuration in Developer → Configure Third-Party Inference. Use the local gateway address, bearer scheme and local dmz token. The generated JSON is a configuration fragment, not a complete OS-managed profile. Evaluate the Code tab only; Chat and Cowork are outside this prototype. Whether an installed desktop build accepts the local HTTP endpoint, sends the required session header and completes startup/model discovery must be verified on that device.

Keep the service terminal visible for warnings. There is no tray application or desktop notification integration. Stopping the service interrupts the configured client route; restore the previous client configuration explicitly before returning to normal use. Maps currently live until reset/exit; automatic idle expiry is not implemented.

## Status and reset

Use the same `DMZ_LOCAL_TOKEN` in the control-command environment:

```sh
/tmp/dmz-local-gateway status --address http://127.0.0.1:43127
/tmp/dmz-local-gateway reset --address http://127.0.0.1:43127
```

`status` returns aggregate counts and warning codes. It exposes neither mappings nor session identifiers. `reset` discards mappings; it cannot retroactively remove data from the client or provider. Start a new conversation after reset when old markers would otherwise remain in history. The HTTP control endpoints require local authentication and reject browser origins.

`instructions_unchanged`, `tool_result_unchanged` and `attachment_unchanged` describe coverage exclusions. Codes such as `mask_error`, `encoded_body_forwarded_unchanged`, `missing_session_identity_forwarded_unchanged` and capacity warnings mean a particular field/request bypassed masking. Once history exceeds a parsing/capacity limit, later requests can repeatedly forward originals; earlier masking does not protect those later requests. `whole_text_masked_context_may_degrade` reports conservative whole-field masking, including unknown-marker cases.

During a launcher run, routine instruction/tool-result/no-user-text counts are summary-only; other warning codes appear at most once each to limit disruption to an interactive client. Full per-code counts appear after the client exits and in service status. The launcher also warns if it observes no model-route traffic. A changed-parts count is not a completeness or accuracy score.

## Traffic outside the gateway

Direct MCP connections, shell networking, web tools, plugins/apps, telemetry, authentication, update checks, desktop synchronization, cloud sessions and remote execution can use other routes. Local client histories may contain originals. The local token can be inherited by tool subprocesses and could appear if an agent prints its environment; tool results are outside masking scope. This gateway does not prevent autosave, provide anonymity, guarantee detection or establish provider retention behavior.

A gateway bound to this computer's loopback interface is not available to an SSH host or cloud worker. Those paths require a separately designed deployment and must not be labelled as covered. Voice and file/image redaction are not implemented.

## Reproduce development checks

```sh
go test ./...
go test -race ./internal/agentgateway ./cmd/dmz ./packages/detect ./packages/surrogates
go vet ./...
go test ./internal/agentgateway -run '^$' -fuzz FuzzTransform -fuzztime 30s
python3 -I scripts/agent-client-smoke.py --dmz /tmp/dmz-local-gateway
```

The Python standard-library harness uses isolated client state directories, synthetic credentials, fabricated files and local fake providers. It does not read account credentials. It configures a refusing proxy for background traffic, which is a test aid rather than a system-wide network firewall. Reports contain versions and aggregate observations; request bodies and client output are not included. Exit 0 means pass, 3 means incomplete, 1 means failure, and 2 means the dmz executable was unusable.

The default Codex tool roundtrip can be incomplete on containers that cannot run its sandbox. Do not weaken a user's sandbox to obtain a passing report. An explicit `--codex-roundtrip-sandbox danger-full-access` fixture run is only an additional local development diagnostic for an already isolated environment; it is not evidence for normal sandbox behavior.

`GO_BIN=/path/to/go ./scripts/check-agent-gateway.sh /tmp/dmz-review` builds native and cross-compiled macOS ARM64/Windows AMD64 executables, runs Go tests/race/vet and bounded fuzzing, compares three existing detector baselines, and runs the default launcher/manual-service CLI fixtures. It writes local review artifacts, applicable notices and a source/artifact hash manifest. Exit 3 preserves an incomplete fixture result rather than treating it as passed. Cross-compilation is not native-platform qualification.

Keep the existing synthetic detector recall gates. Passing integration fixtures is not independent detector validation. Before distribution, review source availability, history, corresponding source, notices and asset rights under the repository's existing readiness process. No public download or general release is created by this prototype.

## Reference documentation

- [OpenAI gateway setup](https://learn.chatgpt.com/docs/enterprise/connect-to-a-gateway)
- [OpenAI gateway protocol compatibility](https://learn.chatgpt.com/docs/enterprise/gateway-compatibility)
- [OpenAI configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference)
- [Claude Code gateway setup](https://code.claude.com/docs/en/llm-gateway-connect)
- [Claude Code gateway protocol](https://code.claude.com/docs/en/llm-gateway-protocol)
- [Claude Desktop gateway configuration](https://claude.com/docs/third-party/claude-desktop/gateway)

These are integration references, not evidence that a particular installed version or account has passed qualification. Record exact versions and auth routes in the operator review.
