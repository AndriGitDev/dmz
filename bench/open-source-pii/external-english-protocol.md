# Public synthetic English diagnostic

Source: `gretelai/gretel-pii-masking-en-v1`, revision
`e06eb1499ca8d54470f085021cd8e54f9efac7fd`, Apache-2.0 per its pinned
card. Download manifests verify upstream LFS hashes. All rows remain private.

This is a broader synthetic diagnostic across the source's domains, not DMZ's
independent acceptance set. Source annotation errors and unknown overlap with
model training/benchmark selection prevent an independent accuracy claim.

Conversion is fixed before predictions:

- Keep original text bytes. Map name/first_name/last_name to name_demo;
  address/street_address to address; email, phone_number, credit_card_number to
  email, phone, card; password/api_key to secret. Other source labels are outside
  this projection. Do not relabel SSNs or arbitrary account numbers as kennitala.
- Trim annotation edge whitespace, then use exact literal occurrences. All
  word-bounded occurrences of a source value are labelled (avoid first-name or
  number substrings embedded in longer identifiers). Do not fuzzy-match missing labels.
  Reject an excerpt if a mapped annotation is empty/missing, its mapped types
  conflict, or projected spans overlap incompatibly. Record rejection counts.
- Remove an English honorific from the start of a person annotation when followed
  by a name (Mr, Mrs, Ms, Miss, Dr, Prof, Sir, Dame; optional period). Join adjacent
  person spans separated only by horizontal spaces, because the source often
  annotates first and last names separately. Deduplicate identical spans.
- Use the source test split in full after deterministic conversion. Preserve
  source domains in aggregate reports. Candidate failures must abstain, not clip
  offsets or silently discard malformed findings. Time-limited runs stay partial.
- Retain source train and validation splits separately. If a compact-model
  adaptation is attempted, declare its selection/training budget before training;
  test and the fresh 90-excerpt follow-up are excluded from training/calibration.

Evaluate the current native core, original candidate configurations and the
already selected GLiNER2 address-only composition. Keep projected exact sets,
per-type recall/precision, false positives and runtime separate. Do not use this
public diagnostic to tune a variant and present the same run as validation.

Interpret “negative excerpts” as **no projected in-scope annotation**, not as
benign or free of PII. The source also contains dates, IPs, SSNs, usernames and
other identifiers outside the six-type projection. A post-run inspection of
12 deterministically selected zero-projected-gold excerpts confirmed this
distinction. GLiNER emitted 10 in-scope spans across five of them, including
employee IDs/usernames as persons and IPs as phone/secret. The inspection did
not change labels or thresholds and is not independent annotation validation.
Retain typed errors even when the underlying text may be sensitive in another
category; broader masking coverage and correct entity classification are
different measurements.

Runtime exception, recorded after a short full-test Privacy Filter prefix:
CPU inference averaged roughly 16 seconds per excerpt, projecting well beyond
the requested eight-hour work slice. Preserve the stopped prefix as incomplete.
Freeze a 512-excerpt sample by SHA-256 ordering of a fixed seed and source IDs,
excluding all prefix IDs already inferred. Selection never examines outcomes.
Evaluate Privacy Filter on that sample and rescore every comparator on exactly
the same IDs. Keep this sample report separate from the complete 4,996-excerpt
reports; do not present the stopped prefix as a full evaluation. The source's
synthetic-data and unknown-training-overlap limitations still apply.
