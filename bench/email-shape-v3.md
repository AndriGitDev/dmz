# Complete email shapes

This 56-excerpt synthetic probe was fixed before changing email recognition.
It contains 39 labelled mailbox examples and 17 malformed/code/URI negatives.
The prior detector reports TP 18, FP 15, FN 21: several unsupported local-part
characters leave a suffix-only match, which is both a false positive and a miss
under exact-span scoring. The revised native detector reports TP 39, FP 0, FN 0.
These selected examples are development evidence, not independent accuracy or
a representative estimate of email coverage.

## Recognition boundary

The recognizer walks mailbox candidates around `@` and retains original UTF-8
byte spans. It accepts dot-separated atom local parts with ASCII email punctuation,
Unicode letters/digits/symbols, and nonempty single-line double-quoted local parts with
backslash escapes. Interior dots cannot be doubled and unquoted local parts
cannot begin or end with a dot. Complete quoting and escaping remain inside the
masked/restored value. Paired apostrophe, backtick and brace wrappers around an
otherwise unquoted mailbox are treated as surrounding prose; internal punctuation
remains part of the local part. This wrapper convention is a heuristic: text alone
can be ambiguous when a real local part starts with the same character.

Domain shapes require at least two nonempty labels, without underscores or
leading/trailing hyphens. The final label is at least two letters or has an
`xn--` prefix with a nonempty body. This does not validate IDNA conversion,
registry existence, DNS, delivery, or all protocol length limits. IPv4 and
`IPv6:` bracket literals are parsed locally with Go's `net/netip`; malformed
addresses and zone identifiers are rejected. Trailing sentence dots are excluded.

Known HTTP(S), FTP(S), SSH and database URL authorities with a bare username
are excluded. Mailboxes within query values, path segments, or more complex
usernames remain eligible. URI separators are interpreted only after an explicit
scheme and `://` in the same token; percent-encoded addresses are not decoded.
In prose or code without such context, valid-looking punctuation can be ambiguous.

The syntax references are [RFC 5322 sections 3.2.3–3.4.1](https://www.rfc-editor.org/rfc/rfc5322.html)
and [RFC 6531 section 3.3](https://www.rfc-editor.org/rfc/rfc6531.html).
The implementation is a prose detector, not a complete RFC mailbox validator.
Obsolete comments/folding, single-label domains, percent-encoded addresses and
unhandled internationalized forms are not established coverage. Existing Unicode
normalization/review policy remains in force, and non-ASCII mailbox detections
retain uncertain confidence. dmz masks them under its best-effort policy.

## Verification

Native tests exercise all local-part punctuation together, quoted/escaped local
parts, wrappers, original multibyte offsets, literals, invalid domains, malformed
quotes and URI contexts. Existing database URL tests caught an intermediate
query-mailbox regression; the final parser preserves those cases. The pinned
corpus retains full values as labels, so partial coverage cannot become success.
Packaged complete masking, exact restoration and prior-corpus comparisons are
reported separately after the WASM build.

## Explicit assignment follow-up

Review of the initial punctuation change identified an overextended local part:
`EMAIL=demo@example.invalid` absorbed the field assignment. A separate, fixed
17-excerpt [context probe](corpus/email-context-v3.jsonl) measures this before the
context correction. Its intermediate native result was TP 8, FP 7, FN 7; the
corrected result is TP 15, FP 0, FN 0. That baseline is the initial punctuation
implementation, not the branch's original detector.

At the existing line/JSON-field assignment boundary, the detector recognises
`email`, `email_address`, `emailAddress`, `mailbox`, and underscore-prefixed email
or email-address fields, case-insensitively with hyphens treated as underscores.
Keys are bounded to 64 bytes. Single/double-quoted literals in explicit
`const`/`let`/`var` declarations also establish a value boundary. Closing quotes
must match. The shared assignment matcher is unchanged for credential detection.
Other equals signs remain local-part punctuation, including a mailbox written as
`email=demo@example.invalid` within prose. This distinction is heuristic;
unsupported field names, escaped source-code representations and incomplete
program syntax are not established coverage. Both the original and follow-up
probes stay pinned and are scored separately.

The packaged [original baseline](results/quality-99-v1/email-before.json) and
[final thirteen-corpus replay](results/quality-99-v1/email-final-after.json)
improve both paths from 34/56 to 56/56 exact excerpts and from 18/39 to 39/39
fully masked mailboxes on the shape probe. The separate
[intermediate context baseline](results/quality-99-v1/email-context-before.json)
improves from 10/17 to 17/17 exact excerpts after assignment correction; all
15 mailboxes are fully masked. Every output on both probes restores exactly.
All eleven preceding corpora retain their full typed-quality, masking and
restoration results. The runs record zero external requests and package/runtime
hashes; these are dirty development builds, not release archives.
