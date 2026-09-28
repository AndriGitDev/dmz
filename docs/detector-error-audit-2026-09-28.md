# Development error audit and domain training — 28 September 2026

## Scope

The owner approved an extended detector improvement session, prioritizing legal
and financial documents and both exact Person precision and recall >=95.5%.
This round uses development data only. It does not change Shield, publish a new
website score, or establish independent final accuracy.

## What the baseline misses

At its previously selected threshold 0.65, the baseline CRF has 1,266 TP, 53 FP,
and 240 FN on 5,582 MIM development sentences.

| Exclusive missed-name reason | Count |
| --- | ---: |
| No decoded name prediction | 164 |
| Exact candidate removed by confidence threshold | 54 |
| Wrong boundary or merged name | 22 |

Among false positives, 30 exactly match an annotation of another entity type
(9 organizations, 7 locations, 14 miscellaneous), 15 overlap a true person span,
1 overlaps another entity, and 7 are not annotated as any entity.

Name characteristics overlap: 169 misses are single-token names; 120 contain a
component unseen as a person in training; 102 contain an entirely unseen word.
Only 3/77 oblique patronymic-shaped gold names are missed. The immediate evidence
therefore favors context, unfamiliar words and short names, rather than a blanket
claim that Icelandic inflection is the dominant problem. These are automated
coverage/shape categories, not human-adjudicated linguistic diagnoses.

## New synthetic document supplement

`bench/corpus/domain/legal-financial-v2/` contains:

- 864 training documents, including 192 without person names; 1,728 Person spans.
- 256 development documents, including 64 without person names; 480 Person spans.
- Separate template families and name pools across train/dev; exact text and
  complete Person surface forms do not overlap between those two synthetic sets.
- Four positive forms: full names, single names, uppercase names, and initials.
- Contracts, powers of attorney, claims, invoicing, lending, payroll, auditing,
  estates and correspondence. Some English examples are included.

Every document and name is fabricated. There are 24 template families, not 1,120
independent real documents. Both splits share an author/generator; common given
names can also occur in the original MIM training corpus. Human review is pending
in `REVIEW.md`. This is a diagnostic development supplement, not a final legal or
financial performance benchmark. No claims of representative sampling are made.

## Controlled CRF comparisons

One copy of synthetic training was appended to the original 45,332 MIM training
sentences, making 46,196 records. Development files were never included. All
comparisons use 100 maximum iterations, c1=c2=0.1, with the original feature set
unless noted. Competing labels distinguish Person, Organization, Location and
Miscellaneous; scoring still counts exact Person spans only.

| Candidate / threshold | MIM dev precision | MIM dev recall | Synthetic domain dev precision | Synthetic domain dev recall |
| --- | ---: | ---: | ---: | ---: |
| Prior baseline / 0.65 | 95.98% | 84.06% | 86.36% | 51.46% |
| Competing labels / 0.70 | 95.91% | 85.66% | 69.14% | 48.54% |
| Document augmentation / 0.65 | 95.68% | 83.86% | 99.75% | 83.33% |
| Both / 0.70 | 96.27% | 85.59% | 100.00% | 82.29% |
| Added initials/case features / 0.65 | 95.55% | 84.13% | 100.00% | 90.21% |

The fixed selection rule requires precision >=95.5% on both development datasets
at the same threshold, then maximizes the lower recall. Of the original four candidates,
the augmented Person-only CRF wins. Its domain counts are TP 400 / FP 1 / FN 80;
the single false positive is a boundary error. Exact-span errors can count as both
one miss and one false detection. None reaches the dual target.

The domain audit breaks down the 400 correct names: 117/120 full forms, 94/120
single names, 102/120 uppercase forms, and 87/120 initial forms. Based on this
exposed development finding, a further feature-v3 experiment adds explicit initial,
length and case features. It is recorded as an added experiment in the protocol.

That added model becomes the best CRF under the same selection rule. MIM counts
are TP 1,267 / FP 59 / FN 239; domain counts are TP 433 / FP 0 / FN 47.
It finds 116/120 full forms, 95/120 single names, 110/120 uppercase forms and
112/120 initial forms. None of the 64 synthetic negative documents triggers a
name finding. Zero false positives on this small, correlated development corpus
does not establish perfect precision in real documents. General precision only
narrowly clears the target, and general recall remains far below it.

The original-text tokenizer check reproduces TP 400 / FP 1 / FN 80 on the
synthetic raw text. This verifies byte-offset handling for these generated forms;
the canonical text was generated using the same tokenization pattern, so it is
not independent evidence of production tokenization quality. The added feature-v3
model also reproduces its canonical counts on the original-text fields.

## Frozen encoder comparison

A separate local IceBERT experiment is defined in
[the protocol](../bench/contextual/experiment-v2.md). It uses a pinned masked
language model, not an existing NER-finetuned checkpoint. Only a small classifier
is trained. Pretraining overlap with MIM source text is not ruled out. Model
weights, original corpus, cached embeddings, and per-example error text stay
outside Git. The encoder is a research baseline, not an extension asset.

## Reproduction and gates

Aggregates are in [error-audit-v2](../bench/results/error-audit-v2/). Scripts and
commands are documented in [the contextual README](../bench/contextual/README.md).
Selection validates split, input hashes, model identity, annotation counts, common
threshold grids, and recomputed precision/recall. Both metrics must independently
meet the target; no accuracy/F1 substitution or pooling of datasets is permitted.

New tests cover exclusive error reasons, typed-label projection, exact original
UTF-8 offsets, reproducible corpus generation and disjointness, candidate selection,
word-boundary encoder chunking, and private cache/checkpoint integrity. Existing
Go, dataset and Shield browser CI gates remain unchanged.
