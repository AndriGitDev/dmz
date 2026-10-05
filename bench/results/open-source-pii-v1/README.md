# Open-source PII research — 5 October 2026

The practical integrations in this research branch are portable international
phone metadata and a bounded subset of vendor-secret patterns. Contextual
English models remain experimental. No result here establishes the acceptance
target of 99% exact entity-set accuracy per excerpt and 99% entity recall per
primary stratum. No production deployment or public accuracy claim is implied.

The original detector baseline is commit
`9b711c06ed85509a14d435b421601a39ff848979`. Browser model comparisons use the
frozen trained extension captured before this slice's integrations. Core replays
reuse those exact contextual-name spans with the new WASM core; they do not
rerun the neural name model. Each aggregate records hashes of its private source
reports. Raw inputs, prediction caches, model weights and private corpora remain
outside Git.

## Initial model comparisons

These 100 authored synthetic excerpts were frozen before candidate inference.
They subsequently became model-selection data. Counts below are excerpts with
the complete typed entity set exactly correct, after adding nonoverlapping
candidate spans while retaining existing trained-detector matches.

| Composition | English general | English workplace | Mixed Icelandic/English |
| --- | ---: | ---: | ---: |
| Frozen trained detector | 25/40 | 27/36 | 22/24 |
| + Presidio | 27/40 | 30/36 | 15/24 |
| + BERT Person recognizer | 25/40 | 28/36 | 22/24 |
| + GLiNER FP32 | 31/40 | 27/36 | 12/24 |
| + GLiNER published QUInt8 | 30/40 | 26/36 | 11/24 |
| + GLiNER2 original | 32/40 | 26/36 | 21/24 |
| + OpenAI Privacy Filter | 33/40 | 32/36 | 18/24 |
| + Gravitee compact INT8 | 26/40 | 30/36 | 15/24 |
| + Gravitee compact FP32 | 26/40 | 30/36 | 14/24 |

Most broad additions regress mixed-language exact accuracy. The BERT control
adds only one exact workplace excerpt here and remains a large, incomplete PII
detector. None justifies replacing or enabling a broad default on this evidence.
See `initial-model-comparisons.json` for standalone, basic/trained composition,
per-type recall/precision and false-positive counts.

## Selected address supplement and structured integrations

GLiNER2 address-only at its original 0.5 threshold was selected before authoring
the separate 90-excerpt follow-up. On that follow-up, it changes English exact
accuracy from 32/60 to 40/60 and leaves mixed exact accuracy at 24/30. It adds 14
true spans and six false spans overall. This is a useful research direction,
not a default integration or independently annotated acceptance result.

The phone/vendor core with unchanged frozen contextual-name predictions changes
the initial set from 74/100 to 80/100 exact and the 90-excerpt follow-up from
56/90 to 61/90. In these general fixtures, the gain comes from phone coverage:
six added true spans and one fewer false span initially; five added true spans
and no added false spans on the follow-up. Every replay verifies exact
restoration. See the two `*-structured-replay.json` reports for all strata and
the separately scored experimental address supplement.

Gitleaks is a specialist secret comparator. Its initial comparison uses the
**phone-updated** baseline, so `gitleaks-phone-baseline-comparison.json` is kept
separate from the original-baseline model table. The aggregate exporter rejects
mixed baseline hashes rather than merging incompatible comparisons.

## Runtime constraints

GLiNER2's FP32 ONNX export reproduces all original predictions on the reused
90-excerpt probe, but its NER graphs total about 1.219 GB. Two declared QUInt8
variants reduce that to about 365 MB and lower standalone exact excerpts from
66/90 to 51/90. The local browser version differs on four prediction sets and
scores 54/90. Both quantized variants are rejected for default use.

The publisher's GLiNER QUInt8 graph is about 197 MB. Compared with FP32, it
changes 21/100 initial and 17/90 follow-up prediction sets; standalone exact
counts fall from 67 to 61 and from 54 to 53 respectively. A local Chromium
prototype makes no external requests and completes 90 excerpts without failures,
but differs from native QUInt8 on six prediction sets and scores 52/90. Its
token IDs, word masks and original word offsets match Python on all 190 reused
synthetic inputs. Input parity does not imply inference parity or acceptable
quality. Neither prototype is imported by the Shield product.

## Validation-only address confidence selection

The published GLiNER QUInt8 model was separately evaluated on 4,995 public
synthetic validation excerpts. The threshold grid and decision rule were fixed
before inference: address-only detections needed at least 50 true positives and
a two-sided 95% Wilson lower precision bound of at least 0.99. No threshold
qualified. At 0.7 there were 1,027 true and 52 false spans; at 0.9 there were
83 true and five false spans; at 0.95 only one true span remained. All three
higher thresholds produced no address detections.

This rejects the declared confidence-only route; the guard was not relaxed and
no new default or follow-up was selected from it. The Wilson statistic on this
correlated synthetic data is a selection guard, not certified field precision
or evidence of the user's exact-excerpt/recall target. All fixed trials and
source hashes are retained in `address-confidence-selection.json`.

See the [survey](../../open-source-pii/survey.md),
[GLiNER2 runtime record](../../open-source-pii/runtime-feasibility.md) and
[GLiNER runtime protocol](../../open-source-pii/gliner-runtime-protocol.md) for
pinned sources, licensing distinctions and evaluation limits.

Broader public English and identical-sample Privacy Filter evaluations are still
running. Their results will be recorded separately from these synthetic probes.
