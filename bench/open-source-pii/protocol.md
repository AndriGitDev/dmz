# Open-source PII integration experiment — 5 October 2026

Owner request: an eight-hour research/integration slice to improve DMZ's overall
accuracy and English performance. Starting reference: `9b711c0`, the completed
October private trained preview; preceding PR 16 remains separate. No model or
integration has been selected yet. Keep research details in this private repo.

## Boundaries and decision procedure

dmz must keep all prompt text, values and mappings on device. A Python-only
candidate can be a server-side comparison or offline research tool, but is not a
dmz integration. Do not send test text to inference APIs. Download public code
and weights into private local paths; perform inference offline. Preserve the
existing Icelandic model and address vocabulary. Do not read blind evaluation
text into a detector: the 142-excerpt packet and unlabelled domain pools remain
reserved for independent annotation.

1. Review primary-source code/model licenses separately, supported labels,
   maintenance, CPU/browser runtime and package size. A model's advertised token
   accuracy/F1 is not DMZ's exact entity-set accuracy.
2. Freeze newly authored, fully labelled synthetic English/mixed-language
   development probes before candidate predictions. Include ordinary prose,
   business text, chat, Unicode, structured values and benign negatives.
   These are author-labelled diagnostics, never independent acceptance data.
3. Compare frozen DMZ, candidates alone, and compatible additive compositions.
   Score exact UTF-8 spans/types, per-excerpt exact sets, recall, precision,
   per-type counts and benign false positives. Keep language strata separate.
   Do not map generic locations or organizations to street addresses or people.
4. Initial candidates: Presidio's pinned English baseline; Knowledgator
   GLiNER-PII base; Fastino GLiNER2-PII; dslim BERT NER as a focused English-name
   control. Assess external provider Privacy Filter as resources and runtime allow.
   Start with upstream documented defaults, record complete configuration, and
   retain failures. If a bounded calibration is needed, declare it before
   examining those results and keep its selection data separate from follow-up.
5. A production-default proposal needs improved English exact-set accuracy and
   recall, no Icelandic per-type recall or exact-set regression on retained
   probes, exact restoration, sound offsets and no external inference requests.
   Otherwise keep a promising adapter explicitly experimental/opt-in, or reject
   it. A synthetic improvement alone cannot justify a general release or 99% claim.
6. Validate integration through the actual Go/WASM boundary and browser where
   applicable, including overlap, malformed spans, lifecycle and fallback. Run
   the frozen Icelandic name development comparison for model-added name paths.

Initial shared scoring scope is the existing nine DMZ entity types. English
phone/address coverage may be poor by design in the old detector; label it
honestly. New categories (e.g. SSN, passport, date of birth) must have an explicit
policy/taxonomy and separately labelled diagnostic before any implementation
claims; they cannot be silently mapped to Icelandic kennitala.

## Session budget and artifacts

The goal started at Unix time 1791193566; the requested eight-hour boundary is
1791222366. Use the time for substantive research, measured experiments and
reviewable code, subject to platform limits. Preserve exact artifact/version
identities and rejected results. Model/data downloads, prediction caches and raw
reports stay under `/root/.local/share/dmz/experiments/open-source-pii-20261005`.
Only synthetic fixtures, harnesses, reviewed aggregates and documentation enter
Git. No merges, deployments or store publication are part of this slice.
