# Threat model: initial scope

Status: outline, not a completed assessment.

## Assets

- Real identifiers in prompts and responses.
- Per-tenant, per-session stand-in maps and encryption keys.
- Tenant policies and audit metadata.

## Trust boundaries

1. Shield: device-local detector and vault → third-party AI website.
2. Portal/Gateway: client → approved dmz deployment (initially Hetzner EU) → approved model route; original PII reaches the provider only under the explicit policy in ADR 0004.
3. Audit: request metadata → tenant audit store. Prompt and response bodies must never enter this path.

The current Shield adapter intercepts recognised submissions from a third-party
composer. The original draft already exists in that page's DOM, where its scripts
can read the text and input events. The [page-managed input fixture](../apps/shield/scripts/chat-smoke.mjs)
models this access before checking that the intercepted chat request contains the
masked value. Device-local detection and the isolated restored-response frame do
not hide the original draft from the host page or protect pre-submit telemetry.
This is an architectural boundary, not a claim that either supported site sends
such telemetry. A composer controlled by the extension would be a separate design
and usability decision; the current preview does not provide it.

## Initial failure cases to test

- Detector misses an identifier, including inflected Icelandic names.
- Stand-in mapping leaks across tenants or sessions.
- Streaming response exposes partial stand-ins or restores the wrong value.
- Logs, exceptions, traces or support tooling record prompt content.
- Browser site change bypasses Shield interception or permits file upload.
- Provider routing violates a tenant policy or retention constraint.

- User bypasses the server-enforced mode or reuses a session under a different policy.
- An unavailable approved route silently falls back to a retaining or global route.
- Hosting, backups or provider processing exceed the approved residency boundary.
- Concurrent requests or retries bypass application spend reservations.

The formal threat model will add abuse cases, controls, test evidence, owners and residual risk before a paid pilot processes live data.
