# Detector and Shield handoff — 5 October 2026

The inherited address-without-postcode work and subsequent Shield correctness changes are implemented in private draft PR [#16](https://github.com/AndriGitDev/dmz/pull/16). The clean trained preview is built from `a8a6704`; its 30 archive members, build record and twenty-corpus measurement package match. The frozen name model remains unchanged. This is a reviewable development preview, not a production release or independent ≥99% result.

## Ready for review

- Approved address forms with house numbers work without a postcode, including spacing, unit/range and Unicode-hyphen variants. The mechanical vocabulary audit covers 23,636 forms; contextual field accuracy remains unproven.
- Canonical Unicode matching retains original byte spans and exact restoration. Malformed JavaScript UTF-16 is rejected before conversion. Issued markers remain opaque on resubmission, and invalid tenant-term updates preserve live mappings.
- Complete supported private keys, database URLs, Bearer values, GitHub tokens, temporary AWS components, SigV4 presigned URLs and extended email forms are masked. Quoted credential punctuation and Unicode preserve complete values instead of dropping them at an unsupported suffix.
- The name scanner keeps a healthy worker, queues two requests behind one active scan, and preserves the admission-based deadline. Overflow/failure uses visible basic fallback; changed drafts remain unsent.

[Preview evidence](../bench/private-preview-20261005.md) records all twenty corpora separately, exact archive identity, WASM/browser checks and all sixteen scanner workflows, including the actual stalled-worker deadline. Both paths restore every evaluated excerpt exactly, with zero external scanner requests. Selected synthetic improvements are not representative acceptance estimates. [Preview instructions](shield-autonomous-preview.md) distinguish the small CI artifact from the private trained archive.

## Research outcome and limits

The [complete auxiliary-supervision trial](../bench/contextual/auxiliary-person-result.md) also fails the fixed research criterion. General exact accuracy improves by one excerpt, but one additional name is missed (recall 96.9456% versus 97.0120%). Domain recall improves to 99.375%, while exact accuracy remains 98.8281%. The recipe is closed without further tuning; no candidate was exported or promoted.

The completed nine-label checkpoints, exact-other-type veto and all 36 fixed threshold/priority combinations fail their unchanged comparison. Their failures remain recorded. The reused Person-only frozen baseline is 98.57% exact excerpt accuracy and 97.01% recall on general development, and 97.66%/98.75% on original synthetic-domain text. Neither stratum meets both targets. The split audit does not establish document independence; the long-context controls do not isolate neural boundary behavior because the CRF alone finds every selected name.

Remaining errors include short/low-entropy/letters-only and unsupported prose credentials, escape/interpolation-shaped values, ambiguous unfenced variable references, missed names and contextual addresses outside the implemented forms. Complete whole-text fallback can protect identifiers while failing typed extraction; restoration correctness does not prove detection completeness. See the [quality contract](detector-quality-contract.md), [quoted-literal results](../bench/quoted-credential-result.md) and per-type preview tables.

## Next actions

1. Arrange independent full-entity annotation, a separate second review and adjudication using the prepared offline packets and [rubric](../bench/holdout/entity-annotation-rubric.md). The original 142-excerpt/88-document packet remains unchanged and model-blind. The additional legal/financial pool also remains unlabelled, with source-use review pending. No reviewer outreach has been sent.
2. Freeze the acceptance sample design before predictions: separate general Icelandic, legal/financial text and mixed chat; document positive/negative mix, lengths, all entity types and document/article-cluster separation. Apply both ≥99% targets to each declared primary stratum, with uncertainty and per-type diagnostics. Do not train or choose thresholds on final labels.
3. Check the exact private trained build in the owner's supported ChatGPT/Claude browser workflows. Synthetic interception tests do not establish compatibility with every current site path. The host page can already access an original draft before masking.
4. Resolve source/model redistribution and remaining release requirements before any general release. Keep the PR in draft until its implementation review and applicable acceptance gates are satisfied; no merge, store release or production deployment was performed.
