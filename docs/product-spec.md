# Product specification: dmz

Active direction: [open-source extension](community-project.md), 31 October 2026.

## Product and availability

dmz is the free, open-source Chromium extension from Kastro, licensed under GPL-3.0-only. Local use requires no dmz account. Basic and separately packaged trained alpha builds support selected text submissions on supported AI sites, with different coverage. Neither is claimed as a general store release. External model/data rights and independent detector validation remain separate from source licensing.

## Current flow

1. The user types into the AI site's composer. Page scripts can already observe the original draft and input events; DMZ does not isolate this field or prevent pre-submit telemetry/autosave.
2. Supported submission paths invoke device-local detection. The Go core compiles to WASM; the separate trained build adds a local name scanner. Basic and trained profiles have different coverage. No remote inference is used by DMZ.
3. Detected values become visible markers with per-tab, device-local mappings. Selected identifiers, uncertain findings and secrets are masked on a best-effort basis.
4. The AI site receives the submitted text under its own terms. Misses, unsupported paths, failed processing and unchanged attachments can expose original data.
5. Unchanged known markers in an assistant reply can be restored in an isolated extension-origin frame. The host page retains masked reply text. Site Copy/Share controls use the masked version; selection inside the frame can copy the local restored version.
6. Reset, reload or closing the chat tab clears that session's map. Old replies cannot then be restored. Theme preference is stored locally; no DMZ prompt/feedback telemetry or account is required.

## Submission policy

The owner-approved 9 October 2026 policy is authoritative: DMZ attempts masking and **never deliberately blocks a submission**. Scanner, core, draft-transition and composer failures warn and continue with the applicable current draft. Trained-scanner failure falls back to basic masking with reduced name coverage. Files, images and attachments pass through unchanged with a warning; their contents are not redacted.

Earlier documents describing blocking, a separate manual side-panel map/dictionary, universal Iceland hosting or a guaranteed sensitive-work boundary are superseded. Strict shared-core callers retain their separate policies; do not change their behaviour as part of community positioning.

## Supported scope and limits

The preview targets selected text paths at `chatgpt.com` and `claude.ai`. Current fixture checks do not establish all authenticated live-site paths. Site changes and unsupported send paths can bypass interception. Other AI sites, mobile/desktop apps, voice, Firefox and document/image redaction are not verified features.

Selected kennitölur, Icelandic names in the trained profile, address forms, email, phone, IS IBAN, card numbers and secret patterns are covered to differing measured degrees. Follow the [quality contract](detector-quality-contract.md) for exact current evidence and the [preview guide](shield-autonomous-preview.md) for workflow limits. Do not imply exhaustive detection, anonymity, encryption of markers, legal compliance, certification or provider zero retention.

## Quality and releases

No release may lower recall for any entity type on the synthetic release benchmark. Existing synthetic gates and the owner's >=99% exact entity-set accuracy and >=99% recall research targets remain; the targets and independent acceptance are not achieved. Reused development results and browser fixtures are not representative field-quality evidence.

Use the existing Go/WASM, scanner, composer/race and browser regression suites for relevant changes. Preserve clean source/build identity, checksums and artifact-profile distinction. Keep research and raw corpora outside public assets. ADR 0006 authorizes the compiled trained alpha package, including its bundled local scanner, for public testing. Publish research summaries and limits only.

See the [distribution checklist](community-distribution.md), [community validation](validation.md) and [launch checklist](launch-checklist.md). No store listing or public download may be invented to satisfy a call to action. The current trained alpha is published as a fabricated-data test ZIP under ADR 0006; it is not a store release.

## Temporary disable — 0.1.3 alpha.3

Settings can pause masking and response restoration for the active chat tab for 15 minutes, or resume immediately. Other tabs stay enabled. The pause is in memory only, ends on expiry/reload, retains the current mapping and adds no permissions. New text is submitted without masking while paused, with a visible status. Pending scans are interrupted for that send and late results cannot overwrite the draft; only this gate's unchanged replacement can be rolled back. Existing edits/composer changes remain authoritative. See the [plan and acceptance scope](temporary-disable-plan.md).
