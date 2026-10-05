# Specialist secret-detector comparison

Evaluate Gitleaks at source revision
`b58d3f102cf3a2c84cb7f923d05c25c9b1aed84b`, using its checked-in default
configuration. Its MIT source and configuration stay in the private research
directory; no dependency is added to the DMZ core. This is a specialist comparator,
not an English Person/address recognizer.

Use stdin with JSON reports captured transiently, no repository scan, no verbose
logging, no secret verification and no diagnostics server. Disable recursive
decoding and archive traversal: their findings do not directly establish the
original-byte secret span contract. Ignore `gitleaks:allow` comments so prompt
content cannot suppress findings. Keep upstream entropy thresholds and rule
allowlists unchanged; do not tune rules on these diagnostic results.

Resolve each reported secret inside its reported original match and location.
Verify exact source bytes and uniqueness within that match; any ambiguous or
invalid location makes the whole candidate response abstain. Never search for
the secret globally and mask unrelated occurrences. Persist only hashes, typed
byte spans, rule identifiers and aggregate counters, never raw tool reports.

First compare the already frozen 100- and 90-excerpt synthetic sets. They are
diagnostic/model-selection evidence for this new candidate, not newly blind
validation. Report Secret-only standalone quality and full-scope additive
composition, retaining all existing DMZ matches on overlaps. If useful, design a
separate specialist probe before changing product rules.
