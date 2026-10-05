# English work suggested by this slice

These priorities are derived after evaluation. They are hypotheses for a new
predeclared experiment, not variants selected or validated on this test set.
The public synthetic annotations may omit PII and overlap candidate training;
all counts below refer only to the projected annotation scope.

## Where the remaining errors are

The updated trained composition on 4,996 public English excerpts has 2,701
false spans and 3,108 missed spans. Person predictions account for 2,503 false
spans (92.7% of all false spans). Address and phone misses account for 2,179
missed spans (70.1% of all misses). This identifies two different problems:
existing English name typing/boundary errors and incomplete contextual address/phone
coverage. Adding a new detector while retaining every existing finding cannot
remove the name errors.

| Type | True spans | False spans | Missed spans |
| --- | ---: | ---: | ---: |
| Name | 1,684 | 2,503 | 575 |
| Address | 0 | 0 | 1,429 |
| Phone | 154 | 131 | 750 |
| Card | 429 | 24 | 233 |
| Email | 1,023 | 37 | 25 |
| Secret | 23 | 6 | 96 |

The zero address matches concern this public English projection; the trained
build also contains a local Icelandic address vocabulary. This table does not
measure that vocabulary's Icelandic coverage. The new phone adapter accepts
explicit international country codes, so many national-format phone cases
remain outside its declared behavior.

The post-evaluation relation analysis records 736 false-name-to-gold-address
overlap pairs. Of the 1,429 missed address spans, 633 partly overlap another core
mask, one is covered under the wrong span/type, and 795 have no entity-span
overlap. A false name may therefore be part of a real address rather than a
benign word. The 1,444 false name spans without projected gold overlap also
cannot be declared benign from these incomplete annotations alone. This supports
testing name/address arbitration together rather than simply removing uncertain
names. Whole-text abstentions are counted separately from absent masks.
See [`current-core-error-relations.json`](../results/open-source-pii-v1/current-core-error-relations.json);
its counts reconcile with every published per-type false/missed count.

## Next experiment

1. Freeze a separately annotated English/general, English/workplace and mixed
   Icelandic/English set before selecting policies. Include complete entity
   sets, truly negative prose, ambiguous names/roles, addresses containing names,
   national and international phones, Unicode and code/identifier boundaries.
   Record annotation agreement and an explicit adjudication process. Keep the
   existing blind Icelandic files isolated from development and tuning.
2. Compare the current trained pipeline, candidate standalone predictions and
   an explicitly specified arbitration policy. Measure lost previously correct
   spans as well as gains. An English name replacement or overlap decision must
   demonstrate mixed/Icelandic safety; document language uncertainty and failure
   handling before inference. Whole-excerpt accuracy and recall per primary
   stratum remain the acceptance metrics.
3. Investigate a compact contextual model or a distillation route with suitable
   training-data licences. The tested GLiNER/GLiNER2 quantizations changed quality
   and browser predictions, while Privacy Filter exceeded the current runtime
   budget. Freeze a device-memory, cold/warm latency and artifact-size budget
   before selecting a replacement. Recheck original UTF-8 span boundaries,
   exact restoration and local-only inference in the real extension.
4. Evaluate national-format phones with an explicit locale contract, including
   ambiguous digit strings and non-phone identifiers. Avoid silently assuming
   US formatting for English or treating numbering-rule validity as proof of
   PII, assignment or reachability.

The existing GLiNER confidence-only address selection rejected every declared
threshold. A lower threshold, address parser or new language policy chosen from
the completed public test errors would need its own untouched evaluation.
GLiNER2 address-only remains a research option: its full-set gain adds 505 true
spans and 297 false spans, so confidence in that supplement's default suitability
has not been established.

Source: [completed aggregate comparisons](../results/open-source-pii-v1/README.md),
including `public-current-core-full-models.json` and
`gliner-public-composition-attribution.json`. No new corpus or policy is enabled
by this document.
