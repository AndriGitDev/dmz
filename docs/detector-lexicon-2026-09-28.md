# Name and address lexicon preview

28 September 2026. Follows the owner-approved [dataset preparation](../bench/datasets/README.md).
This pass builds a small training-derived dictionary and a gazetteer lookup in the
shared Go detector. It does not train a neural model. Portal/Gateway remain deferred.

## Measured name results

MIM Person annotations are scored by exact original-byte span after reconstructing
source tokens with single spaces. All counts are aggregate; text remains local.

| Split/configuration | TP | FP | FN | Precision | Recall |
| --- | ---: | ---: | ---: | ---: | ---: |
| Development, original rules | 198 | 64 | 1,308 | 75.57% | 13.15% |
| Development, minimum 2 occurrences | 437 | 71 | 1,069 | 86.02% | 29.02% |
| Development, selected minimum 1 | 534 | 76 | 972 | 87.54% | 35.46% |
| Reserved test, original rules | 189 | 65 | 1,353 | 74.41% | 12.26% |
| Reserved test, selected lexicon | 530 | 88 | 1,012 | 85.76% | 34.37% |

The training corpus alone supplies candidate name phrases and ambiguity counts.
Two configurations were compared on development data; the selected pack was fixed
before scoring the test split. No tuning followed that test result. Test has now
been evaluated, so future tuning requires a fresh independent evaluation set.
The hash-based splits are exact-text disjoint, not document/entity-disjoint. This
result is not field accuracy, safe handling of arbitrary PII, or a GA release gate.
Absolute false positives increase even though precision improves. The remaining
1,012 missed test spans rule out a claim of comprehensive name coverage.

## Selected data pack

- 4,431 exact case-sensitive name phrases, each occurring as a full Person span
  at least once in training, with at least 95% Person-span/phrase-occurrence ratio
  in training. This threshold is a training filter, not a confidence probability.
- 23,636 canonical address forms from HMS nominative/dative columns. Unsupported
  characters, numeric forms and entries over eight tokens are excluded; repeated
  whitespace is normalised and duplicates removed. No full household addresses,
  property IDs or coordinates are embedded.
- 437,132-byte JSON pack, SHA256
  `f3e3d9470773e481915f8f42c9015c93c4761e37531657b2a7e171c307aa4f31`.
- WASM with this pack: 1,624,712 bytes (previous formatting build: 1,165,250).
  Total unpacked extension approximately 1.68 MB. These are measured build sizes,
  not browser-memory measurements.

The pack lives at `/root/.local/share/dmz/datasets/lexicon-v2.json`, outside Git.
Only aggregate reports under `bench/results/lexicon` are committed. No external
service receives the corpus or user prompts. No Bedrock invocation/spend occurred.

## Runtime behaviour

An immutable token trie supports at most eight tokens per phrase. Original UTF-8
byte offsets are retained. Names are case-sensitive; addresses use case-insensitive
token lookup without lowercasing the original text. Matching does not join lines.
The existing Unicode-review guard runs first. Both new name and address findings
are uncertain, so the current dmz policy **blocks the whole prompt for review**.
They are not automatically masked or approved for sending.

Address lookup requires a known form plus a house number, optional apartment
number, and a three-digit postcode; a city can follow. A bare place/street name
is insufficient. Postcodes are shape-checked, not verified against a postal map.
The existing heuristic remains as fallback. Both paths reject explicit currency
suffixes (`kr`, `krónur`, `króna`, `ISK`), including when the optional city pattern
would otherwise consume `ISK`. This clears the known benign price fixture.
Numbers without a currency suffix can still be ambiguous.

Synthetic tests cover apartment suffixes, dative/lowercase/multiword forms, UTF-8
spans, initials, word boundaries, line boundaries, price negatives, Unicode review,
secrets and tenant-term coexistence. Existing synthetic per-kind recall floors
pass with and without the selected pack. Updated exploratory/holdout snapshots
remove the price false positive; their exposed-literal counts remain unchanged.
No labelled real-address recall or field false-positive rate is claimed.

## Reproduce and build

```sh
python3 bench/datasets/train_lexicon.py \
  --train /root/.local/share/dmz/datasets/prepared-v2/train.jsonl \
  --addresses /root/.local/share/dmz/datasets/prepared-v2/address-forms.json \
  --minimum 1 --purity 0.95 \
  --output /root/.local/share/dmz/datasets/lexicon-v2.json

go run ./bench/ner-eval \
  -input /root/.local/share/dmz/datasets/prepared-v2/dev.jsonl \
  -lexicon /root/.local/share/dmz/datasets/lexicon-v2.json

DMZ_LEXICON_FILE=/root/.local/share/dmz/datasets/lexicon-v2.json \
  ./scripts/build-wasm.sh
cd apps/shield
npm run build
DMZ_LEXICON_FILE=/root/.local/share/dmz/datasets/lexicon-v2.json \
  node scripts/wasm-smoke.mjs
npm run test:browser
npm run record:build
```

Use `GO_BIN` and `TINYGO_BIN` overrides if the tools are outside PATH. Recording
requires a clean committed tree. The build stages an ignored generated embed file
and removes it on exit. Supplying no pack builds the ordinary baseline; the
standard CI artifact therefore **does not contain this external lexicon**.
`detector-data.json` identifies pack hash/counts in the enriched build and is
covered by the build-record checksums. No runtime download or remote lookup occurs.
CI exercises embedding with a fabricated fixture through `check-lexicon.sh`.

## Verification and next live checks

Go unit tests (ordinary and embedded data), Python trainer tests, corpus pins,
recall gates and vet pass. The local enriched WASM passes its optional pack smoke
check, existing mask/restore checks, synthetic supported AI sites interception and
restoration tests, and network-body/composer-race/broken-core checks.
Authenticated live-site confirmation remains pending.

After loading the enriched local preview, refresh both AI tabs. Try fictional/test-only
examples on supported AI sites:

- `Skálduð persóna: Sherlock Holmes.` → review block from learned name entry.
- `Tilbúið heimilisfang: Laugavegur 999/2, 000 Prófstaður.` → address review block.
- `Verð: Prófgata 99, 999 krónur.` → no address block.
- `demo@example.com` → existing masking and local restoration still work.

The fictional character and invented address examples are test-only. Expect a review block for
new lexicon findings, not an automatically redacted send. Generalisation to unseen
names, name inflection and contextual ambiguity needs more work; a context-aware
local model is a later comparison, subject to device size/latency requirements.

## Subsequent precision round

The owner requested a 95% precision target. A stricter pack with learned name
boundaries now reaches 96.22% exact-name precision on the reused test set, with
31.39% recall. See [the full trade-off and reproduction](detector-precision-2026-09-28.md).
