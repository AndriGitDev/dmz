# Private-key block regression corpus

`corpus/private-key-v3.jsonl` contains 15 deliberately synthetic excerpts: ten complete private-key blocks across five labels and two newline conventions, one excerpt containing two blocks, three public-material negatives, and one truncated private-key block. The payloads explicitly say they are synthetic and are not functional keys. The cases were written while developing the block detector; they are regression fixtures, not independent validation.

Previously, the core reported only a private-key header. dmz already responded by masking the entire prompt, protecting the body but losing surrounding prose and abstaining from individual typed spans. Complete blocks now receive one Secret span from opening fence through matching closing fence. The surrounding text is preserved, and restoration retains original bytes. Incomplete, nested or mismatched fences retain whole-prompt fallback; strict core policy still blocks private-key detections.

The detector recognizes key-shaped text rather than validating cryptographic material. Matching labels and newline handling follow the boundary convention described in [RFC 7468, section 2](https://www.rfc-editor.org/rfc/rfc7468#section-2). Public-key, certificate and certificate-request fences are not private-key signals. This does not establish that their contents contain no other identifiers.

The synthetic corpus includes a truncated block so the remaining typed abstention stays visible. Do not relabel that case negative or omit it from the denominator. Per-type masking coverage and exact typed detection remain separate measurements.

## Packaged results

The before/after reports are preserved in `results/quality-99-v1/private-key-before.json` and `private-key-after.json`. Both use the frozen name model; the latter also replays all five earlier synthetic corpora, whose measurements are unchanged.

| Path | Exact excerpts before → after | Exact Secret TP before → after | Typed abstentions before → after | Fully masked identifiers |
| --- | --- | --- | --- | --- |
| Basic | 3/15 → 14/15 | 0 → 12 | 12 → 1 | 13/13 both runs |
| Trained | 2/15 → 13/15 | 0 → 12 | 12 → 1 | 13/13 both runs |

Every allowed result restored exactly in both runs, and no external requests occurred. The truncated case remains a typed abstention. The frozen trained scanner produces two false Person detections in one public-material negative, unchanged between runs. Neither path reaches the 99% point targets on this development corpus.

`GO_BIN=/path/to/go scripts/check-private-key.sh` verifies the corpus hash, exact core report and strict-policy zero-exposure snapshot in CI. Native and WASM tests separately require full block spans, exact restoration, surrounding-prose preservation and whole-prompt fallback for malformed blocks.
