# Small address-parser feasibility trial

`usaddress==0.5.16` is a MIT-licensed US address-component parser, not a general
PII detector or an address validator. Its bundled CRFsuite model is about 131 KiB.
Evaluate it locally as a potentially lighter component; do not call its parsing
accuracy free-text detection accuracy or infer that English implies US locale.
The isolated install pins `python-crfsuite==0.9.12` and `probableparsing==0.0.1`.

Before predictions, fix two diagnostic paths on the already reused initial
100-excerpt set:

1. Filter the preselected GLiNER2 address candidates: require parsed
   `AddressNumber`, `StreetName` and `StreetNamePostType`, a digit-leading house
   number and a post-type present in the parser's published suffix vocabulary.
   Require minimum marginal probability 0.90 across those required components.
   Preserve the original GLiNER2 span and score; parsing never expands it.
2. Parse a bounded whole excerpt and extract a contiguous numbered-street span
   with those same required labels and posterior floor. Optional directional,
   number-suffix and occupancy components may extend the street span. Extend a
   place/state/ZIP tail only when it includes an explicit US state abbreviation
   and a 5- or 9-digit ZIP. This is an experimental grammar, not postal validation.

Keep original text and offsets. Any parser/token alignment failure abstains for
the whole candidate. No HTML entity rewriting, fuzzy substring search, clipping,
network call or runtime training. Keep raw parser predictions transient. Both
variants preserve all existing DMZ spans on overlaps. The 100-excerpt results
are calibration evidence; any selected new integration requires a newly frozen
follow-up, not reuse of the already-inspected 90 excerpts as blind validation.
