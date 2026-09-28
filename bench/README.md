# Synthetic detector benchmark v0

`corpus/v0.jsonl` has 42 synthetic examples. Every positive annotation records an entity type and the exact visible string, from which the scorer derives a UTF-8 byte span. The scorer compares exact span **and** entity type. It counts true positives, false positives and false negatives per type, then reports precision and recall in `results/v0.json`.

Run from the repository root:

```sh
go run ./bench/scorers
go run ./bench/scorers -baseline bench/results/v0.json
go run ./bench/scorers -detector regex
python3 -m venv /tmp/dmz-presidio
/tmp/dmz-presidio/bin/pip install -r bench/baselines/requirements.txt
/tmp/dmz-presidio/bin/python bench/baselines/presidio.py
```

The baseline option fails if recall falls below the published result for any type. CI also compares generated JSON against the committed result, requiring an intentional results update when code or corpus changes.

For the public website, run `./scripts/sync-public-benchmark.sh` from the product repository. It copies only this synthetic corpus and aggregate result JSON into the sibling `dmz-web/public/benchmark` directory. Review both copies before publishing.

These results **do not** establish production detection quality. The corpus is tiny and partly built around the initial rules. The seven name examples yield about 71% recall, and no general Icelandic name lexicon is loaded. Address recall is 0 because that detector is not implemented. The dmz scorer configures one exact synthetic tenant term, `Verkefni Gervi`; that one example is not evidence of general dictionary quality. `results/regex-v0.json` is a simple pattern-only comparison on the same corpus, without date/checksum/IBAN/Luhn checks or a lexicon. Its recognizers can overlap, so it generates many false positives. `results/presidio-v0.json` scores Presidio Analyzer 2.2.364 with its default English `en_core_web_lg` model and recognizers, mapped to matching benchmark types. It has no Icelandic recognizer or tenant dictionary configuration. Expand the corpus independently, add hard negatives and licensed-data-safe name forms before publishing broad quality claims.

Committed kennitölur use century digit `8` and cannot belong to living people. They include both matching legacy check digits and plausible nonmatching shapes to cover the February 2026 rule change. Every account, email, phone, card and secret is a synthetic or reserved test value. No real prompts, logs, customer data or licensed lexicon data belong here.
