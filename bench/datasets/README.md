# External detector datasets

On 28 September 2026 the owner confirmed permission to use both datasets identified
in the tuning discussion: HMS Staðfangaskrá and MIM-GOLD-NER 2.0. This records owner
confirmation, not an independently reviewed licence opinion or permission to
publish the source data. BÍN/Mannanafnaskrá are separate and not covered here.

## Sources and local boundary

- HMS: https://hms.is/gogn-og-maelabord/grunngogntilnidurhals/stadfangaskra
  CSV: https://hmsstgsftpprodweu001.blob.core.windows.net/fasteignaskra/Stadfangaskra.csv
- MIM-GOLD-NER 2.0: Ingólfsdóttir, Guðjónsson and Loftsson, Reykjavik University,
  https://repository.clarin.is/repository/xmlui/handle/20.500.12537/230
  ZIP: https://repository.clarin.is/repository/xmlui/bitstream/handle/20.500.12537/230/MIM-GOLD-2_0.zip?isAllowed=y&sequence=15
  The archive README describes the special MIM-derived user licence.

Source copies and derived text live under `/root/.local/share/dmz/datasets`, outside
Git. Only tooling, fabricated tests, hashes and aggregate results are committed.
No source text is sent to model APIs, CI or the public marketing repository.
The standard CI Shield package omits external data. An explicit local build can
embed the approved lexicon; see [the lexicon tuning report](../../docs/detector-lexicon-2026-09-28.md).

## Reproduce preparation

Download the two official files to local storage, then run:

```sh
python3 bench/datasets/prepare.py \
  --hms /root/.local/share/dmz/datasets/stadfangaskra.csv \
  --mim /root/.local/share/dmz/datasets/mim-gold-ner-2.zip \
  --output /root/.local/share/dmz/datasets/prepared-new
python3 -m unittest discover -s bench/datasets -p 'test_*.py'
go run ./bench/ner-eval \
  -input /root/.local/share/dmz/datasets/prepared-new/dev.jsonl
```

Output must be a new directory outside the checkout. Failed imports can leave a
partial directory; only a completed `manifest.json` indicates success. Compare
input hashes with the committed manifest to reproduce this snapshot. HMS changes
over time; do not silently replace a baseline with a newer source release.

## Prepared snapshot

`manifest-2026-09-28.json` records:

- HMS: 139,561 records → 24,401 unique nominative/dative address forms. Only
  `HEITI_NF` and `HEITI_TGF` are retained. No house numbers, property identifiers,
  coordinates, employee identifiers or full address records in the derived file.
  This vocabulary also contains place/address names; it is not purely street names.
- MIM: 56,672 retained sentences; 45,332 train / 5,582 development / 5,758 test.
  1,750 exact text+annotation duplicates removed; three malformed sentences
  quarantined with member names/line numbers recorded; 19 orphan I-tags treated
  as B-tags and counted. No malformed line is silently concatenated into a token.
- Source tokens are reconstructed with single spaces. Spans use UTF-8 bytes.
  Original document punctuation spacing is not recovered.
- Text-hash assignment keeps exact duplicate text in one split even when its
  annotations conflict. It is **not document-disjoint** and does not eliminate
  near-duplicates, common entities or topical overlap. Use document/source-aware
  evaluation before making a generalisation claim. The test split was reserved during lexicon selection, then scored once in the
  [lexicon pass](../../docs/detector-lexicon-2026-09-28.md). It is now a recorded
  evaluation set; future tuning requires a fresh independent test set.

## Untuned development baseline

`name-baseline-dev.json` evaluates the current `detect.Name` spans against MIM's
Person spans on 5,582 development sentences: **198 TP, 64 FP, 1,308 FN**, or
**75.57% precision / 13.15% recall** for exact spans. This is a baseline of the
small existing heuristic, not a newly trained model. Whole-prompt Unicode blocks
can prevent transmission without producing a matching name span; this report
measures name recognition, not outbound privacy protection.

MIM Person includes fictional beings and animals as well as people. Location
includes many things other than addresses. Do not map every Location to an
address or treat all non-Person labels as non-sensitive. The evaluator filters
predictions to names; it is not a complete PII benchmark. It prints aggregates
only and refuses records from a split other than the explicitly selected one.

## Next tuning pass

Use only training/development partitions to analyse name-length, inflection,
foreign-name and context errors. Keep raw error examples local. Compare expanded
rules or a local NER baseline before choosing what can fit in Shield's WASM budget.
Build an address lookup interface using HMS forms and fabricated numbered-address
fixtures, with price/product-name negatives. Packaging and refresh behaviour need
an explicit implementation; no detection improvement is claimed from preparation.

## Precision-focused second round

The optional `name_parts` field contains training-derived boundary components.
`--part-minimum` and `--part-purity` control their frequency filters; these parts
only extend existing name matches. Full-name filtering still uses `--minimum`
and `--purity`. Sentence-end candidate counts now exclude truncated slices.
See [precision results and reproduction](../../docs/detector-precision-2026-09-28.md)
for the selected configuration: 96.22% exact-name precision and 31.39% recall on
the reused test set. This is not an all-PII or field-performance claim.

## Recall improvement

Opt in to standalone Person-derived tokens with `--seed-minimum 1 --seed-purity .99`.
The default `--seed-minimum 0` disables this feature, preserving previous pack
bytes. Tokens can be learned inside full Person spans; runtime boundary checks
reject likely incomplete seed matches. All new findings remain uncertain.
See [recall results](../../docs/detector-recall-2026-09-28.md): 65.37% name recall
at 95.55% precision on the reused test corpus. Source entries remain outside Git.
