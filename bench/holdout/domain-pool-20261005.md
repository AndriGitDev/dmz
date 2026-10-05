# Legal and financial review pool — 5 October 2026

The additional private collection contains 59 downloaded documents and 145
unlabelled excerpts (32,004 words). It adds tax rulings and fund-manager articles;
it is neither a representative customer sample nor an independently scored test
set. No labels or model predictions were used for selection or allocation.

## Selection and extraction

Ten explicit public index pages were checked with the existing bounded,
robots-respecting fetcher. Seven were accessible. The two Íslandsbanki index
requests were disallowed by robots rules, and Samkeppniseftirlitið returned HTTP
403; neither restriction was bypassed. Other accessible indexes did not supply
additional matching article links.

`expand_domain_sources.py` freezes unused URLs from the saved
[Yfirskattanefnd 2026](https://yskn.is/urskurdir/?year=2026),
[Yfirskattanefnd 2025](https://yskn.is/urskurdir/?year=2025) and
[Íslandssjóðir news](https://islandssjodir.is/frettir/) indexes. A fixed hash order
selects at most 40 documents per publisher: 40 tax rulings and 19 financial
articles. `domain-sources-20261005.json` preserves the URLs and index hashes.
Collection date does not establish publication novelty or absence from pretraining.

The initial downloads preserved all HTML, but the generic extractor could not
locate the tax-ruling body. A host-specific `div#resultviewer` selector now captures
that bounded viewer. Tests use fabricated HTML and confirm that unrelated content
stays outside the excerpt. Re-extraction uses checksummed cached downloads without
new requests; all 19 financial document extractions remain unchanged. This is
structural validation, not human verification of the source extraction.

Raw HTML, full text and excerpts remain outside Git under
`/root/.local/share/dmz/datasets/domain-web-pool-20261005-v3`. The earlier download
and extraction run is preserved as `domain-web-pool-20261005-v2`. Only public source
URLs, tool changes and aggregate evidence are committed. The
[collection aggregate](../results/quality-99-v1/domain-web-collection.json) records
provenance and limitations without source text or individual predictions.

## Frozen allocation

Overlap checks use 38 prior text inventories, including 234 checksummed full web
documents, previous synthetic and MIM data, and the original blind packet. The
same conservative exclusion rule removes a whole document with any normalized
exact 12-gram overlap. This excludes 29 documents; three more are collection
near-duplicates. Exclusion can reflect shared legal wording rather than copied
case facts. It does not establish semantic independence for retained documents.

The 27 retained documents contain 58 excerpts and 11,695 words. The existing
transitive 20% containment grouping and fixed 60/20/20 hash-bucket rule produce
26 groups; the related pair stays in one split. The seed was not redrawn to change
the resulting allocation.

| Review role | Documents | Excerpts | Legal excerpts | Financial excerpts |
| --- | --- | --- | --- | --- |
| Training review | 21 | 49 | 27 | 22 |
| Development review | 1 | 1 | 0 | 1 |
| Reserved review | 5 | 8 | 3 | 5 |

This is still severely underpowered and unbalanced. Tax rulings do not cover the
range of legal documents, financial material comes from one fund manager, and no
mixed-chat data was added. Do not pool these counts into a headline acceptance
claim. See the [allocation aggregate](../results/quality-99-v1/domain-review-splits.json)
for hashes and counts. Split membership stays private in
`/root/.local/share/dmz/datasets/domain-review-splits-20261005-v1`.

Each split has a separate full-entity packet and separate reviewer A/B archives
with the offline editor. All labels remain null. No packet has been opened in an
automated browser, run through the detector, sent to reviewers or used in training.
The original 142-excerpt blind packet retains SHA-256
`18e32974981aed65237247926fca5a3a9a7ef3b06287ac1e8cf03e7f1d6919b6`.
Earlier allocations and reviewer packets remain unchanged.

Human extraction checks, complete independent annotation, adjudication, source-use
review and a predeclared balanced sampling plan remain required. The new materials
are private review candidates; they do not authorize training, redistribution or
an independent 99% quality claim.
