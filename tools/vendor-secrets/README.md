# Portable vendor-secret rules

`source.json` is a selected snapshot of the MIT-licensed Gitleaks configuration
at `b58d3f102cf3a2c84cb7f923d05c25c9b1aed84b`. The complete source configuration
SHA-256 is `e163e53b9e7e8a8511e77271e2b323ed057759542a6d988258afe3a1fa329caf`.
This snapshot contains public detection rules, no issued credentials.

Run `python3 tools/vendor-secrets/generate.py --check` to verify the embedded
pack, or omit `--check` to regenerate it. The one-time `--extract PATH` operation
accepts only the pinned upstream configuration and refuses an existing snapshot.
There is no automatic update or network fetch in builds or detection.

The subset has 17 rules covering DigitalOcean, GitLab, Hugging Face, Linear,
Notion, npm, PyPI, SendGrid, Shopify, Slack, Stripe and Twilio token families.
It preserves token-body regexes, entropy thresholds, keyword prefilters and
applicable global secret allowlists. The original trailing code delimiter is
replaced with DMZ's whole-token boundary checks so sentence punctuation is
supported. Original non-secret matches retain precedence except findings
strictly contained inside a complete validated vendor token: masking
only a phone/card/IBAN/kennitala-shaped fragment would expose the remainder of
that credential. The same applies to a configured term that matches only a
substring of the token. Surrounding or equal-span entities, including an
explicitly configured whole-token tenant term, retain precedence. Explicit
review signals are never displaced. The runtime does not
import Gitleaks or any new Go module dependency.

The PyPI rule's `{50,1000}` repetition overflows the TinyGo WASM regexp
compiler stack. Its runtime adapter uses the same ASCII character class with
`+` and explicit 50–1000 body-byte bounds. Together with the whole-token
boundary check, this preserves the accepted token language. Native and WASM
checks cover 49, 50, 1000 and 1001 bytes; the generated source pattern remains
unchanged for provenance.

The rules-only WASM build also exposed stack limits when compiling the global
uniform-letter allowlist and path allowlists. Uniform ASCII letter/star/period
runs are checked directly (including case folding); exhaustive native parity
covers all 16,384 two-byte ASCII strings. The two exact pinned path patterns
have a literal `/` prefilter and compile only if that prefix is present. Selected
vendor tokens start with a letter, so those inapplicable patterns never compile
in detection. Other patterns retain their original semantics. Caches contain
public rules only, never submitted values.

Those reductions alone did not make the rules-only callback stack reliable:
the first accepted PyPI fixture still hung with TinyGo 0.42.0's 64 KiB WASM
default. A 256 KiB stack first restored the full smoke suite. The final build
sets `-stack-size=128kb`: both build profiles pass cold smoke, and the smaller
candidate preserves every checked native/WASM phone case, specialist fixture,
English replay and Icelandic Person replay, plus browser workflows. It also
reduces observed latency spikes relative to the 256 KiB trial on the contended
test host; these measurements are not field latency guarantees. See the
[runtime record](../../bench/open-source-pii/structured-runtime.md).
TinyGo uses fixed-size goroutine stacks
when it cannot infer a size; see its [datatype/runtime documentation](https://tinygo.org/docs/concepts/compiler-internals/datatypes/).
Keep tests for both rules-only and trained builds, since success in one binary
layout did not establish safety in the other.

This is a limited adapter, not behavioral equivalence with all of Gitleaks.
It excludes generic/contextual rules, client IDs, repository/path policy,
suppression comments, decoding, archives and verification. Low-entropy tokens,
unusual token revisions and tokens embedded in paths/encoded strings may be
missed. A recognizable shape is not proof of an active credential. Keep fresh
quality, false-positive and native/WASM evidence alongside changes.

The upstream MIT licence is distributed in
`apps/dmz/public/licenses/gitleaks-MIT.txt`; the accompanying notice describes
DMZ's changes.
