# AWS credential component probe

This 38-excerpt synthetic probe was fixed before changing recognition. It has
23 labelled credential components across 19 positive excerpts and 19 negatives.
The original native result is TP 7, FP 0, FN 16. The revised result is TP 21,
FP 1, FN 2. All strings are fabricated; no issuer or authentication calls occur.
These are selected development cases, not a field accuracy estimate.

## Recognition choices

AWS distinguishes long-term `AKIA` and temporary `ASIA` access-key IDs in its
[identifier documentation](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_identifiers.html).
The existing bare 20-character uppercase/digit key heuristic now accepts either
prefix. It does not generalize other IAM identifiers such as user, role, group,
instance-profile, policy, certificate or public-key IDs into credentials.
An access-key ID is a credential component, not a usable secret by itself;
DMZ conservatively masks it under the existing Secret policy.

The [STS API](https://docs.aws.amazon.com/STS/latest/APIReference/API_GetAccessKeyInfo.html)
accepts a wider 16–128-character word-character input envelope. That API envelope
is not proof that every accepted shape is an issued credential. Within explicit
`AccessKeyId`, `access_key_id` or `AWS_ACCESS_KEY_ID` fields, case-insensitively
with hyphens normalized to underscores, DMZ recognizes the complete value when
it starts with `AKIA`/`ASIA` and its remaining characters are ASCII letters,
digits or underscores. Such contextual findings are uncertain. Bare shapes keep
the narrower existing heuristic to limit unrelated uppercase identifier matches.

The [STS credential structure](https://docs.aws.amazon.com/STS/latest/APIReference/API_Credentials.html)
also has `SecretAccessKey` and `SessionToken`. Existing assignment handling already
recognizes the secret-key field; the generic field vocabulary now includes
`session_token` and its compact camel/Pascal spellings. Its unchanged whole-value,
length, mixed-character and entropy checks still apply, as do explicit programming
fence/literal rules and public-field exclusions. No fixed session-token maximum
is introduced. No changes to name models, thresholds or outbound policy occur.

## Remaining errors and scope

The two labelled misses remain in the probe: a shorter API-shaped key written in
prose without a recognized field, and a session token described in prose rather
than an assignment. The one false positive is an unfenced `SessionToken` property
reference under the probe's intended source-code interpretation. The same
reference in an explicitly labelled program fence remains negative. Plain text
cannot always distinguish a reference from an opaque literal, so this limitation
is retained rather than removed from scoring.

Tests cover complete multi-component JSON and environment records, 16/19/20/21/128
contextual lengths, long session tokens, invalid lengths/alphabet, non-key IAM
prefixes, original UTF-8 offsets, embedded identifiers, public fields and code
references. Strict processing blocks 21 labelled components and one benign
reference, and leaves the two misses exposed. dmz best-effort masking and exact
restoration are measured separately through the packaged browser.

This is not comprehensive AWS credential support. Other credential families,
XML fields, URL-encoded values and unrepresented contexts need separate evidence.
The API documentation supplies context and bounds, not a detector quality claim.

The packaged [before](results/quality-99-v1/aws-before.json) and
[after](results/quality-99-v1/aws-after.json) runs improve both paths from 23/38
to 35/38 exact excerpts and from 7/23 to 21/23 completely masked components.
All 38 outputs restore exactly. The two misses and one false positive remain;
all thirteen earlier corpora retain their complete typed-quality, masking and
restoration results. The runs record zero external requests and artifact/runtime
hashes. The after package is a dirty development build, not a release archive.
