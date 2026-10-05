# Structured detector runtime validation

The portable phone/vendor integration runs in the existing local Go/TinyGo core.
The first direct phone-library attempt failed during protobuf initialization in
TinyGo, so only public numbering metadata is imported at build time. The
original PyPI bounded repetition also exceeded the WASM regex compiler stack;
an equivalent ASCII run plus explicit byte bounds preserves the pinned token
language without expanding the large repetition.

The rules-only build subsequently hung with TinyGo 0.42.0's default 64 KiB
callback stack, even after equivalent allowlist rewrites. A 256 KiB build passed
the full smoke suite. A smaller 128 KiB candidate was then tested against the
same product source and selected after all declared runtime guards passed:

- Rules-only and trained cold smoke, including PyPI lengths and the trained
  local address lexicon.
- All 49,245 geographic/upstream-example and seeded-random phone-rule cases:
  native/WASM typed spans agree and restoration is exact. Nine global-network
  examples are checked separately in both 128 KiB profiles. These compare
  numbering rules; they are not independent PII accuracy data.
- All 236 vendor-token and 36 phone specialist fixtures agree with native
  results and restore exactly in the trained build.
- All 4,996 public English cached-name replay records are identical between
  128 and 256 KiB, including all typed spans, abstentions and restoration.
- All 5,838 reused Icelandic development Person replay records are identical.
  This does not measure full-entity quality or rerun the name model.
- The trained private extension copy passes settings, send interception,
  restoration, network-body, composer-race and broken-core browser checks.

The source trees for `packages/detect`, `packages/surrogates` and
`apps/shield/wasm` are identical across the two stack candidates. The final
build uses `-stack-size=128kb`; imported rules and model thresholds did not
change during this runtime comparison.

## Timing limits

After stack selection, a separate overlap regression was fixed: a validated
vendor token must cover phone/card-like substrings inside its body. Final
generic and trained builds were rechecked after that product change, including
the complete phone audit, all 272 specialist fixtures, cached-name replays and
browser send/restoration workflows. The immutable earlier score reports are
linked to the final binaries by exact record equality in
[`final-runtime-validation.json`](../results/open-source-pii-v1/final-runtime-validation.json).
That report records the final product source and both binary hashes; the
earlier stack-comparison artifacts remain historical measurements.

Measurements ran on a contended CPU host alongside model evaluation; they are
diagnostics, not device or field latency guarantees. A short synthetic
phone/email input was measured 100 times after 20 warmups, timing each callback
separately. Median masking was about 1 ms for both candidates; its observed p95
was about 156 ms at 256 KiB and 63 ms at 128 KiB. The full phone audit's masking
p95 was about 172 ms and 87 ms respectively. These observations motivated the
smaller stack but do not identify a GC mechanism or prove universal latency.

Both earlier failures and successful candidates are retained privately. The
aggregate runtime record keeps source/artifact hashes and counts without input
text or prediction records. No model weights or remote verification service are
added to Shield by these integrations.
