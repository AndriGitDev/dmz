# Multi entity annotation rubric

Version 1 is a proposed evaluation convention for human review, not an assertion
that Shield already detects every form below. Freeze this rubric, entity scope,
sample membership and dictionary before predictions. Resolve policy ambiguities
before accepting final labels; do not change gold labels to make a detector pass.

The target metrics are exact entity-set accuracy per excerpt and exact entity
recall, each at least 99%. An excerpt is correct only when all entity spans and
types agree and no extra entity is predicted. Public information and fabricated
examples can still contain target identifiers; this is a detection convention,
not a legal determination of whether a particular value is personal data.

## Independent review

Give reviewer A and reviewer B separate copies of the unchanged text, this rubric,
source catalog and configured tenant terms. Do not show them model predictions or
each other's labels. Use stable reviewer codes rather than personal email addresses.
Both reviewers must read every excerpt, including apparently empty/negative ones,
and compare extraction with the recorded source. Reading order, tables, clipped
words and excerpt boundaries need attention. A spot-check does not complete review.

Each row starts with `entities: null`. It becomes `[]` only after the reviewer has
checked the entire excerpt for every listed type and found none. Record
`reviewed: true`, `extraction_verified: true` and the reviewer code. Mark unusable
extraction with a concrete `exclusion_reason`; excluded rows retain null entities.
Exclusions must not be a way to discard difficult but usable detection cases.

A third reviewer adjudicates every excerpt after both independent submissions,
including agreements. Record a distinct adjudicator code, an explicit decision
note, final entities or an exclusion, and any prior-overlap resolution. Record
uncertainties in reviewer `issues`; resolve them in the adjudication note. A name
in an identity field is not proof that review was performed by a distinct human.
The tooling checks structure and identities as recorded, not actual independence.

## Span boundaries

Use zero-based UTF-8 byte starts and exclusive ends in the exact supplied text.
For example, `Ása` occupies bytes 0 through 3, so its span is start 0, end 4.
Do not normalize accents, alter whitespace, fix spelling or reflow text. Internal
spaces, tabs, line breaks and punctuation belong inside a span when they are part
of the identifier. Exclude surrounding labels, quotes and sentence punctuation.
Preserve punctuation that is part of a name initial or identifier itself.

Annotate every occurrence. Repeated values have separate offsets. Do not annotate
an incomplete boundary-clipped mention as if its missing characters were present;
flag the clipping and adjudicate whether the visible form is a complete identifier.
Different surface forms are separate spans even if they refer to the same person.

Use one semantic type for each interval; do not nest annotations. A name-like email
local part is part of the Email, not a separate Person. A person-derived street name
inside a postal address is part of Address. A connection string containing an
email-like username is a single Secret. Explicit tenant terms are annotated outside
enclosing structured entities; a term exactly equal to a standalone name uses
TenantTerm. For other ambiguous overlaps, adjudicate and record the reasoning.

## Entity types

| Type | Include | Distinguish from |
| --- | --- | --- |
| Person | Full or single names, initials, inflected and foreign names, and name mentions across layout breaks when continuous in the source | Titles/job descriptions alone, organizations and geographic names |
| Address | Street/delivery address with house number and unit; attached postcode and locality when supplied; rural property address when context establishes a delivery/contact address | A town, country, street or property mentioned only as a place rather than an address |
| Kennitala | A ten-digit Icelandic person/entity identifier in context, including formatting separators and plausible date-shaped forms without a legacy check-digit match | Dates, prices, version numbers, unrelated ten-digit codes |
| Email | Entire local part, at sign and domain; preserve case and internationalized spelling | Invalid domain punctuation or an isolated name-like local part |
| Phone | Icelandic seven-digit contact number with any supplied country/dialling prefix and formatting | Amounts, measurements, unrelated numeric IDs and dates |
| IBAN | Entire Icelandic IBAN as presented, preserving grouped formatting | Generic numeric strings without account context |
| Card | Complete payment-card number, including internal formatting; explicit card context matters for transcription errors | Checksums, invoice numbers and arbitrary long digit strings |
| Secret | API/token credentials, bearer token value, JWT, full private-key block, secret-bearing connection string; unfamiliar tokens when surrounding context identifies a credential | Public checksums, ordinary source-code identifiers, placeholder prose without a credential value |
| TenantTerm | Exact, case-sensitive configured term from the packet's dictionary | Case variants and similar words that were not configured |

Person follows the broad MIM-compatible named-person convention used by the
existing development reference, including named fictional beings and animals.
Keep that scope visible when interpreting results; a broader named-entity score
must not be described as an estimate confined to real customers' private names.

Address deliberately includes contextual rural/unnumbered addresses beyond the
current gazetteer-plus-number implementation. Phone includes clear contact numbers
even when their typography is currently unsupported. Annotators must not use the
current regexes as a definition of what counts. Organization and generic Location
are auxiliary model-training classes, not additional privacy entity types here.

For explicitly presented account/card identifiers, flag a suspected typo or checksum
failure for adjudication rather than silently declaring it negative. For standalone
numbers without that context, do not infer sensitivity from length alone. Distinguish
the existing synthetic benchmark's intentionally invalid negatives from naturally
occurring text under this rubric; never pool incompatible annotation policies.

Unicode formatting/combining characters are not a gold entity type. A benign word
with decomposed accents remains negative. A detector may conservatively mask the
whole excerpt, but that is measured separately as masking burden, not exact entity
detection. Do not include real credentials or customer values in bug reports or Git.

## Freeze and evaluation

`entity_review.py compare --packet PATH` validates the two completed independent
submissions and reports aggregate agreement. It does not copy an agreement into
gold automatically. Complete `adjudication.jsonl`, then run
`entity_review.py freeze --packet PATH --output NEW_PRIVATE_DIRECTORY` to freeze
gold text/label hashes and review evidence. Both commands refuse incomplete review.

A structurally valid frozen packet still needs source-use review, verified human
independence and annotation correctness, and a predeclared representative sample
with document/related-article separation. Its default `eligible_for_final` remains
false. Approving final evaluation is a separate evidence decision. The existing
88-document packet and any reserved review pool must not be used for model-assisted
labels, training or threshold selection and later called independent validation.
