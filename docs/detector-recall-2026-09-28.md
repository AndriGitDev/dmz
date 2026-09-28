# Detector recall round — 28 September 2026

Owner requested higher recall while preserving the 95% name-precision target.
The same MIM-GOLD-NER 2.0 training/dev/test files and exact UTF-8 Person-span
metric are used. Evaluation labels are not added to training.

## Results

| Configuration | Split | TP | FP | FN | Precision | Recall |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Previous precision pack | dev | 496 | 20 | 1010 | 96.12% | 32.93% |
| **Selected recall pack** | dev | 997 | 39 | 509 | **96.24%** | **66.20%** |
| Previous precision pack | test | 484 | 19 | 1058 | 96.22% | 31.39% |
| **Selected recall pack** | test | 1008 | 47 | 534 | **95.55%** | **65.37%** |

On test, 524 additional names are correctly detected, with 28 additional incorrect
detections. Recall rises **33.98 percentage points**. Precision remains above 95%,
but 534 complete name spans are still missed. Partial predictions count as both a
false positive and a missed complete span. These are name metrics, not all-PII
metrics or outbound-leakage measurements. Aggregate reports and hashes are in
[`bench/results/recall-v4`](../bench/results/recall-v4).

## Changes

The full-name trainer counts a token as positive only when it is an entire
annotated Person. Given names inside longer names contribute to total frequency
but not that positive count, so the conservative filter omits useful names.
The separate `name_seeds` field now learns capitalized tokens from any training
Person span. Selected seeds require at least one positive observation and a
Person-token frequency ratio of .99. This ratio is not a calibrated probability;
one-observation seeds remain weak evidence.

At runtime seeds create uncertain review findings. Existing learned components
extend them where possible. An adjacent unresolved capitalized word signals a
possibly incomplete name; that seed candidate is declined. A surname following
an initial is also declined. Checks cross horizontal whitespace only; punctuation,
newlines and existing patronymic stopping rules remain boundaries. Declining a
seed does not remove baseline/full-name/secret/tenant matches. The Unicode guard
still runs first. Token lookup during expansion now uses binary search instead
of scanning every token for every candidate.

Old packs without `name_seeds` retain their behavior. `--seed-minimum` defaults
to 0 (disabled), preserving previous pack bytes. The synthetic CI pack exercises
the new field without corpus data. All derived entries remain outside Git.

## Selection and limitations

Simply adding Person-derived tokens increased development recall but left
precision below 95%, mainly due to incomplete name spans. Boundary checks address
that failure without changing the metric. Development experiments compared seed
minimum counts 1, 2, 3 and 5 at .99 purity, plus an early .999 comparison.
With the final boundary checks:

| Seed minimum | Dev TP | Dev FP | Dev precision | Dev recall |
| --- | ---: | ---: | ---: | ---: |
| 3 | 893 | 28 | 96.96% | 59.30% |
| 2 | 932 | 33 | 96.58% | 61.89% |
| **1, selected** | **997** | **39** | **96.24%** | **66.20%** |

Minimum 1 gave the highest recall above the precision target. Configuration was
frozen before this round's test run; test errors were not inspected or used for
further tuning. The test split was evaluated in previous rounds: this is a reused
corpus regression test, **not newly unseen independent validation**. Sentence-hash
splits share entities and potentially related documents; reconstructed token
spacing differs from real prompts. 95.55% is a point estimate from 1,055 predictions,
not a statistical guarantee of >=95%. Fresh annotated daily-use prompts are still
needed for field/GA claims. This is lexicon training, not neural fine-tuning.

## Reproduce

```sh
python3 bench/datasets/train_lexicon.py \
  --train /root/.local/share/dmz/datasets/prepared-v2/train.jsonl \
  --addresses /root/.local/share/dmz/datasets/prepared-v2/address-forms.json \
  --minimum 2 --purity .95 --part-minimum 1 --part-purity .5 \
  --seed-minimum 1 --seed-purity .99 \
  --output /root/.local/share/dmz/datasets/lexicon-recall-v4-n1.json

go run ./bench/ner-eval \
  -input /root/.local/share/dmz/datasets/prepared-v2/test.jsonl -split test \
  -lexicon /root/.local/share/dmz/datasets/lexicon-recall-v4-n1.json

DMZ_LEXICON_FILE=/root/.local/share/dmz/datasets/lexicon-recall-v4-n1.json \
  ./scripts/build-wasm.sh
```

Selected pack: 955 full names, 4,518 seeds, 4,738 boundary components, 23,636 address
forms; 471,050 bytes. SHA-256:
`bbed53a046f3feac95e52fe7ff6afc9eadbce82c7714086c98684645ba197312`.
The owner-local preview uses this pack; default CI artifacts omit external data.
