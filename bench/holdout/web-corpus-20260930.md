# Fresh Icelandic web candidates, 30 September 2026

Status: **collected and frozen for blind annotation, not labelled gold or an
independent accuracy result**. No model inference or training used this corpus.

The owner requested fresh scraped Icelandic content. Source selection used public
index snapshots and a fixed seed, without conditioning on detected names or
errors. The frozen pool contained 90 URLs from eight publishers. Up to 12 URLs
per publisher (eight Ombudsman cases and ten privacy-regulator news items), and
up to three contiguous excerpts per document, were selected by a fixed hash.
The pool is an availability sample of institutional prose, not population-random
Icelandic or representative private legal/financial customer documents.

Final private packet: `/root/.local/share/dmz/datasets/web-testset-20260930`.

- 76 documents, seven publishers, 126 excerpts, 23,147 whitespace-delimited words.
- General prose: 39 excerpts; public administration: 27; financial: 36; legal: 24.
- Publishers: Háskóli Íslands, Akureyrarbær, Stjórnarráðið, Landsbankinn, Hagstofa,
  Persónuvernd and Umboðsmaður Alþingis. Legal material includes eight published
  Ombudsman case summaries; it is not a collection of private contracts.
- Every label remains null/unreviewed. The annotation packet includes numbered
  UTF-8 text files, a source/stratum catalog, reviewer instructions and hashes.
- Review JSONL SHA-256: `0963980264b37fa233f19c7529bf6d596ce22a4a8ef4f8530304e53c13a9e9b6`.

The raw first pass is preserved at `web-eval-20260930-v1`; extraction cleanup is
in `web-eval-20260930-v2`. The same eight preselected legal URLs were retried with
an explicit `section.case` extractor in `web-eval-legal-20260930-v1`. No models
were run between extraction revisions. The combined packet uses v2 plus that
legal collection. Earlier blind-review-v1/v2 packets remain unchanged.

Reykjavík's 12 article requests did not return HTML to this collector and were
excluded. One municipal category page and one short/unusable statistics page
were excluded. Seðlabanki and Arion indexes did not expose usable article links
to discovery; Samkeppniseftirlitið returned an HTTP error. No workaround for
access controls was used. These source gaps bias the available sample.

Extraction decodes HTML entities, collapses block whitespace and removes
publisher-inserted discretionary soft hyphens; other Unicode is preserved.
Offsets address the resulting frozen UTF-8 text. Spot checks caught navigation
and heading artifacts and improved selectors, but every excerpt still requires
human extraction verification and boundary annotation before evaluation.

Overlap screening used 32 prior JSONL files, including all prepared MIM splits,
synthetic/domain training and development, earlier unlabelled review text and
available parity/error records. Zero normalized 12-gram overlap was found in
retained excerpts or their documents, and no cross-collection document pair met
the >=50% 12-gram-containment exclusion threshold. Files without text fields are
recorded with zero contributing rows. Shorter shared phrases, semantic copies,
unknown external training and base-model pretraining exposure remain untested.
Newly collected does not mean all documents were newly published.

Keep raw pages, excerpts, annotations and individual predictions outside Git.
Only collection code, synthetic tests, source URLs and aggregates are committed.
Collection is not a redistribution licence. Source-use review remains pending
before distributing source text beyond this private evaluation work.

Next: human annotation and independent second review, adjudication and immutable
label hashes, then one evaluation of the already frozen reference at unchanged
thresholds. If this set informs tuning, retire it to development and reserve a
new document sample. Do not turn null labels into negatives or report scores now.
