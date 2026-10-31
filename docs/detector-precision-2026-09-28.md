# Detector precision round — 28 September 2026

Owner requested at least 95% precision. This round targets **exact-span Person/name
precision** on MIM-GOLD-NER 2.0. It does not establish 95% precision for every PII
category, field prompts, or all dmz blocking decisions.

## Results

| Configuration | Split | TP | FP | FN | Precision | Recall |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Previous selected pack | dev | 534 | 76 | 972 | 87.54% | 35.46% |
| Boundary learning, minimum 1 | dev | 586 | 33 | 920 | 94.67% | 38.91% |
| **Selected precision pack, minimum 2** | dev | 496 | 20 | 1010 | **96.12%** | **32.93%** |
| Previous selected pack | test | 530 | 88 | 1012 | 85.76% | 34.37% |
| **Selected precision pack** | test | 484 | 19 | 1058 | **96.22%** | **31.39%** |

Aggregate machine-readable reports and hashes are in
[`bench/results/precision-v3`](../bench/results/precision-v3).
The stricter pack removes many rare full-name entries: precision improves, but
recall falls 2.98 percentage points on test. It still misses 1,058 of 1,542 gold
name spans. Higher precision must not be read as comprehensive PII protection.

## Changes and selection

- Fixed a trainer denominator bug: a phrase at the end of a sentence was counted
  repeatedly when the slice requested a longer candidate length.
- Learn capitalized name components from training Person spans. Components only
  extend an existing detection; they do not independently trigger matches.
- Extend across spaces/tabs, with a maximum of eight tokens. Do not cross an
  unknown token, punctuation or newline. Do not extend right past a patronymic
  ending or left across another patronymic, to reduce adjacent-name merging.
- Expanded spans remain uncertain and require review. No uncertain findings are
  promoted to automatic masking confidence.
- Preserve baseline rules, tenant terms and the Unicode guard. Old data packs
  without `name_parts` retain their prior behavior.

The initial development error audit found 69 partial/overlapping name predictions
and 7 predictions with no Person overlap, out of 76 false positives. Boundary
repair was therefore the first tuning target. All are still counted as false
positives under the unchanged exact-span metric.

Development experiments used full-name minimum counts 1 or 2; purity .95 or 1;
component minimum counts 1 or 2; and component purity .95, .8 or .5. After boundary
repair, minimum 1 gave 94.67% precision, so minimum 2 was selected to meet the
owner's precision target. The selected configuration was frozen before this
round's test evaluation. No test errors were inspected or used for further tuning.

Selected thresholds: full-name minimum **2**, purity **.95**; component minimum
**1**, purity **.5**. Component purity is a training frequency filter, not calibrated
confidence. Pack: 955 names, 4,738 boundary components and 23,636 address forms;
422,144 bytes. SHA-256:
`1eefb090e59afb5074dd394c13899425a429baa940f2fffeb43e13f8b18b9686`.

## Evaluation limits

The existing train/dev/test split was retained. It is sentence-hash based, not
document- or entity-disjoint, and the test set was already evaluated in the
previous round. This is a reused corpus test, **not newly unseen validation**.
Resplitting the same corpus would not fix that limitation. Text is reconstructed
with token spacing and includes fictional characters and other annotated Persons.

96.22% is a point estimate from 503 predictions; it does not establish a statistical
lower bound of 95% or day-to-day performance. A separate, independently annotated
Icelandic prompt set is needed before making a field claim. Keep all source
sentences, derived names and data packs outside Git; only aggregate reports and
synthetic tests are committed. No neural model was fine-tuned: this is lexicon
training plus boundary-rule tuning.

## Reproduce the selected pack locally

```sh
python3 bench/datasets/train_lexicon.py \
  --train /root/.local/share/dmz/datasets/prepared-v2/train.jsonl \
  --addresses /root/.local/share/dmz/datasets/prepared-v2/address-forms.json \
  --minimum 2 --purity .95 --part-minimum 1 --part-purity .5 \
  --output /root/.local/share/dmz/datasets/lexicon-v3-n2-q0.95.json

go run ./bench/ner-eval \
  -input /root/.local/share/dmz/datasets/prepared-v2/test.jsonl -split test \
  -lexicon /root/.local/share/dmz/datasets/lexicon-v3-n2-q0.95.json

DMZ_LEXICON_FILE=/root/.local/share/dmz/datasets/lexicon-v3-n2-q0.95.json \
  ./scripts/build-wasm.sh
```

The regular CI artifact still contains synthetic data only. The owner-local
dmz preview must be built with this explicit pack and checked through the
WASM and Chromium smoke suites. The prior enriched pack remains locally available
for recall comparisons. No source data or trained pack is uploaded to the PR.

## Subsequent recall work

The [recall round](detector-recall-2026-09-28.md) supersedes this pack for the
owner-local preview: 65.37% recall at 95.55% precision on the same reused test
set, using Person-derived seed tokens and conservative boundary checks.
