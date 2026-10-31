# Stable resubmission of masked credentials

A synthetic coverage-guided fuzz run found a repeat-processing error. For
`AuthoriZAtion:BeArer 0 0`, the first scan correctly masks the first value and
leaves the following prose token alone. On a second scan, dmz replaced its
already-issued marker with spaces in an internal scan copy. The Bearer recognizer
skipped those spaces and incorrectly masked the second token as another value.
The minimized regression is retained under the Go fuzz test data directory.

Issued markers now become NUL-byte barriers in the internal scan copy. This
preserves byte offsets while preventing whitespace-based recognizers from
reaching through a protected value. These internal bytes do not enter output,
restoration maps or the host page. Unknown, edited and nested markers still use
the existing review/whole-text protection rules. Separate raw identifiers after
an issued marker remain detectable. Tenant terms reject NUL bytes, which are
reserved for internal scan barriers; invalid term updates preserve live mappings.

Native regressions cover Bearer, generic credential assignment and contextual
AWS access-key fields, repeated resubmission, and an additional raw email. The
new fuzzer checks valid UTF-8 inputs up to 16,000 bytes for exact restoration,
valid non-overlapping final typed spans, issued-marker idempotency and stable
same-session repetition. Its fabricated seeds include all recent credential
families, email forms, canonical Unicode variants and unknown markers.

The initial run found the regression in 3.21 seconds. After the correction,
the barrier-only run passed 173,152 executions over 181.066 seconds. The final
code including NUL-term validation passed a further 76,166 executions over
181.136 seconds with one worker and 67 new coverage-interesting inputs. These
counts include repeated/cache-derived inputs and are not latency comparisons. The [aggregate evidence](results/quality-99-v1/marker-roundtrip-fuzz.json)
records code, reproducer and log hashes. This is robustness evidence, not an
exhaustive proof, a detector-completeness test or an accuracy estimate. The
ordinary native and fixture suites and all current detector snapshot gates pass.

The final WASM checks pass for repeated credentials, separate raw identifiers and
rejected NUL-term updates. The [trained-browser record](results/quality-99-v1/scanner-marker-final.json)
passes all sixteen workflows, including actual chat resubmission and the real
120-second queued-worker deadline. External scanner requests are zero. All
packaged files match their pre-test hashes after deliberately corrupted-model
and stalled-worker tests restore their fixtures. Source and package hashes are
recorded; this is a development build, not a clean release archive.

The subsequent [fifteen-corpus replay](results/quality-99-v1/marker-after.json)
retains every first-pass typed-quality, masking and restoration metric from the
presigned-URL package on both paths, with zero external requests.

A separate [custom-term fuzz run](results/quality-99-v1/tenant-term-fuzz.json)
passes 812,250 executions over 180.087 seconds. It checks rejected-update mapping
preservation, accepted-update session reset, full coverage of an inserted accepted
term, exact restoration and stable resubmission. The count includes rejected
configurations and skipped invalid/oversized strings; it is not a count of unique
accepted terms or an accuracy estimate.
