# Independent evaluation preparation

Status: **candidate collection only, no independent score yet**.

The 13 CoNLL text members of the approved MIM-GOLD-NER 2.0 archive are already
consumed by preparation. The archive has no additional untouched annotated
partition. The current test split was evaluated in earlier rounds. A new random
split of these sentences would not independently validate the frozen candidate.
MIM remains useful for training and diagnostics. A future retraining experiment
could reconstruct document groups and start again with a document-disjoint split;
that does not retrospectively validate the current model.

## Collection

`sources.json` lists a small public-source pilot: annual financial reports,
parliamentary legislation and submissions, and a published legal opinion. These are candidates, not a
representative customer corpus. Official documents can overrepresent public
figures, structured tables and repeated institutional language. Court annual
reports are administrative material, not case files or contracts.

```sh
/root/.local/share/dmz/encoder-venv/bin/python bench/holdout/collect.py \
  --mim /root/.local/share/dmz/datasets/prepared-v2 \
  --output /root/.local/share/dmz/datasets/holdout-new
```

Requires `pypdf==6.1.1`. Use `--only source-id ...` for a subset. The collector
fetches sequentially, caps each PDF at 20 MB, and never follows a link graph.
Failed requests are recorded; do not work around access controls. Output must
be a new directory outside the repository. All PDFs, extracted pages and review
text remain local. Only source URLs, hashes and aggregate counts belong in Git.
Collection does not establish redistribution rights. Review source-use terms
before making an approved evaluation release; do not publish these PDFs/text.

Local runs:

- `/root/.local/share/dmz/datasets/holdout-candidates-v1`
- `/root/.local/share/dmz/datasets/holdout-legal-candidates-v1` (all three requests returned HTTP 403)
- `/root/.local/share/dmz/datasets/holdout-opinion-candidates-v1`

Three documents downloaded: 247 source pages, 36 sampled review pages. Three
MIM-overlapping 12-grams were flagged in the Byggðastofnun sample; the other two
samples had none. This does not establish independence. Court download failed;
Alþingi returned HTTP 403 and was not bypassed. The legal-opinion PDF emitted
object-offset repair warnings during extraction; visual review is required.

Each `review.jsonl` contains deterministically sampled pages, with `entities: null`
and `reviewed: false`. Null means **unlabelled**, not no names. The file is not an
input to the existing evaluator and must not be silently converted to negatives.
Selection occurs before model inference. Exact normalized 12-gram overlap with
all MIM splits is flagged. This catches some copied wording, not all leakage.
No model predictions have been generated for these documents.

## Freeze, annotate, then evaluate

1. For the current full-precision run, use the immutable
   `bench/results/full-precision-v1/selected-candidate.json` and its pinned
   component hashes. The earlier `error-audit-v2` selection remains a preserved
   baseline. Freeze any subsequently accepted compressed candidate separately
   before independent inference. Do not choose thresholds using these documents.
2. Expand to a predeclared sample across independent documents, issuers and
   legal/financial genres. Keep whole documents and duplicate versions together.
   Separate collection/annotation pilots from the final sample before inference.
3. Review extraction against the source PDF (tables, columns, broken words).
   Exclude unusable pages with recorded reasons before seeing predictions.
4. Resolve flagged overlap and near-duplicates with existing train/dev/test data.
   Record remaining pretraining uncertainty. New dates alone do not prove novelty.
5. Have a human mark every Person mention, including single names, inflections,
   initials and foreign names. Review genuine negative pages too. Use UTF-8 byte
   offsets for the exact extracted text, matching the existing evaluator.
   Follow MIM's Person policy consistently; disagreements require adjudication.
6. Record reviewer, annotation version and corpus hash. A second review of a
   sample should check boundary agreement. Agent-generated labels alone do not
   constitute human-reviewed gold.
7. Evaluate the frozen candidate once. Report TP/FP/FN, per-form breakdown,
   document-level false alarms, and uncertainty using document-level resampling.
   Do not claim both metrics >=95.5% from development numbers or unlabelled pages.
8. If errors inform another training round, retire that sample to development and
   reserve new documents for the next independent evaluation.

Sources: [MIM-GOLD-NER 2.0](https://repository.clarin.is/repository/xmlui/handle/20.500.12537/230),
[Central Bank 2025 report announcement](https://sedlabanki.is/frettir-og-utgefid-efni/grein/arsskyrsla-sedlabanka-islands-2025),
[Byggðastofnun publications](https://www.byggdastofnun.is/is/utgefid-efni),
and the direct Alþingi links in the source manifest.

## Blind annotation packet

`review.py prepare --source /outside/collection/review.jsonl ... --output /outside/packet`
creates numbered UTF-8 text files, an annotation template, checksummed provenance,
and reviewer instructions. No predictions or automatic labels are included.
`review.py validate --packet /outside/packet` rejects missing reviews, null labels,
unresolved overlap, changed source text, invalid UTF-8 offsets, overlapping spans,
and attempts to promote this pilot to final evaluation. Passing validation checks
structure only; it cannot establish human annotation correctness or independence.

The current private packet is `/root/.local/share/dmz/datasets/blind-review-v1`:
36 pages, three documents, all unlabelled. It includes the already collected
financial and legal-opinion pilot pages; no new download or model inference.

## General-prose pilot added 29 September

`collect_general.py` collects only the six URLs frozen in `general-sources-v1.json`:
the first three article links in each of two retrieved news-index snapshots from
Náttúrufræðistofnun and Veðurstofa Íslands. These are nature/science and weather
news, not a representative general-Icelandic sample. The index/search snapshots
can lag live publication order. Selection was made before any detector inference;
neither names nor model errors were used to select among these six articles.

Raw HTML, extracted text and null-label review rows stay outside Git at
`/root/.local/share/dmz/datasets/holdout-general-candidates-v1`. Extraction uses the
largest article container and must be checked against the source, including
headings, captions, tables and initials. Source-use review remains pending;
collection is not permission to redistribute or train on these pages. In
particular, [Veðurstofan's terms](https://vedur.is/um-vi/vefurinn/notkunarskilmalar/)
distinguish data from written material; no blanket licence claim is made here.

All six downloads/extractions succeeded and no exact normalized MIM 12-gram
overlap was found. This does not establish semantic, document or pretraining
independence. All annotations remain null. The combined private review packet
`/root/.local/share/dmz/datasets/blind-review-v2` includes the earlier 36 PDF pages
and these six articles: 42 review records across nine source documents. Version 1
is preserved. Neither packet has been run through the detector or used in training.
