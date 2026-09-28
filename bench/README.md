# Synthetic detector benchmarks

`corpus/v0.jsonl` has 42 synthetic examples. Every positive annotation records an entity type and the exact visible string, from which the scorer derives a UTF-8 byte span. The scorer compares exact span **and** entity type. It counts true positives, false positives and false negatives per type, then reports precision and recall in `results/v0.json`.

Run from the repository root:

```sh
go run ./bench/scorers
go run ./bench/scorers -baseline bench/results/v0.json -floor bench/gates/selected-v0-recall.json
go run ./bench/scorers -detector regex
go run ./bench/scorers -mode exposure -max-exposed 0
go run ./bench/scorers -corpus bench/corpus/challenge-v0.jsonl -floor bench/gates/challenge-v0-recall.json
go run ./bench/scorers -corpus bench/corpus/challenge-v0.jsonl -mode exposure -max-exposed 2
go run ./bench/scorers -corpus bench/corpus/exploratory-v1.jsonl -floor bench/gates/exploratory-v1-recall.json
go run ./bench/scorers -corpus bench/corpus/exploratory-v1.jsonl -mode exposure -max-exposed 13
python3 -m venv /tmp/dmz-presidio
/tmp/dmz-presidio/bin/pip install -r bench/baselines/requirements.txt
/tmp/dmz-presidio/bin/python bench/baselines/presidio.py
```

The baseline option fails if recall falls below the referenced report for any type. CI also uses separate per-type recall floors under `gates/` and outbound exposure caps; updating a result snapshot alone cannot lower these gates. Existing corpus versions are pinned by `gates/corpus.sha256`; add new cases in a new versioned corpus instead of silently rewriting one. Any gate or corpus pin change requires explicit review and evidence. CI compares generated JSON against committed results, requiring an intentional results update when code or corpus changes.

`results/exposure-v0.json` answers a different question: with the prototype's fixed mask/block policy and one configured synthetic tenant term, how many annotated direct-identifier literals remain in **allowed outbound text**? Each annotation is counted as masked, blocked with its whole prompt, or exposed. On this selected corpus, no annotated literal remains in allowed text: two unfamiliar patronymic-shaped names and one address shape stop their prompts for review, with ten prompts blocked in total. This is **not a zero-leak finding**: the corpus is small, selected, and omits quasi-identifiers and many direct-identifier forms. The simulation calls no model and sends no prompt over the network.

`corpus/challenge-v0.jsonl` is a separate 18-case synthetic challenge set, deliberately written after the first detector rules. Its original, **untuned** results are frozen in `results/challenge-untuned-v0.json` and `results/challenge-untuned-exposure-v0.json`: 8 annotated direct identifiers remained in allowed outbound text. The set includes alternate name and address forms, token prefix families, an internationalised email and a phone prefix variant.

The detector has since been tuned against those cases. The current exact-span report in `results/challenge-v0.json` measures 100% recall for address, email and phone, 80% for names, and 66.7% for secrets **on these same 18 cases**. The current `results/challenge-exposure-v0.json` records **2 annotated literals still present in allowed outbound text**: an ASCII-only fabricated name and a generic service token. Six other formerly exposed literals now stop their prompts for review or blocking. This is a regression fixture, not an independent estimate of detector quality. It cannot establish field recall, precision, or safe use; do not send sensitive work through this prototype without independent controls.

The **original untuned** `corpus/exploratory-v1.jsonl` evaluation is a further 29-case synthetic probe written after the targeted fixes and scored without tuning the detector to it. The committed corpus SHA-256 is `9fbd1039bf26cc85decb6629f611c8c4aa92383c5b108b0b221fd694af4c07aa`. It deliberately samples unsupported formatting, name, address, token and dictionary variants and includes four negatives; it is **not** a random or field-representative sample. `results/exploratory-untuned-exposure-v1.json` records **13 annotated literals in allowed outbound text**. They comprise three addresses, two kennitölur, two names, two secrets, one email, one phone, one card and one differently cased tenant term. One benign price line is falsely flagged as an address, and a harmless combining-mark word is blocked for Unicode review. The exact-span report also counts a partial phone match as a false positive even though that prompt is blocked. These results show both coverage gaps and review burden; do not combine counts across the sets into a field leak rate.

For the public website, run `./scripts/sync-public-benchmark.sh` from the product repository. It copies only this synthetic corpus and aggregate result JSON into the sibling `dmz-web/public/benchmark` directory. Review both copies before publishing.

These results **do not** establish production detection quality. The corpus is tiny and partly built around the initial rules. All seven name examples are detected, but only five are masked; the other two match an uncertain patronymic shape and block the prompt. No general Icelandic name lexicon is loaded. The one address example matches a narrow street-suffix/number/postcode/city heuristic and blocks; no gazetteer is loaded. The dmz scorer configures one exact synthetic tenant term, `Verkefni Gervi`; that one example is not evidence of general dictionary quality. `results/regex-v0.json` is a simple pattern-only comparison on the same corpus, without date/checksum/IBAN/Luhn checks or a lexicon. Its recognizers can overlap, so it generates many false positives. `results/presidio-v0.json` scores Presidio Analyzer 2.2.364 with its default English `en_core_web_lg` model and recognizers, mapped to matching benchmark types. It has no Icelandic recognizer or tenant dictionary configuration. Expand the corpus independently, add hard negatives and licensed-data-safe name forms before publishing broad quality claims.

Committed kennitölur use century digit `8` and cannot belong to living people. They include both matching legacy check digits and plausible nonmatching shapes to cover the February 2026 rule change. Every account, email, phone, card and secret is a synthetic or reserved test value. No real prompts, logs, customer data or licensed lexicon data belong here.

## Formatting tuning v2

See [the tuning report](../docs/detector-tuning-2026-09-28.md) for changes, data
sources and live Shield checks. Current exploratory exposure is **8** (previously
13); name/address/generic-secret/case-variant-term gaps remain. Its original
snapshots are retained as `exploratory-untuned-*`.

`format-v2.jsonl` has 41 examples authored before this change (23 identifier
annotations and 18 benign prompts). Untuned reports preserve 14 exposed literals;
current reports have zero exposed annotations and zero false detections.
`holdout-v2.jsonl` is a fresh 30-case post-change probe, not tuned against: eight
literals remain exposed and two of ten benign prompts are blocked. Both sets are
synthetic, deliberately selected, and authored by the implementing agent; neither
estimates field accuracy. Do not pool their results into a headline success rate.

Exposure reports now identify exposed cases and benign prompts blocked or altered.
An unannotated row must explicitly mean benign, not incomplete annotation.

```sh
./scripts/check-detector-v2.sh
go test ./...
```

CI checks the new corpus pins separately in `gates/corpus-v2.sha256`. Existing
pins and floors are unchanged. Updating the new pins or weakening the format
regression assertions requires review, as do existing gates.
