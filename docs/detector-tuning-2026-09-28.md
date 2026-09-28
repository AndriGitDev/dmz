# Detector tuning session: formatting and evaluation

Date: 28 September 2026. Portal/Gateway work is deferred at the owner's request.
The current detector is deterministic Go rules, not a trained statistical model.
This pass improves those rules and the evaluation process; no external dataset,
model service, customer text or AWS credits were used.

## Scope and evidence

Baseline: `acf207b` (merged Portal policy documentation). The 41-case
`bench/corpus/format-v2.jsonl` was authored and scored before implementation.
Its two `format-untuned-*` reports preserve that original result. It includes
23 fabricated identifier annotations and 18 explicitly benign examples.

| Set | Before: literals exposed in allowed text | After | Benign prompts blocked after |
| --- | ---: | ---: | ---: |
| Selected v0, 42 examples | 0 | 0 | 0 / 8 |
| Challenge v0, 18 examples | 2 | 2 | 0 / 2 |
| Exploratory v1, 29 examples | 13 | 8 | 2 / 4 |
| Format v2, 41 examples (tuning set) | 14 | 0 | 0 / 18 |
| Holdout v2, 30 fresh examples (post-change probe) | Not measured | 8 | 2 / 10 |

Check the JSON reports for authoritative counts. Exact-span detection and whole
prompt blocking are different measures: Unicode review can prevent transmission
without identifying the annotated email span. All format-v2 annotations now match
exactly, with no false detections. No existing per-kind recall floor regresses.

The 30-case holdout was authored after implementation and scored without further
tuning. It is a same-author, deliberately difficult probe, not a blind external or
representative evaluation. It includes different fabricated values, an alternate
card grouping and known unsupported cases. It becomes a regression fixture if
used to guide future changes. Do not claim field accuracy or GA readiness from
these results.

## Behaviour changes

- Kennitölur accept a dot, ordinary space, nonbreaking space or narrow nonbreaking
  space between their six- and four-digit groups. Date/century and legacy checksum
  classification remain in force.
- Cards accept those horizontal separators between digits and retain length and
  Luhn checks. Existing support for repeated spaces is preserved.
- Icelandic phone shapes accept dots and nonbreaking spaces; `(+354)` is recognized
  as a complete country prefix. Adjacent numeric fragments across these separators
  are rejected as partial phone matches.
- Email domains accept Unicode letters. Any internationalised email is uncertain
  and blocks for review, as internationalised local parts already did. This is
  shape recognition, not IDNA/DNS validation. Empty labels, edge hyphens and invalid
  local-part dot placement are rejected.
- All spans refer to original UTF-8 bytes. No normalisation changes the input.
  Shield's existing mask/block policy and device-local restoration remain in force.
- Exposure reports now include benign-prompt totals, blocked/altered benign case
  IDs and exposed case IDs. No raw prompt or identifier values are added to reports.

Old corpus pins and recall/exposure gates are unchanged. New corpora have a separate
checksum file and CI snapshot checks. A Go regression test enforces no misses,
false detections, exposed annotations or benign-prompt interference on format-v2.
Historical exploratory reports are retained as `exploratory-untuned-*`.

## Remaining work, in order

1. Decide handling of tabs, Unicode dashes and identifiers split across lines.
   Holdout exposes four such examples. Blind whitespace removal can join unrelated
   table columns or numbers: collect document-layout negatives first.
2. General names and addresses need permitted lexicons/contextual evaluation.
   Holdout misses a foreign name and an apartment address. Current address heuristic
   still blocks a price phrase. Broader suffix matching alone is insufficient.
3. Generic service tokens remain exposed. Evaluate contextual key/value and entropy
   signals together with hashes, source code, IDs and prose negatives.
4. Tenant terms remain explicitly case-sensitive. Case-insensitive variants need
   a deliberate policy, UTF-8 span tests and restoration identity tests.
5. Combining marks still block harmless decomposed prose. Normalisation must retain
   original-byte mapping and preserve protection against hidden-character bypasses.

## Live Shield check after installing this build

Use fabricated values only, with a fresh Shield session between rows. Ask each site
(ChatGPT and Claude in Brave) to repeat the value exactly. Masked rows should send
markers and restore locally; review rows should stop before sending.

| Input | Expected |
| --- | --- |
| `Tilbúið kt: 140182.8858` | Mask, send marker, restore |
| `Prófunarkort: 4111.1111.1111.1111` | Mask, send marker, restore |
| `Óúthlutað prófnúmer: (+354) 000 0001` | Mask, send marker, restore |
| `Tilbúið netfang: demo@dæmi.invalid` | Block for review |
| `Óúthlutað prófnúmer: 00354 000.0001` | Block for review |
| `Dagsetning: 14.01.1882. Verð: 123.456,78 kr.` | Unchanged, no warning |

Check one masked example over multiple turns and one in each colour mode. These
live confirmations are pending; automated tests cannot establish authenticated
provider behaviour.

## Data the owner can contribute

Best next input: 20–50 **fabricated** examples preserving the structure of likely
customer tasks (emails, invoices, support tickets, contracts, code/configuration).
Include Icelandic/English mixtures, grammatical cases, punctuation and copied-table
formatting. For each, mark the exact sensitive span/type and whether it should be
masked or block the prompt. Supply an equally useful set of harmless lookalikes.
Keep some independently authored examples unseen until evaluation.

No real credentials, customer documents or personal records in this repository.
Useful source metadata: owner, source URL/version, licence/permission, commercial
use and redistribution terms, intended entity types and update cadence.

## Public-data candidates (discovery only)

- [HMS Staðfangaskrá](https://hms.is/gogn-og-maelabord/grunngogntilnidurhals/stadfangaskra)
  provides a downloadable CSV and describes weekly updates. Useful for street/place
  vocabulary. Reuse/redistribution terms still need verification; do not import
  complete household-address records as synthetic benchmark examples.
- [BÍN data](https://bin.arnastofnun.is/gogn/) could support inflected names. The
  current site requires JavaScript, and the exact downloadable resource licence
  has not been verified here. Repository instructions prohibit committing licensed
  BÍN/Mannanafnaskrá data; any permitted integration needs a separate data boundary.
- [MIM-GOLD-NER 2.0](https://repository.clarin.is/repository/xmlui/handle/20.500.12537/230)
  supplies annotated Icelandic NER text under a special user licence. Review that
  licence and the proposed use before obtaining it. Its entity taxonomy and text
  domain would need mapping to DMZ's privacy-policy evaluation; NER performance is
  not directly PII-protection performance.
- [Icelandic NER API / ELECTRA-base](https://repository.clarin.is/repository/xmlui/handle/20.500.12537/118)
  is listed under MIT. Potential local comparison baseline; separately verify model
  artefact dependencies and browser resource cost before considering integration.

Prefer official versioned downloads to scraping when provided. No source above was
scraped into a dataset or incorporated into the detector during this pass.

## Automated verification

Go unit/regression tests and vet; frozen corpus checksums and recall floors;
new corpus pins/report snapshots; Shield TypeScript check; TinyGo WASM build
and mask/block/restore smoke; Chromium extension smoke; synthetic ChatGPT/Claude
interception/restoration; network-body, composer-race and broken-core checks.
Authenticated live retesting by the owner remains pending.
