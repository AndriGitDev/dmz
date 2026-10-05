# Long-prompt boundary baseline

This completes the [predeclared diagnostic](long-context-protocol.md) with the frozen packaged model.

The frozen trained path passes all four groups. The basic path misses the two
English-style names in each group, retaining them as labelled failures.

| Group | Basic exact excerpts | Trained exact excerpts | Basic / trained fully masked names | Exact restorations, each path | Trained fallbacks |
| --- | --- | --- | --- | --- | --- |
| Short control | 7/9 | 9/9 | 6/8 / 8/8 | 9/9 | 0 |
| Name crosses a chunk cut | 7/9 | 9/9 | 6/8 / 8/8 | 9/9 | 0 |
| Nearby interior control | 7/9 | 9/9 | 6/8 / 8/8 | 9/9 | 0 |
| Crossing, NFD | 7/9 | 9/9 | 6/8 / 8/8 | 9/9 | 0 |

Each trained group has TP8/FP0/FN0. Each basic group has TP6/FP0/FN2. External
requests are zero. The [packaged report](../results/quality-99-v1/long-context-baseline.json)
records artifact and runtime hashes. Strict native processing protects six names
and leaves the two misses exposed per group; pinned native snapshots preserve
that existing floor without pretending it meets the owner’s target.

A separate fixed-.75 [CRF-only diagnostic](../results/quality-99-v1/long-context-crf.json)
also finds all eight complete names in each group, without extra detections.
Therefore these cases do not isolate neural-only boundary robustness: the CRF
can supply every gold name regardless of what the neural head does at the cut.
This result supplies no reason to change production chunking. Such a change would
need different evidence and a separately declared comparison.

The positive long cases contain two encoder chunks and are at most1,672 UTF-8
bytes, while the name-free long filler control still fits one chunk. This is a
small boundary-position diagnostic, not a test of maximum supported input size,
multiple distant cuts, arbitrary names, informal casing or long negative text
that itself crosses a cut. The four groups are correlated and must not be pooled
into representative accuracy. Training ran concurrently; elapsed times are not
latency measurements. No production behavior or model was changed.
