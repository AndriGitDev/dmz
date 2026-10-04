# Address vocabulary format audit

This audit checks complete address spans for mechanically generated variants of an approved address vocabulary. It found that Unicode hyphens (`U+2010` and non-breaking `U+2011`) were missing from street lookup and house-number ranges. Address lookup now accepts those characters and preserves original byte offsets. Name vocabulary matching is unchanged.

Run from the repository root, using an approved address-only pack outside the checkout:

```sh
go run ./bench/address-audit \
  -lexicon /private/address-pack.json \
  -output /private/new-address-format-report.json \
  -failures /private/new-address-format-failures.jsonl
```

The source and output paths must resolve outside the checkout. Output files are created exclusively with mode `0600`; existing files are never overwritten. The optional failure file contains source text and must remain private. `-failure-limit` defaults to 100 examples across all formats. Standard error prints aggregate progress only.

Each positive case passes only when exactly one Address detection covers the entire generated value. Partial matches fail. Negatives require no Address detection. Other entity types are outside this audit's scope. A format with zero applicable forms is untested, not a successful result.

The 4 October audit used 23,636 approved forms. Both Unicode range variants improved from 0 to 23,636 complete detections, and both hyphenated-street variants improved from 0 to 1,724. All previously passing formats and price/overlong-number negatives stayed passing. No apostrophe-bearing forms were available, so that variant remains untested. Aggregate before/after reports are in `bench/results/quality-99-v1/address-format-*.json`.

This is a vocabulary regression check, not independent validation or a representative estimate of address accuracy. Generated contexts do not establish whether real prose refers to an address. The project's ≥99% exact entity-set accuracy and recall targets still require independently reviewed excerpts across all supported entity types.
