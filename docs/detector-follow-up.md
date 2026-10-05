# Detector follow-up from the 29-case exploratory probe

**Update:** The [formatting tuning session](detector-tuning-2026-09-28.md) reduces the exploratory exposure count from 13 to 8 and adds fresh holdout evidence. The analysis below records the original untuned probe.

**5 October update:** Current detector and packaged-system evidence is maintained
in the [quality contract](detector-quality-contract.md), including approved
address forms, Unicode handling, secret-pattern fixes and the completed model
trial. The priority table below is historical: its full-prompt secret-blocking
proposal does not describe the owner's current Shield policy. Shield masks
recognised secrets under best-effort handling; strict core callers retain blocks.

Date: 28 September 2026. Source: [`bench/corpus/exploratory-v1.jsonl`](../bench/corpus/exploratory-v1.jsonl) and its [exposure report](../bench/results/exploratory-exposure-v1.json). The 29 cases were written after the targeted changes to the original challenge set; the detector was not tuned to them. They are synthetic, deliberately difficult examples, not a sample of customer traffic or a field recall estimate.

The current prototype leaves **13 annotated literals** in allowed outbound text: three addresses; two each of kennitölur, names and secrets; and one each of email, phone, card and tenant term. The corpus also includes a benign price phrase that the address heuristic blocks. Full case IDs and annotations are in the corpus. The small original challenge set remains a regression fixture: its post-fix exposure count is two, while its preserved untuned snapshot records eight.

## Next detector work

| Priority | Evidence in probe | Work to test before changing the rule |
| --- | --- | --- |
| 1. Explicit formatting variants | `v1-kennitala-dotted`, `v1-kennitala-spaced`, `v1-card-dotted`, `v1-phone-dotted` are exposed | Specify which separators are accepted per type. Keep date/check-digit, Luhn and boundary checks. Add benign number and date negatives before broadening matches. |
| 2. Email and Unicode | `v1-email-unicode-domain` is exposed; combining-mark and invisible-character cases are blocked for review | Decide how internationalized domains are recognized with exact original-byte spans. Keep the conservative Unicode review guard until normalization and spoofing behavior is tested. |
| 3. Names and addresses | Two unlisted names and three address variants are exposed; `v1-negative-price` is blocked as an address | Obtain permitted name and street data before claiming broad coverage. Measure inflected forms and false positives together. Do not expand a suffix regex solely to clear these cases. |
| 4. Secrets and tenant terms | Two arbitrary prefixed tokens and the differently cased tenant term are exposed | Design a context and entropy rule with benign code/text negatives; keep secret handling as full-prompt block. Decide whether tenant dictionary variants are explicit entries or locale-aware matching. Keep mappings device-local for Shield. |

For each change, create a new versioned synthetic corpus before scoring it, retain prior snapshots and corpus checksums, and review both exact-span detection and outbound exposure. Record blocked benign examples alongside misses. The current selected and challenge sets are too small for the Gate 2 recall targets; a separate representative evaluation set and licensed-data-safe name coverage remain prerequisites. Until those controls exist, Shield remains a manual demo for fabricated text only.
