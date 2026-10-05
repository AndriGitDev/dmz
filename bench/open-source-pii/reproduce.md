# Reproducing the retained comparisons

Run from the DMZ repository root. All corpus downloads, weights, raw caches and
new comparison outputs must stay in a private directory outside the repository.
The scripts refuse existing output files; use a new output name for a retrial.
Only aggregate exporters write reviewed reports under `bench/results`.

## Recorded environment

The 5 October run used Go 1.27.1, TinyGo 0.42.0 and ONNX Runtime Web 1.23.2.
Python package metadata was checked from the research virtual environment:

| Package | Version |
| --- | --- |
| torch | 2.8.0+cpu |
| transformers | 4.57.6 |
| onnxruntime | 1.23.2 |
| onnx | 1.19.0 |
| numpy | 2.2.6 |
| gliner | 0.2.29 |
| gliner2 | 2.0.0 |
| presidio-analyzer | 2.2.364 |
| spacy | 3.8.16 |
| en-core-web-lg | 3.8.0 |
| opf | 0.1.0 |
| usaddress | 0.5.16 |
| python-crfsuite | 0.9.12 |
| probableparsing | 0.0.1 |

Model revisions and schemas are in `candidates.json`; the public source and
conversion are specified in `external-english-protocol.md`. The evaluator
requires each model's `download-manifest.json`, verifies the pinned revision and
every recorded file hash, and denies socket connections before loading/inference.
The model directory is named by replacing `/` with `--` in its model identifier.
Use the separately retained 512-excerpt selection for Privacy Filter; its
incomplete larger attempt is excluded from complete-set comparisons.

## Evaluate and compare

These examples use an already prepared, checksum-pinned private corpus and model
directory. Set `DMZ_PII_TASK` to that private experiment directory and use the
research virtual environment's `python` on PATH. `models` denotes the directory
of pinned model folders; `datasets/gretel-projected-v2` is the converted corpus.

```sh
python bench/open-source-pii/evaluate_external.py \
  --candidate gliner2 --models "$DMZ_PII_TASK/models" \
  --corpus "$DMZ_PII_TASK/datasets/gretel-projected-v2" \
  --output "$DMZ_PII_TASK/external-gliner2-retrial.json"

python bench/open-source-pii/compare_external.py \
  --reference "$DMZ_PII_TASK/external-contained-token-core-replay.json" \
  --candidate "$DMZ_PII_TASK/external-gliner2-retrial.json" \
  --corpus "$DMZ_PII_TASK/datasets/gretel-projected-v2" \
  --output "$DMZ_PII_TASK/current-gliner2-retrial-comparison.json"
```

Repeat `--candidate` when comparing multiple complete candidate caches. A
candidate JSON and its adjacent `.partial.jsonl` are both required. The comparator
checks corpus identity, complete IDs, original text hashes, annotation equality,
candidate-prefix hashes and restoration. Model failures remain abstentions.
For the sample comparison, pass the same `--selection` manifest to inference and
comparison; full-set candidate caches can be rescored on those selected IDs.

The original-core reference is `external-frozen-browser.json`. The current-core
reference is `external-contained-token-core-replay.json`. Export requires an
explicit `--baseline original` or `--baseline current` and verifies its hash:

```sh
python bench/open-source-pii/export_external_summary.py \
  --kind public-models --baseline current \
  --source "$DMZ_PII_TASK/current-gliner2-retrial-comparison.json" \
  --output bench/results/open-source-pii-v1/current-gliner2-retrial.json
```

## Final core and browser confirmation

`replay_core.mjs CORPUS PRIVATE_REFERENCE WASM PRIVATE_OUTPUT` reruns masking
with cached name spans. It does not perform neural inference. The final runtime
verifier checks that replays and specialist/native/WASM audits agree and links
the immutable scores to the tested final binary hashes.

`candidate-browser.mjs PRIVATE_OUTPUT CORPUS EXTENSION BUILD_SHA256` performs
fresh trained inference and both masking modes in Chromium. The extension must
be a recorded private trained build with the approved model, tokenizer and CRF.
The browser script checks assets before and after the run, aborts external HTTP
requests and rejects any observed external request. It appends each successful
excerpt and writes an atomic checkpoint. After an interruption, `--resume` checks
the build/corpus/measurement identity and committed prefix hash before reusing
any records; an uncommitted tail is discarded. Keep evaluator/imported
measurement files and the private build unchanged during the run.

`verify_fresh_browser.py --private PRIVATE_DIRECTORY --output AGGREGATE_PATH`
requires the completed fresh run and all 4,996 original records to match the
scored final replay. A prediction difference fails verification and requires
new scoring rather than reusing the old quality table. A resumed run is retained
as such; the fresh-run confirmation specifically requires zero resumed excerpts.
Neither runtime equality nor reproducibility makes these synthetic/development
scores independent validation.

## Aggregate figure

`plot_public_comparisons.py` reads only the four published model aggregates and
checks both denominators and metric arithmetic. It emits PNG/SVG artwork plus
input/output hashes, refusing existing artifacts. The figure used a separate
reporting virtual environment with `report-requirements.txt`; these packages
were not installed into the model-inference environment or the Shield product.
The plot is a presentation of the same diagnostic scores, not new evaluation.
