# Offline contextual name experiments

This trains a local linear-chain CRF on the approved MIM-GOLD-NER training split.
It is an experimental Person recognizer, **not a shipped Shield detector**.
Corpus text, extracted vocabulary, and model weights stay outside Git and are not
sent to a provider. Only aggregate reports and fabricated examples are committed.

## Reproduce

From the repository root, with prepared splits from `bench/datasets/prepare.py`:

```sh
python3 -m venv /tmp/dmz-contextual-venv
/tmp/dmz-contextual-venv/bin/pip install -r bench/contextual/requirements.txt
/tmp/dmz-contextual-venv/bin/python -m unittest discover -s bench/contextual -p 'test_*.py'
/tmp/dmz-contextual-venv/bin/python bench/contextual/ner.py train \
  --input /path/outside/repo/train.jsonl --model /path/outside/repo/model.crfsuite \
  --feature-version 2 --iterations 150
/tmp/dmz-contextual-venv/bin/python bench/contextual/ner.py evaluate \
  --input /path/outside/repo/dev.jsonl --model /path/outside/repo/model.crfsuite \
  --split dev --thresholds 0 .5 .6 .65 .7 .75 .8 .9 .95 > /tmp/dev-report.json
python3 bench/contextual/gate.py /tmp/dev-report.json --threshold 0.7
```

The gate exits 1 unless **both** precision and recall are at least 0.955. A failed
research gate must not be made green by changing the denominator or target. It is
not wired as a passing release requirement while the experimental model fails it.
Existing synthetic per-entity release floors remain in force.

Select features and a confidence threshold using development results, then freeze
them before evaluating test data. Keep unsuccessful development runs. Training
rejects non-training rows; evaluation checks the model checksum against its
metadata. Feature v1 uses local affixes and neighboring words/shapes; v2 adds
neighboring affixes and word/shape pairs; v3 adds explicit initials, length and
case features. Minimum BIO token marginal is a ranking
score, not a calibrated probability that a whole name is correct.

Metrics require an exact UTF-8 span match. A partial name counts as one false
positive and one false negative. Names alone are scored here: this does not measure
other identifier types, outbound exposure, or the combined Go/Shield pipeline.
The input assumes the corpus's canonical token-spaced text; production tokenization,
Go/WASM inference, streaming, package size, and latency still need implementation
and validation before considering integration.

## Legal and financial probe

`bench/corpus/domain/legal-financial-v1.jsonl` contains 40 deliberately selected,
agent-authored synthetic documents: 30 positive snippets with 37 Person spans and
10 benign snippets. Names and scenarios are fabricated. It covers contracts,
claims, powers of attorney, invoices, loans, payroll, estates and auditing, with
inflections, initials, foreign names, and casing variants. It is a short-snippet
challenge set, not a sample of real legal/financial documents or legal advice.

The JSONL has token-spaced `text`/`entities` plus `original_text`/
`original_entities` preserving typography; offsets are UTF-8 bytes in the
corresponding text. Current experiments use the token-spaced version only.
The SHA-256 pin in `bench/gates/legal-financial-v1.sha256` freezes this version.
Copy it outside the repository to evaluate with the contextual CLI, and verify
the copy's checksum. Do not tune against its failures and continue calling it
independent. Add a new version for future probes.

The old MIM test split has already informed previous development, and its
sentence-based split does not enforce document or entity disjointness. It is a
**reused evaluation set**, not fresh evidence of generalization. The domain probe
is newer but small and authored by the same developer. A 95.5% claim requires a
larger independently annotated, document-disjoint legal/financial evaluation,
including negative documents and names absent from training. Report each dataset
separately; do not pool easy structured identifiers with names to inflate scores.

## Recorded experiment

The [28 September report](../../docs/detector-contextual-2026-09-28.md) records
both configurations and the development-selected threshold. Final reused MIM
results are 95.26% precision / 84.70% recall; the synthetic domain probe gives
96.88% / 83.78%. **Neither meets the dual 95.5% target.**

## Error-driven follow-up

The [v2 experiment protocol](experiment-v2.md) defines the development-only
comparisons, selection rule and pretrained encoder provenance. Start with:

```sh
python bench/contextual/audit.py --input /outside/dev.jsonl \
  --training /outside/train.jsonl --model /outside/model.crfsuite
```

Only aggregates go to stdout. Optional `--errors-out /outside/review.jsonl`
contains source text for local review, refuses overwrites, and must stay outside
Git. Error reasons are mutually exclusive within FP/FN; name characteristics
overlap and must not be added together. Inflection counts are suffix-shape probes,
not a morphological analyzer or a human annotation audit.

`--label-mode entities` trains competing entity labels while still scoring exact
Person spans only. `--text-mode original` evaluates the synthetic original-text
fields using a candidate offset-preserving tokenizer; it does not establish
production tokenization coverage. `--progress` writes numeric progress to stderr.

The v2 legal/financial corpus is generated by `domain.py`. The committed manifest
pins train/dev bytes. The generator never produces a final test split. Review
`REVIEW.md` before treating annotations as independently verified. To reproduce a
mixed training input, concatenate MIM `train.jsonl` and the synthetic `train.jsonl`
once outside the repository; do not include either development file.

The optional encoder tools require a separate environment. Install CPU PyTorch
from its official wheel index, then `encoder-requirements.txt`. Use a local pinned
IceBERT snapshot and set `HF_HUB_OFFLINE=1 HF_HUB_DISABLE_TELEMETRY=1` for all model
runs. `encoder.py encode` caches mean-subword vectors outside Git, `train` fits
only the small BIO classifier and saves safetensors checkpoints, and `evaluate`
checks development spans. Corpus and model/cache hashes bind each result. No
remote inference or uploaded prompts are involved. These large research caches
and models are not extension assets.

Example encoder commands (all paths below are outside the checkout):

```sh
export HF_HUB_OFFLINE=1 HF_HUB_DISABLE_TELEMETRY=1
python bench/contextual/encoder.py encode --input /outside/train-domain.jsonl \
  --split train --base /outside/icebert-base --output /outside/encoder-train
python bench/contextual/encoder.py encode --input /outside/dev.jsonl \
  --split dev --base /outside/icebert-base --output /outside/encoder-dev
python bench/contextual/encoder.py encode --input /outside/domain-dev.jsonl \
  --split dev --base /outside/icebert-base --output /outside/encoder-domain
python bench/contextual/encoder.py train --cache /outside/encoder-train \
  --output /outside/head-linear --hidden 0 --epochs 8
python bench/contextual/encoder.py train --cache /outside/encoder-train \
  --output /outside/head-mlp --hidden 128 --epochs 8
python bench/contextual/encoder.py evaluate --cache /outside/encoder-dev \
  --head /outside/head-linear --epoch 1
```

Evaluate each epoch 1–8 of both heads against both development caches, preserving
every report. The default encoder threshold grid matches the protocol. Cache and
head output directories must be new; a cache is complete only when its metadata
and checksums have been written. Use `--threads` to limit local CPU usage.

The [error-audit report](../../docs/detector-error-audit-2026-09-28.md) records
this round separately from the older reused-test results. Candidate selection is
reproduced with `python bench/contextual/selection.py
bench/results/error-audit-v2/crf-candidates.json`. It checks one common threshold
grid and requires the precision floor on both development datasets.
