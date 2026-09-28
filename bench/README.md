# Synthetic detector benchmark v0

`corpus/v0.jsonl` has 42 synthetic examples. Every positive annotation records an entity type and the exact visible string, from which the scorer derives a UTF-8 byte span. The scorer compares exact span **and** entity type. It counts true positives, false positives and false negatives per type, then reports precision and recall in `results/v0.json`.

Run from the repository root:

```sh
go run ./bench/scorers
go run ./bench/scorers -baseline bench/results/v0.json
go run ./bench/scorers -detector regex
go run ./bench/scorers -mode exposure
go run ./bench/scorers -corpus bench/corpus/challenge-v0.jsonl
go run ./bench/scorers -corpus bench/corpus/challenge-v0.jsonl -mode exposure
python3 -m venv /tmp/dmz-presidio
/tmp/dmz-presidio/bin/pip install -r bench/baselines/requirements.txt
/tmp/dmz-presidio/bin/python bench/baselines/presidio.py
```

The baseline option fails if recall falls below the published result for any type. CI also compares generated JSON against the committed result, requiring an intentional results update when code or corpus changes.

`results/exposure-v0.json` answers a different question: with the prototype's fixed mask/block policy and one configured synthetic tenant term, how many annotated direct-identifier literals remain in **allowed outbound text**? Each annotation is counted as masked, blocked with its whole prompt, or exposed. On this selected corpus, no annotated literal remains in allowed text: two unfamiliar patronymic-shaped names and one address shape stop their prompts for review, with ten prompts blocked in total. This is **not a zero-leak finding**: the corpus is small, selected, and omits quasi-identifiers and many direct-identifier forms. The simulation calls no model and sends no prompt over the network.

`corpus/challenge-v0.jsonl` is a separate 18-case synthetic challenge set, deliberately written after the first detector rules and **not used to tune them**. It includes alternate name and address forms, token prefix families, an internationalised email and a phone prefix variant. The exact-span results in `results/challenge-v0.json` show 50% address recall, 50% email recall, 60% name recall, 50% phone recall and 33.3% secret recall on this tiny set. More directly, `results/challenge-exposure-v0.json` records **8 annotated direct identifiers still present in allowed outbound text**. These are known misses. The figures are neither statistically representative nor a safe-use threshold. Do not send sensitive work through this prototype without independent controls.

For the public website, run `./scripts/sync-public-benchmark.sh` from the product repository. It copies only this synthetic corpus and aggregate result JSON into the sibling `dmz-web/public/benchmark` directory. Review both copies before publishing.

These results **do not** establish production detection quality. The corpus is tiny and partly built around the initial rules. All seven name examples are detected, but only five are masked; the other two match an uncertain patronymic shape and block the prompt. No general Icelandic name lexicon is loaded. The one address example matches a narrow street-suffix/number/postcode/city heuristic and blocks; no gazetteer is loaded. The dmz scorer configures one exact synthetic tenant term, `Verkefni Gervi`; that one example is not evidence of general dictionary quality. `results/regex-v0.json` is a simple pattern-only comparison on the same corpus, without date/checksum/IBAN/Luhn checks or a lexicon. Its recognizers can overlap, so it generates many false positives. `results/presidio-v0.json` scores Presidio Analyzer 2.2.364 with its default English `en_core_web_lg` model and recognizers, mapped to matching benchmark types. It has no Icelandic recognizer or tenant dictionary configuration. Expand the corpus independently, add hard negatives and licensed-data-safe name forms before publishing broad quality claims.

Committed kennitölur use century digit `8` and cannot belong to living people. They include both matching legacy check digits and plausible nonmatching shapes to cover the February 2026 rule change. Every account, email, phone, card and secret is a synthetic or reserved test value. No real prompts, logs, customer data or licensed lexicon data belong here.
