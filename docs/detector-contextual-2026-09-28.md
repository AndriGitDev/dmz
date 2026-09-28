# Contextual name experiment — 28 September 2026

## Outcome

The requested **95.5% exact name precision and 95.5% exact name recall** target is
not achieved. No Shield runtime, local preview artifact, or public website metric
is changed by this research pass.

| Evaluation | Current lexicon precision | Current lexicon recall | Experimental CRF precision | Experimental CRF recall |
| --- | ---: | ---: | ---: | ---: |
| Development, 5,582 sentences | 96.24% | 66.20% | 95.98% | 84.06% |
| Reused MIM test, 5,758 sentences | 95.55% | 65.37% | 95.26% | 84.70% |
| New synthetic legal/financial probe, 40 snippets | 92.00% | 62.16% | 96.88% | 83.78% |

On the reused MIM test, CRF counts are 1,306 true positives, 65 false positives,
and 236 false negatives, versus 1,008 / 47 / 534 for the current lexicon. That is
298 more exact names detected, with 18 more false detections. The domain probe has
31 / 1 / 6 versus 23 / 2 / 14. A partial name is a false positive plus a false
negative; no token-accuracy or F1 substitution is used.

These are **Person-only** scores on token-spaced text. The CRF is evaluated alone,
not combined with the Go detector, masking policy, or browser. No outbound privacy
or production-quality claim follows from these numbers.

## Method and selection

Two local linear-chain CRFs were trained on the existing 45,332-sentence approved
MIM-GOLD-NER training split. No test or domain annotations entered training.

- Feature v1: word, shape, affixes, patronymic flag, and two-token context;
  100 iterations, c1=0.1, c2=0.1; model 2,141,268 bytes.
- Feature v2: adds neighboring affixes and word/shape pairs;
  150 iterations, same regularization; model 3,590,324 bytes.
- Both used the fixed development threshold grid
  `0, .5, .6, .65, .7, .75, .8, .9, .95`.
- Select the configuration with highest development recall subject to precision
  >=95.5%. V1 at 0.65 won: 95.98% / 84.06%; V2 at 0.65 gave 95.53% / 83.67%.
- Selection was written before the final test and domain evaluations. The
  threshold was not raised after observing test precision below target.

Minimum predicted BIO token marginal supplies the confidence ranking. It is not a
calibrated whole-name probability. Training uses python-crfsuite 0.9.12, entirely
locally. The source corpus, model weights, and vocabulary stay outside Git.

[Aggregate reports](../bench/results/contextual-v1/) preserve both development
runs, training parameters and hashes, selected configuration, and final outcomes.
[Instructions](../bench/contextual/README.md) reproduce training and the gate.

## Limits and next experiment

The reused MIM test has informed earlier work, and sentence-based splits are not
document- or entity-disjoint. The new legal/financial probe is frozen and contains
fabricated names, but has only 37 name annotations and was written by the same
agent implementing the experiment. Neither supports a field-wide 95.5% claim.
The current results make this probe an exposed evaluation set for future work.

Next work should develop a larger document-disjoint legal/financial corpus with
independently reviewed exact name spans, including negatives, inflections, initials,
and names absent from training. Representative fabricated templates supplied by
the user can guide coverage; training and final evaluation documents must remain
separate. After that, compare a pretrained Icelandic contextual encoder with this
CRF baseline, using a model whose fine-tuning has not already consumed the final
evaluation corpus. Compute requirements, model license, and any paid training
route need verification before that experiment. More context features alone did
not close the gap in this round; a larger model is a hypothesis, not a guarantee.

Before any model enters Shield, implement and test production tokenization and
local inference, measure WASM/package size and latency, and run the existing
per-entity recall and browser regression gates. The offline CRF does not satisfy
those runtime requirements.

## Verification

- Seven contextual tests, including fabricated end-to-end train/evaluate,
  metadata mismatch rejection, exact UTF-8 spans, and dual-target gate checks.
- Six existing dataset tests and `go test ./...` pass.
- Legal/financial corpus SHA-256 pin passes.
- Both selected-model evaluations fail the 95.5% dual gate, as expected.
