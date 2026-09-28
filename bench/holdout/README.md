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

1. Freeze `bench/results/error-audit-v2/selected-candidate.json` and its component
   hashes. Do not choose thresholds using these documents.
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
