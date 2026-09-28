# Validation and launch checklist

## Gate 1: 18 December 2026

- [ ] Submit the Rannís R&D project application by 1 October 2026, 23:59 Icelandic time, and retain the receipt. See the [2026 readiness note](rannis-2026-readiness.md); company and cost inputs remain missing.
- [ ] Apply for OpenAI and Anthropic ZDR by 16 October 2026. Do not use a ZDR claim until a signed agreement exists.
- [ ] Test Bedrock EU retention mode `none` in a real account by 16 October 2026.
- [ ] Confirm commercial BÍN rights and a Mannanafnaskrá export by 30 October 2026.
- [ ] Publish a synthetic Icelandic PII benchmark v0 by 13 November 2026.
- [ ] Demonstrate Shield on ChatGPT and Claude web by 27 November 2026.
- [ ] Meet Stafrænt Ísland about approved proxy patterns by 27 November 2026.
- [ ] Interview 5 vendors, 3 finance or insurance firms, and 2 municipal or health-adjacent bodies by 4 December 2026.
- [ ] Obtain 3 paid pilot commitments from 10 targets at list price by 18 December 2026.
- [ ] Count vendors delivering to Stafrænt Ísland; if fewer than 40, revise the financial model.

## Before publishing dmz-web

- [ ] Activate `pilot@dmz.is`, `hello@dmz.is`, and `security@dmz.is` on an approved mail system.
- [ ] Confirm the legal controller, privacy contact, message retention period, and final public privacy notice.
- [ ] Review marketing statements and legal pages with counsel.
- [ ] Configure dmz.is DNS and TLS, then check both language routes and security.txt.
- [ ] Replace the email pilot CTA with a form only after Iceland-hosted lead intake and retention controls are live.

## Before processing customer data

- [ ] DPA, sub-processor list and incident SLA reviewed by counsel.
- [ ] Threat model and DPIA template reviewed.
- [ ] Prove prompt and response bodies are absent from logs, traces and durable storage.
- [ ] Validate per-tenant encryption, expiry, and cross-tenant isolation.
- [ ] Publish measured recall for every supported entity type and gate releases on no regression.
