# Reused split overlap audit

This read-only audit compares the fixed 46,196-row training mixture against
5,582 general development excerpts and 256 original synthetic-domain excerpts.
It performs no inference and changes no text, labels, splits, thresholds or
candidate-selection decisions. The protected blind and review packets are not
inputs. Results and source/code hashes are in the
[aggregate record](../results/quality-99-v1/split-overlap-audit.json).

| Token-sequence comparison | Unique training sequences | General matching rows | General matches with at least five tokens | Domain matching rows |
| --- | --- | --- | --- | --- |
| Exact | 46,195 | 0 | 0 | 0 |
| NFC per token | 46,195 | 0 | 0 | 0 |
| NFC then case-fold per token | 46,005 | 7 | 1 | 0 |

All seven case-folded general matches are Person-negative and have a projected
label sequence already present in training. Six are shorter than five tokens.
There is one duplicated exact training token sequence; its projected labels
have two different versions. Case folding produces 190 duplicated training
sequence groups in total, again with one group containing multiple projected
labelings. These are descriptive counts, not human adjudication of label errors.

The label comparison uses the training model's nine-class projection: O and
B/I-Person, Organization, Location and Miscellaneous. Other annotated types,
such as Date, Time, Money and Percent, map to background; their token boundaries
are still validated. “Multiple labelings” therefore refers to this projection,
not a complete comparison of every source annotation type. Capitalization or
unavailable document context can legitimately affect entity interpretation.

No denominator is removed and no frozen score is recomputed. Short repeated
phrases and case-folded matches do not by themselves demonstrate problematic
leakage; absence of such matches does not prove document independence. The audit
does not detect paraphrases or shared related-article context. The general split
remains sentence-based, and all existing development selection remains reused.
Independent document/cluster-separated review and evaluation are still required.

`split_overlap.py` reads private canonical prepared corpora, writes only aggregates
and hashes to a new private output, and refuses to overwrite an audit. Six tests
cover normalization/case distinctions, duplicates versus multiple labelings,
short-sequence counts, separate strata, source-split validation and background
projection without bypassing annotation-boundary validation.

```sh
python bench/contextual/split_overlap.py \
  --train /private/train-domain.jsonl \
  --general /private/general-dev.jsonl \
  --domain /private/domain-dev.jsonl \
  --output /private/split-overlap-audit.json
```
