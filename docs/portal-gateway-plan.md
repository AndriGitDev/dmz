# Portal and Gateway: first preview plan

Status — 27 October 2026: deferred under [ADR 0005](adr/0005-free-community-project.md). This is a historical implementation proposal, not active work or a delivery commitment. ADR 0004 applies only if this scope is revived.

Status: planning only · 28 September 2026. Architecture direction accepted in [ADR 0004](adr/0004-portal-hosting-and-pii-policy.md). Portal/Gateway are not implemented. All initial end-to-end tests use fabricated data.

## Sequence and acceptance gates

1. **Verify a Bedrock route.** Confirm credit eligibility/expiry, account permissions, regional endpoint and model/profile availability. Treat `eu-north-1` as provisional. Record the exact model, API, profile destinations, effective retention configuration and contractual evidence. Test a small synthetic invocation and stream, with bounded output. Verify body logging is disabled. No automatic global-region or retention fallback.
2. **Implement the shared Go gateway.** Authenticate every call; bind each conversation to its owner/tenant and immutable policy mode. Start with one account and one allowlisted model. Implement both policy paths behind server-side configuration: mask/restore, and approved original-data processing (disabled without evidence). Check all outbound context, not just the newest message. Add cancellation, bounded memory/session TTL, request/token/concurrency limits, metadata-only accounting and conservative cost reservation before dispatch. Reject unsupported content and unapproved routes before invocation. Policy changes require a new session.
3. **Add SvelteKit Portal.** Login, text-only chat, streaming, clear-session control, model/region/policy disclosure and usage display. Keep history and mappings in memory for the preview; no persistent transcripts. Make the selected processing mode understandable before sending. AWS credentials remain server-side. Shield is not required for Portal.
4. **Exercise live workflows.** Test multi-turn and streaming, secret blocking, reset/expiry, cancellation, denied provider access, unavailable models, exhausted budget and restarts. Run isolation tests across users/sessions and verify forbidden requests never reach the provider. Inspect synthetic test traffic and logging configuration. In approved PII mode, synthetic original identifiers should reach only the authorised test route; in masking mode, annotated identifiers should be replaced. Known detector gaps remain release constraints.
5. **Add one external Gateway client.** Select the first actual client before committing to an API surface. Implement and test its supported message/streaming subset with scoped keys. Reject unsupported tools/files rather than forwarding them. Broader OpenAI/Anthropic compatibility, tools and multi-tenant administration follow separately.

## Initial scope

Existing Hetzner EU VPS, one Go process and a SvelteKit client; one Bedrock adapter (Converse/ConverseStream where supported by the chosen model), one allowlisted model, one authenticated account, synthetic prompts, no tools or attachments. No OpenBao or durable conversation-map dependency is needed for an in-memory preview. Encryption/key management and tenancy require a separate production design before durable mappings are introduced.

Credential blocking remains active in both modes. Approved PII processing must not reuse the current fixed mask-everything `Session.Process` policy unchanged: introduce explicit policy evaluation around the shared detector. Restoration runs only for transformed values. Failure of required policy evaluation blocks dispatch. Do not infer permission from an absence of detector findings.

## Proposed credit allocation

| Purpose | Proposed allocation |
| --- | ---: |
| Connection smoke tests | $5 |
| Integration/failure tests | $15 |
| Owner daily preview | $50 |
| Reserve | $30 |

These are planning allocations, not spending authorisation or guaranteed coverage. Model-specific prices and credit applicability must be verified before use. Cap requests and output tokens, reserve conservative estimated costs atomically before dispatch, settle against returned usage, and count retries. Billing alerts supplement application limits. A gateway budget cannot cap unrelated account spending or guarantee an exact AWS invoice ceiling.

## Before real customer PII

Confirm the customer's permitted purpose/data categories and processor arrangements; approve the exact route and retention evidence; verify deployment locations, backups and operator access; test isolation, deletion/expiry, log redaction and policy enforcement. No GA, blanket GDPR, Iceland-only or universal ZDR claims follow from Bedrock access or the Shield workflow checks.
