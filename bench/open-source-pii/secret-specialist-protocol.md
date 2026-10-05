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

The 102-excerpt vendor-token probe is now calibration data: Gitleaks finds all
72 positive spans but also flags 11/30 negatives; the existing core finds 16/72
and has no false positives. These repeated fabricated values cover 18 token
families, not 72 independent secrets or a field distribution.

Next bounded adapter, fixed before implementation: extract the 17 context-free
rules covering those families (Stripe's one rule covers two fixture families).
Keep their original token bodies, entropy thresholds and applicable global
secret allowlists. Remove only the trailing source-code delimiter expression;
DMZ will check whole-token Unicode boundaries and reject continuations through
identifier characters, hyphens, plus/slash/equal signs or dotted identifiers.
This permits ordinary English sentence punctuation without accepting partial
tokens embedded in longer identifiers. No generic keyword rule, client-ID rule,
path filter, archive/encoding handling, source-code suppression comment, network
verification or upstream runtime is included.

Preserve existing non-secret detections on overlap. Existing broader Secret
matches keep their normal precedence. Runtime uses generated public rule data
and standard Go regexp/entropy code, with upstream MIT notice included. Verify
native/WASM spans, exact restoration, the existing synthetic recall gates and
a new seeded specialist follow-up after the rule/boundary choice is fixed.
Do not retune thresholds or add rules based on that follow-up and call it blind.

Post-evaluation correctness fix, 5 October: code review prompted a separate
fabricated PyPI-token probe containing phone/card-shaped substrings. The original
overlap rule masked only those fragments and exposed the token remainder. A
complete validated vendor token now outranks strictly contained findings,
including substring tenant terms. Surrounding/equal-span entities and explicit
review signals retain precedence. This is a derived regression fix, not a new
blind accuracy result; keep the earlier calibration/follow-up unchanged and
repeat their checks plus native/WASM/browser and core replay guards. No rule,
entropy threshold or label was tuned from the follow-up.
