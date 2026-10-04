# Database URI regression corpus

`corpus/database-url-v3.jsonl` contains 25 synthetic excerpts: 14 credential-bearing database URIs, ten non-secret connection references, and one email-valued query parameter. Values are fabricated and hosts use reserved example domains or an IPv6 documentation address. These cases were written during implementation and are not independent validation.

The old detector marked `postgres://` and `mysql://` references as secrets solely because of their schemes. The new rule requires a nonempty userinfo password or a recognized nonempty credential query parameter. It accepts `postgres://`, `postgresql://` and `mysql://` case-insensitively, preserves the full detected URI and original bytes, and leaves a simple username@host authority out of email detection. Actual email-shaped query values and more complex usernames remain eligible for email detection.

PostgreSQL documents both URI spellings, optional credentials, percent-encoded values and multi-host syntax in its [connection URI reference](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-URIS). It also defines `sslpassword` as a private-key password. The detector recognizes credential-shaped text; it does not connect to a database or validate connection parameters. A malformed percent escape does not make a visible userinfo password safe to expose.

Recognized query names are `password`, `sslpassword`, `passwd`, `pwd`, `access_token`, `token` and `api_key`, including percent-encoded key names. Matching does not imply that every database driver accepts every parameter. Empty credential values and unrelated parameter names are not secret signals.

Quoted and whitespace-delimited boundaries are covered. Query-value punctuation is preserved because it can be credential material; ambiguous unquoted prose can still be overmasked. General keyword/value connection strings and other database schemes are outside this change. Other entity detectors and configured tenant terms can still mask content in a non-secret URI.

## Packaged results

The aggregate reports in `results/quality-99-v1/database-url-before.json` and `database-url-after.json` use the same frozen name model. Both basic and trained paths improve from 8/25 to 25/25 exact excerpts, from 4 to 15 true positives, from 14 to zero false positives, and from 11 to zero false negatives. Complete masking rises from 4/15 to 15/15 annotated identifiers. All 25 excerpts restore exactly; all requests remain local.

The after report also replays the six earlier synthetic corpora, whose counts remain unchanged. These selected development examples cannot establish representative accuracy or the project's independent 99% target. Native and WASM tests additionally protect credential punctuation inside a surrounding parenthesis, which was caught during boundary review.

`GO_BIN=/path/to/go scripts/check-database-url.sh` verifies the corpus hash, exact core report and strict-policy exposure snapshot in CI. Existing corpus pins and per-type gates are unchanged.
