# GitHub token regression cases

The previous secret rule covered only the `ghp_` prefix with exactly 36
alphanumeric body characters. It missed the other documented GitHub prefixes
and longer values. The detector now recognises `ghp_`, `gho_`, `ghu_`, `ghs_`,
`ghr_` and `github_pat_`, retaining the complete token-shaped value.

GitHub documents these prefixes in its [authentication reference](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github).
Its [2 October 2026 changelog](https://github.blog/changelog/2026-10-02-stateless-github-app-installation-tokens-rolled-out/)
announces completion of the stateless installation-token rollout: newly minted
values use `ghs_APPID_JWT`, typically about 520 characters, and integrations must
treat them as opaque strings. The earlier [token-format announcement](https://github.blog/changelog/2021-03-04-authentication-token-format-updates/)
also describes underscore-containing token alphabets and warns against relying
on a fixed length. These sources were checked on 5 October 2026.

## Detection boundary

This is local **shape recognition**, not issuer validation. The prefix is
case-sensitive. The body must have at least 36 ASCII letters, digits, underscores,
hyphens or internal periods; its first and last characters cannot be periods.
The minimum is a DMZ heuristic retained from the earlier token-body length, not
a provider guarantee about every token type. There is no fixed maximum length
in this rule. Dots preserve the full stateless JWT-shaped body, without decoding
or checking its claims or signature.

Whole-word boundaries reject a prefix embedded in a longer word and a value
continued by a Unicode letter. Whitespace and quotes delimit values. Final prose
periods remain outside the span. Short examples that only name a prefix are
negative cases. A long noncredential string with a supported prefix and alphabet
can still be a false positive; undocumented alphabets, encoded forms and future
shorter credentials may be missed. These limitations prevent treating this rule
as complete credential discovery or validity checking.

The older native assertion that a 37-character `ghp_` body must be negative is
removed deliberately: masking a longer provider-prefixed shape is now supported.
Tests preserve the 35-character negative boundary and verify complete 36-, 37-
and longer-body values. Strict session processing blocks recognised secrets;
best-effort processing masks the complete span and restores the original bytes.
No value is sent to an issuer or used to authenticate.

## Evidence

`corpus/github-token-v3.jsonl` contains 32 fabricated excerpts: 15 token spans
and 17 negatives. They cover every listed prefix, longer values, internal
separators, stateless installation shapes exceeding 255 characters, prefix-only
documentation, short values, embedded words and line breaks. Native and WASM
tests additionally exercise a value exceeding 1,000 characters and exact UTF-8
offsets after an emoji. All values are deliberately nonfunctional synthetic
strings; no real credentials are present.

The core snapshot detects all 15 spans without extra detections; strict exposure
checks block the 15 positive prompts and preserve all 17 negatives. Run
`scripts/check-github-token.sh` to verify the pinned corpus and snapshots.
Packaged before/after results are measured separately. These selected development
cases do not establish independent field accuracy or recall. Existing corpus
labels, model thresholds and the frozen browser model are unchanged.

The packaged replay improves both basic and trained paths from 18/32 to 32/32
exact excerpts and from 1/15 to 15/15 completely masked identifiers, with no
extra typed detections. All 32 outputs restore exactly before and after. Every
typed-quality, masking and restoration metric in the eight earlier corpora is
unchanged. Both runs report zero external requests. The aggregate
[before](results/quality-99-v1/github-token-before.json) and
[after](results/quality-99-v1/github-token-after.json) reports retain package,
measurement and corpus hashes; the after artifact is the recorded dirty build
of this change, not a clean release archive.
