# Verified private Shield preview — 5 October 2026

The clean trained preview from `78a558b535e083af7227d14845d6fdb690aeefa7` passes WASM, synthetic settings/chat/race browser checks, all 16 scanner workflows and the 19-corpus replay. This is development evidence, not independent acceptance or a production release. The frozen model is unchanged; the separate auxiliary-training experiment is not included.

All 30 ZIP members match the recorded build, including `BUILD.json`. The replay's package file map matches the same build, and fault-injection fixtures restore every packaged file before replay. The stalled-worker workflow exercised the actual deadline (121,501 ms). No scanner requests left the browser.

- Archive SHA-256: `d0af08319d50c7a21601ab187e0bac41e747abe045e418a91a69070f115f70e8`.
- Archive bytes: 233858459. The archive stays outside Git.
- [Verification and measurement hashes](results/quality-99-v1/preview-78a558b-verified.json).
- [Complete per-type and per-corpus measurements](results/quality-99-v1/preview-78a558b-system.json).
- [Scanner workflow results](results/quality-99-v1/preview-78a558b-workflows.json).

Every quality, masking and restoration metric matches the preceding 15-corpus marker replay and four long-context baselines. Four trained scans report `unsupported_input` and use basic fallback; all other scans avoid fallback. Runtime under concurrent training is not a latency benchmark.

| Synthetic corpus | Basic exact excerpts | Trained exact excerpts | Trained typed recall | Trained fully masked |
| --- | --- | --- | --- | --- |
| v0.jsonl | 42/42 | 42/42 | 37/37 | 37/37 |
| challenge-v0.jsonl | 16/18 | 17/18 | 15/16 | 15/16 |
| exploratory-v1.jsonl | 23/29 | 25/29 | 21/25 | 22/25 |
| format-v2.jsonl | 41/41 | 41/41 | 23/23 | 23/23 |
| holdout-v2.jsonl | 25/30 | 25/30 | 16/20 | 17/20 |
| private-key-v3.jsonl | 14/15 | 13/15 | 12/13 | 13/13 |
| database-url-v3.jsonl | 25/25 | 25/25 | 15/15 | 15/15 |
| bearer-v3.jsonl | 25/25 | 25/25 | 15/15 | 15/15 |
| github-token-v3.jsonl | 32/32 | 32/32 | 15/15 | 15/15 |
| credential-context-v3.jsonl | 36/42 | 36/42 | 19/25 | 19/25 |
| credential-expression-v3.jsonl | 17/21 | 17/21 | 10/10 | 10/10 |
| email-shape-v3.jsonl | 56/56 | 56/56 | 39/39 | 39/39 |
| email-context-v3.jsonl | 17/17 | 17/17 | 15/15 | 15/15 |
| aws-credentials-v3.jsonl | 35/38 | 35/38 | 21/23 | 21/23 |
| presigned-url-v3.jsonl | 40/40 | 40/40 | 30/30 | 30/30 |
| long-context-short-v3.jsonl | 7/9 | 9/9 | 8/8 | 8/8 |
| long-context-crossing-v3.jsonl | 7/9 | 9/9 | 8/8 | 8/8 |
| long-context-interior-v3.jsonl | 7/9 | 9/9 | 8/8 | 8/8 |
| long-context-nfd-v3.jsonl | 7/9 | 9/9 | 8/8 | 8/8 |

Both paths restore every excerpt exactly. Full masking can succeed through whole-text fallback while typed extraction fails; exact restoration does not prove complete detection. Do not pool the selected/reused corpora, especially the correlated long-context groups, into a headline accuracy estimate. The long-context names are all detected by the CRF alone, so those controls do not isolate neural boundary behavior.

The agreed acceptance contract remains ≥99% exact entity-set accuracy and ≥99% entity recall per declared stratum. The reused Person development baselines remain below that joint target, and protected independent annotations are still pending. See [the quality contract](../docs/detector-quality-contract.md) and [long-context limits](contextual/long-context-result.md).
