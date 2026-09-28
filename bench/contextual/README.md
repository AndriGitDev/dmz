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
neighboring affixes and word/shape pairs. Minimum BIO token marginal is a ranking
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
