# Threat model: initial scope

Status: outline, not a completed assessment.

## Assets

- Real identifiers in prompts and responses.
- Per-tenant, per-session stand-in maps and encryption keys.
- Tenant policies and audit metadata.

## Trust boundaries

1. Shield: device-local detector and vault → third-party AI website.
2. Portal/Gateway: client → Iceland-hosted dmz core → selected model provider.
3. Audit: request metadata → tenant audit store. Prompt and response bodies must never enter this path.

## Initial failure cases to test

- Detector misses an identifier, including inflected Icelandic names.
- Stand-in mapping leaks across tenants or sessions.
- Streaming response exposes partial stand-ins or restores the wrong value.
- Logs, exceptions, traces or support tooling record prompt content.
- Browser site change bypasses Shield interception or permits file upload.
- Provider routing violates a tenant policy or retention constraint.

The formal threat model will add abuse cases, controls, test evidence, owners and residual risk before a paid pilot processes live data.
