# ADR 0004: Portal hosting and policy for personal data

Status: accepted direction · 28 September 2026 · implementation and provider verification pending

## Decision

The owner approved using existing Hetzner infrastructure in Germany or Finland for the initial Portal/Gateway. Icelandic hosting remains an option for customers with a specific residency, contractual or operational requirement. The actual VPS location, backups, operators and processor arrangements must be checked before deployment. An Icelandic gateway forwarding original data to an overseas model does not provide Iceland-only processing.

Portal provides controlled access to approved AI processing. Its shared Go gateway authenticates users, applies customer policy, restricts models and processing regions, enforces usage limits and emits metadata-only audit records. SvelteKit Portal is the first client; external API compatibility follows.

| Customer policy | Provider input | Local mapping/restoration |
| --- | --- | --- |
| Minimise identifiers | Text with detected identifiers replaced by visible markers | Required; tenant/session isolated |
| Approved PII processing | Necessary original personal data permitted by customer policy | Not required for unchanged input |

Approved PII processing is disabled until the exact model/account/API/region route has documented retention and processing-location evidence, applicable contractual approval, and tested enforcement. It is an explicit server-enforced entitlement, not an end-user bypass switch. Missing approval or an unavailable route blocks the request; never silently change retention, geography or policy mode. Both policies retain credential blocking and all other applicable customer restrictions. Detector findings support policy but do not establish a lawful basis or comprehensive compliance.

In masking mode, originals and mappings remain within the approved DMZ deployment boundary. In approved PII mode, original personal data necessarily reaches the selected provider for processing. ZDR concerns retention, not whether the provider processes the input. No universal claim that identifiers remain in Iceland or that all providers receive only masks applies to Portal/Gateway.

Shield remains device-local and continues to mask before supported consumer chat submissions. This decision does not enable raw PII submission in Shield.

## Supersession and constraints

This decision replaces the universal RunAtlas/Iceland requirement and unconditional provider-masking requirement for Portal/Gateway in the original product plan. The business plan remains a historical hypothesis baseline. ADR 0002 still defines the representation used whenever masking is selected. ADR 0001's public/private repository split and separate pilot-intake requirement are unchanged. Public marketing claims need separate review before publication.

Prompt/response bodies must not enter gateway logs, traces, audit, crash reports or durable storage. Provider invocation logging and downstream infrastructure must be checked as well. The first preview uses synthetic data, bounded in-memory sessions and no persistent chat history. It does not establish production readiness or change known detector limitations.

## Evidence and unresolved account details

The owner reports Bedrock access and $100 credits, with `eu-north-1` likely; no successful invocation, model choice, credit eligibility/expiry or verified retention configuration has been supplied. There is no spending or deployment authorisation in this decision.

Sources reviewed in the planning discussion on 28 September 2026:

- [Hetzner data protection](https://docs.hetzner.com/general/company-and-policy/data-protection-at-hetzner/): EU locations and processing information.
- [EDPB international transfers](https://www.edpb.europa.eu/sme/be-compliant/international-data-transfers_en): EEA transfer framework; hosting location alone does not settle all processing obligations.
- [Bedrock retention](https://docs.aws.amazon.com/bedrock/latest/userguide/data-retention.html): account/region/model-specific retention controls; verify the exact API route.
- [Bedrock geographic inference](https://docs.aws.amazon.com/bedrock/latest/userguide/geographic-cross-region-inference.html): geographic routing boundaries differ from global routing.
- [Bedrock invocation logging](https://docs.aws.amazon.com/bedrock/latest/userguide/model-invocation-logging.html): optional input/output logging must be checked separately.

See [implementation plan](../portal-gateway-plan.md).
