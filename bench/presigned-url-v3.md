# SigV4 presigned URL probe

This fixed synthetic development probe has 40 excerpts and 30 labelled entities:
29 complete URLs and one standalone credential component in an unsigned URL.
It was generated and scored before recognition changed. Native results improve
from TP 1, FP 3, FN 29 to TP 30, FP 0, FN 0. These selected cases are not a
representative accuracy estimate. All values are fabricated; no URL is visited.

AWS describes [presigned URLs as bearer credentials](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html).
Masking only an embedded access-key ID leaves the rest of that credential visible.
The [AWS Go SDK signer](https://github.com/aws/aws-sdk-go-v2/blob/main/aws/signer/v4/v4.go)
provides primary implementation evidence for the query authentication shape.
DMZ now masks the entire HTTP(S) URL when it contains exactly one of each:

- `X-Amz-Algorithm=AWS4-HMAC-SHA256`;
- `X-Amz-Credential` containing nonempty key, eight-digit date, nonempty region,
  nonempty service and `aws4_request` scope components;
- `X-Amz-Signature` containing 64 hexadecimal characters.

Parameter names and values are percent-decoded only for recognition. Original
bytes, parameter order, path, query, fragment and encoding remain within the
masked span and restore exactly. Hostnames and key prefixes are not allowlisted,
so compatible endpoints can be recognized. This is credential-shape detection,
not proof of authenticity: no issuer, signature, date, permission or expiration
check occurs. Date/expiration/signed-header query fields are not prerequisites
for conservative masking once the three authentication fields are present.

Quotes, markup and directly established brackets delimit the URL. A raw apostrophe
is retained unless a preceding single quote establishes a quoted value. Terminal
sentence punctuation is removed only if the original shape fails recognition and
the trimmed shape succeeds. Already recognized opaque query values keep their
punctuation. Plain prose remains ambiguous: punctuation following an unknown final
query value can be included, and nested wrappers are not a general Markdown parser.

The probe covers quotes and wrappers, Unicode offsets, query order and encoding,
HTTP/HTTPS, IPv6, arbitrary hosts, temporary keys and session parameters, expired
shapes, other services, embedded email/key findings, opaque punctuation, fragments,
two URLs, incomplete authentication and ordinary unsigned URLs. Tests additionally
exercise malformed encodings, duplicate authentication fields and boundary guards.
The whole Secret span takes precedence over interior detections.

This rule does not cover SigV2, CloudFront signed URLs, SigV4a ECDSA signatures,
HTML entity-encoded query separators, or arbitrary escaped/serialized URL forms.
Malformed or duplicate authentication fields can be missed. These are remaining
coverage limits, not negative claims about whether those formats carry secrets.

The packaged [before](results/quality-99-v1/presigned-before.json) and
[after](results/quality-99-v1/presigned-after.json) runs improve both paths from
12/40 to 40/40 exact excerpts and from 1/30 to 30/30 fully masked entities.
All 40 results restore exactly; external requests are zero. Every quality,
masking and restoration metric on the fourteen earlier corpora is unchanged.
The report records the development artifact, runtime and measurement hashes;
this dirty build is not a release archive. Native strict processing blocks all
30 labelled entities without blocking or changing any of the eleven negatives.
