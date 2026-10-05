# Exact other-type veto result

**Retain the frozen reference.** All four typed replays completed under the
[predeclared protocol](entity-veto-protocol.md), frozen at commit `ed30de5`.
Every replay reproduced the corresponding original Person predictions,
confidences and gold cache exactly. Neither complete checkpoint passes the
unchanged research gate: precision improves, but general Person recall falls.

| Checkpoint | General exact excerpts | General TP / FP / FN | General recall | Domain exact excerpts | Domain TP / FP / FN |
| --- | --- | --- | --- | --- | --- |
| Frozen reference | 5502/5582 | 1461 / 68 / 45 | 97.0120% | 250/256 | 474 / 3 / 6 |
| Epoch 1, veto | 5502/5582 | 1450 / 58 / 56 | 96.2815% | 250/256 | 474 / 1 / 6 |
| Epoch 2, veto | 5508/5582 | 1454 / 56 / 52 | 96.5471% | 252/256 | 476 / 1 / 4 |

Relative to the original same-checkpoint combination, epoch one removes 14
false names and nine true names; epoch two removes 15 false names and six true
names. Neither domain set has a veto. The complete
[comparison](../results/quality-99-v1/entity-veto-comparison.json) and
[provenance](../results/quality-99-v1/entity-veto-provenance.json) retain all
candidates, failed criteria, input/cache/code hashes and the unchanged decision.
The four `epoch-*-*-veto.json` files retain original and revised quality and harm
counts. No raw text or individual predictions are committed.

There is also a structural limit, identified while the replays were running:
this exact-span rule cannot increase Person recall. Neural typed spans are
non-overlapping; a removed CRF span equals a non-Person span and therefore cannot
have blocked any neural Person span. Removing it cannot add a new true Person.
Since both original candidates already have lower general recall than the
reference, the recall-improvement gate was unreachable for this rule alone.
The experiment quantifies its precision/recall tradeoff; it does not justify
weakening that gate or relabelling lost true names as success.

The separate [mechanical chunk-boundary audit](../results/quality-99-v1/chunk-boundary-audit.json)
finds one encoder chunk per excerpt in all 5,582 general and 256 domain records.
There are no split excerpts or gold Person spans crossing a chunk boundary.
Window overlap would therefore not address the measured errors in these two
sets. This says nothing about long user prompts, which require separate coverage.

These are reused, Person-scoped development results. The original training trial
remains rejected, the three-label browser model remains unchanged, and no
independent review packets were read or scored. A different combination would
be a separate, explicitly counted experiment, not a revision of these results.
