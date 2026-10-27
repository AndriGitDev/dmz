# Product specification: free community Shield

Active direction: [ADR 0005](adr/0005-free-community-project.md), 27 October 2026. Detailed prior hosted requirements are retained in the [historical specification](product-spec-commercial-baseline.md); they are not delivery commitments.

## Product and availability

DMZ is a free community tool from Kastro for reducing accidental sharing of selected direct identifiers and secrets with AI. Shield is the initial product, with Icelandic-language detection as a focus. There are no DMZ subscriptions, seat minimums, premium protection tiers or mandatory accounts. Optional Kastro assistance is separate. The AI service retains its own account requirements and costs.

Existing Chromium rule-based previews and a separately packaged trained alpha support selected text submissions on ChatGPT and Claude. They are not general store releases. Free access does not establish source/model publication rights or independent detector quality. Portal/Gateway, tenant services and server audit are deferred.

## Current flow

1. The user types into the AI site's composer. Page scripts can already observe the original draft and input events; Shield does not isolate this field or prevent pre-submit telemetry/autosave.
2. Supported submission paths invoke device-local detection. The Go core compiles to WASM; the separate trained build adds a local name scanner. Basic and trained profiles have different coverage. No remote inference is used by Shield.
3. Detected values become visible markers with per-tab, device-local mappings. Selected identifiers, uncertain findings and secrets are masked on a best-effort basis.
4. The AI site receives the submitted text under its own terms. Misses, unsupported paths, failed processing and unchanged attachments can expose original data.
5. Unchanged known markers in an assistant reply can be restored in an isolated extension-origin frame. The host page retains masked reply text. Site Copy/Share controls use the masked version; selection inside the frame can copy the local restored version.
6. Reset, reload or closing the chat tab clears that session's map. Old replies cannot then be restored. Theme preference is stored locally; no DMZ prompt/feedback telemetry or account is required.

## Submission policy

The owner-approved 9 October 2026 policy is authoritative: Shield attempts masking and **never deliberately blocks a submission**. Scanner, core, draft-transition and composer failures warn and continue with the applicable current draft. Trained-scanner failure falls back to basic masking with reduced name coverage. Files, images and attachments pass through unchanged with a warning; their contents are not redacted.

Earlier documents describing blocking, a separate manual side-panel map/dictionary, universal Iceland hosting or a guaranteed sensitive-work boundary are superseded. Strict shared-core callers retain their separate policies; do not change their behaviour as part of community positioning.

## Supported scope and limits

The preview targets selected text paths at `chatgpt.com` and `claude.ai`. Current fixture checks do not establish all authenticated live-site paths. Site changes and unsupported send paths can bypass interception. Other AI sites, mobile/desktop apps, voice, Firefox and document/image redaction are not verified features.

Selected kennitölur, Icelandic names in the trained profile, address forms, email, phone, IS IBAN, card numbers and secret patterns are covered to differing measured degrees. Follow the [quality contract](detector-quality-contract.md) for exact current evidence and the [preview guide](shield-autonomous-preview.md) for workflow limits. Do not imply exhaustive detection, anonymity, encryption of markers, legal compliance, certification or provider zero retention.

## Quality and releases

No release may lower recall for any entity type on the synthetic release benchmark. Existing synthetic gates and the owner's >=99% exact entity-set accuracy and >=99% recall research targets remain; the targets and independent acceptance are not achieved. Reused development results and browser fixtures are not representative field-quality evidence.

Use the existing Go/WASM, scanner, composer/race and browser regression suites for relevant changes. Preserve clean source/build identity, checksums and artifact-profile distinction. Keep private weights and research outside public website/release assets; publish reviewed summaries and limits only.

See the [distribution checklist](community-distribution.md), [community validation](validation.md) and [launch checklist](launch-checklist.md). No store listing or public download may be invented to satisfy a call to action. Existing private previews remain fabricated-data evaluation packages.

## Deferred architecture

The Go server, Portal, Gateway, encrypted tenant vault, audit export and model-provider routes are retained as scaffold/research only. ADR 0004's approved-boundary and provider-evidence rules apply if hosted work is revived. No consumer subscription-token proxying, prompt/response-body logging or unapproved provider dispatch is authorized by the community transition.
