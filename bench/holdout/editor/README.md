# Offline blind-review editor

`entity_review.py package` includes a self-contained `review.html` in each reviewer’s private archive. Open that file locally in Chromium or Brave. It embeds only the selected reviewer’s blank form, immutable excerpt texts, source index, frozen rubric and configured tenant terms. It contains no model predictions or other reviewer’s submission.

Enter the reviewer code agreed with the packet owner. Select each entity in the original text, choose its type, and add it. Highlights do not change the source text. The editor converts browser selections to UTF-8 byte spans and rejects character splits, overlaps and unconfigured TenantTerm labels. Follow the rubric for semantic decisions; these structural checks cannot decide whether a label is correct.

Mark an excerpt reviewed only after checking its extraction and every supported entity type. An explicitly reviewed excerpt with no entities exports `[]`. Unreviewed excerpts retain `null`. Exclusion requires a reason and retains `null`, so it cannot become a negative label. Completed excerpts must be reopened before edits.

Download progress before leaving the tab. There is no automatic browser storage. The progress file includes annotation state, reviewer slot and packet identity; it does not duplicate source text. Loading progress replaces the tab’s annotations only after all identity and span checks pass. A completed JSONL reviewer form can be exported only when every excerpt is reviewed or explicitly excluded. Return it privately to the packet owner for the existing comparison and third-person adjudication workflow.

The editor checks embedded content hashes, renders source text through text nodes, and uses a restrictive Content Security Policy with fixed script/style hashes and network access disabled. Source URLs are displayed as text rather than fetched. Ask the packet owner for the saved source material needed to verify extraction. The self-contained file contains private excerpts: keep the archive, HTML and downloads private. Content hashes establish internal consistency, not publisher authenticity, reviewer identity or independent acceptance.

The original packet files and frozen rubric are not changed by packaging or editing. Preparing new editor-enabled archives does not overwrite the earlier archives or label any excerpts.

Validation:

```sh
python -m unittest discover -s bench/holdout -p 'test_*.py'
node bench/holdout/editor/test-core.cjs
node bench/holdout/editor/test-browser.mjs
```

Install the existing holdout requirements for the Python suite and the Shield Playwright dependencies/browser for the browser test. All editor tests use fabricated excerpts, including hostile-looking markup, emoji, CRLF and decomposed accents. The browser test verifies exports with the Python submission validator and confirms that network attempts are blocked. Do not open protected real excerpts in automated/model-assisted browser verification.
