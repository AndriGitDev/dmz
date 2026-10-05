# Uncertainty for clustered excerpts

`quality_clusters.py` implements a fixed **descriptive diagnostic** for exact
entity-set accuracy, pooled entity recall and precision within each declared
stratum. It does not establish independent validation or a population guarantee.
The implementation was fixed on 5 October 2026 using fabricated test records,
before receiving independent labels or running predictions on the blind packets.

## Procedure

Each record supplies `excerpt_id`, `document_id`, `cluster_id`, `stratum`, `gold`
and `predicted`, with the same UTF-8 span triples and optional `abstained` flag as
the shared `quality.py` scorer. Every excerpt needs complete labels for the
declared `kinds`. Null labels fail. Callers must verify immutable source and
prediction identities, byte boundaries and annotation completeness first.

Assign related articles to the same cluster before viewing predictions. A
document cannot cross clusters or strata; a related-article cluster cannot cross
strata. Conflicting assignments fail rather than being split silently. Resolve
such cases in the sampling protocol before evaluation. Use fixed, nonidentifying
stratum names: these names and the declared entity types appear in the report.
No source text, span triples or excerpt/document/cluster IDs are emitted.

For each stratum, draw its original number of clusters with replacement 10,000
times. Every selected cluster contributes all its documents and excerpts; a
repeated draw contributes the entire cluster again. Sum confusion and excerpt
counts before computing ratios. Do not average cluster percentages or sample
individual excerpts. Cluster resampling follows the grouped-observation principle
illustrated by [Stata's panel bootstrap example](https://www.stata.com/support/faqs/statistics/bootstrap-with-panel-data/).
Treating related articles as the resampling unit is DMZ's design choice.

The seed is `20261005`. Each stratum gets a separate random stream derived from
SHA-256 of the JSON-encoded seed and stratum. Sort cluster identities first, so
row ordering and adding another stratum do not alter existing draws. Record the
Python runtime version. The two-sided 95% percentile interval uses the 2.5th and
97.5th percentiles, with linear interpolation between order statistics (type 7).
The [SciPy bootstrap reference](https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.bootstrap.html)
describes percentile intervals; this implementation uses Python's standard
library and does not claim SciPy's default BCa correction.

These settings are deliberately not command-line tuning options. Any future
procedure revision must precede its evaluation predictions and be documented as
a new protocol. Per-type counts and point metrics remain in the shared scorer's
output; the bootstrap intervals here cover the three aggregate metrics within
each stratum. There is no pooled cross-stratum uncertainty headline and no
simultaneous-coverage claim across metrics or strata.

## When no interval is reported

- Fewer than two clusters: `insufficient_clusters`.
- Any replicate has an undefined denominator: `undefined_replicates`, with
  defined and undefined replicate counts. Do not discard undefined replicates.
- All replicate values are identical: `degenerate`. In particular, observing no
  errors does not produce a `[1, 1]` population guarantee.

Bounds are null in these cases. An available interval is still an approximate,
sample-dependent diagnostic. Few clusters, rare errors, convenience sampling,
unrecognised dependence and label errors can make it misleading. This procedure
cannot quantify missing coverage or make a selected sample representative.
The 99% point-estimate targets are unchanged; interval availability or bounds
do not authorize acceptance or model promotion.

## Running it

The input is a JSON object with `kinds` and `records`. Both input and output must
be outside the repository. Output creation is exclusive with mode `0600`; an
existing report is never overwritten.

```sh
python bench/contextual/quality_clusters.py \
  --input /private/evaluation/clustered-records.json \
  --output /private/evaluation/clustered-quality.json
```

The report includes the original point metrics, stratum sample sizes, intervals
and statuses, membership hash, input hash and both scorer implementation hashes.
Keep raw evaluation records private. Review even aggregate reports for source-use
and disclosure constraints before publishing them.

The existing Person development caches do not provide verified independent
document/related-article clusters. Do not invent those identities to attach
confidence intervals to the frozen baseline. The newly prepared blind packets
remain unlabelled, unscored and ineligible for a final claim. A sampling design,
independent review and adjudication, source-use review and the separate packaged
system checks remain necessary before an independent acceptance run.
