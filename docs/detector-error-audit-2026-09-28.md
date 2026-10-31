# Development error audit and domain training — 28 September 2026

## Scope

The owner approved an extended detector improvement session, prioritizing legal
and financial documents and both exact Person precision and recall >=95.5%.
This round uses development data only. It does not change dmz, publish a new
website score, or establish independent final accuracy.

**Final selected development result: MIM 95.92% precision / 95.29% recall;
synthetic documents 98.51% / 96.25%. The overall dual 95.5% target is not met.**

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

The frozen encoder processed 46,196 training records / 824,869 tokens in about
29 minutes on this CPU host. Mean-subword vectors are cached as float16, then
converted to float32 for the two classifier heads. The linear and 128-unit
nonlinear heads each trained for eight epochs; all 16 checkpoints were evaluated.
The head's reported evaluation time excludes the expensive encoder pass and must
not be presented as end-to-end latency.

Across the 21 standalone candidates, the fixed rule selects nonlinear head epoch
8 at threshold 0.65:

| Development set | TP | FP | FN | Precision | Recall |
| --- | ---: | ---: | ---: | ---: | ---: |
| MIM | 1,366 | 47 | 140 | 96.67% | 90.70% |
| Synthetic legal/financial | 427 | 19 | 53 | 95.74% | 88.96% |

This improves the balanced result over the CRFs but still misses the dual 95.5%
target. All checkpoints, including early ones with no predictions at high
thresholds, remain in the reports. A zero-prediction trial gets zero precision and
cannot qualify; it is not silently dropped. Existing command-line release-gate
behavior still rejects a report without predictions.

## Added combination experiment

The two model families recover different names. A follow-up fixes CRF priority
when spans overlap and adds only non-overlapping neural spans. Its 81 development
combinations reuse the same nine thresholds for each model. At CRF 0.75 / neural
0.65, the epoch-8 combination reaches:

| Development set | TP | FP | FN | Precision | Recall |
| --- | ---: | ---: | ---: | ---: | ---: |
| MIM | 1,429 | 65 | 77 | 95.65% | 94.89% |
| Synthetic legal/financial | 461 | 6 | 19 | 98.72% | 96.04% |

The MIM result is still ten correct names below the recall target. There are 35
misses with no proposed name, 22 below threshold, 17 boundary errors and 3 correct
neural predictions displaced by CRF overlap. Changing overlap handling alone
cannot close this gap. The document set clears both targets as a development
point estimate only. A longer 16-epoch head run is recorded as another added
development experiment; no annotations or test sets are changed.

## Final selection and remaining gap

The longer run reproduces the first eight head checkpoints byte-for-byte. Epochs
9–16 add eight standalone candidates and 648 CRF/head threshold combinations.
A further fixed equal-probability average of all eight late checkpoints adds 81
combinations; it does not beat the best single-checkpoint combination. All runs
and their order are recorded in the protocol. In total, 119 configurations at
nine thresholds produce **1,071 development trials**, not independent tests.
This amount of development selection makes fresh evaluation especially important.

The final best standalone head is epoch 16 at 0.70: MIM P95.56/R91.43 and
synthetic-domain P95.68/R92.29. The best overall composition uses feature-v3 CRF
at 0.75 and nonlinear head epoch 12 with no extra confidence cutoff (0.0).
BIO argmax still decides whether each word is a name; a zero cutoff does not turn
every word into a finding. The same threshold pair is used on both datasets.

| Final selected composition | TP | FP | FN | Precision | Recall |
| --- | ---: | ---: | ---: | ---: | ---: |
| MIM development | 1,435 | 61 | 71 | 95.92% | 95.29% |
| Synthetic legal/financial development | 462 | 7 | 18 | 98.51% | 96.25% |

Compared with the original CRF on the same MIM development set, missed names fall
from 240 to 71, while false detections rise from 53 to 61. General precision stays
above the target but recall needs four more exact names to reach it, without
losing the precision floor. Do not round 95.29% into a passing result.

The final composition reproduces its complete domain score grid on the original
text fields, including TP 462 / FP 7 / FN 18 at the selected threshold. Every
individual prediction cache used for combination was checked against its separate
standalone evaluator at all nine thresholds. This guards the combination code
against changing the underlying model's results.

The selected recipe and hashes are frozen in `selected-candidate.json`; this is
not a release manifest. It still requires the large local Python encoder, and no
Go/WASM integration, memory budget, browser latency or combined-entity exposure
claim follows from these results. dmz is unchanged.

Next evidence should come from human review of the synthetic annotations and a
new document-disjoint evaluation set, including genuine document structures and
negative examples. Further model work should investigate encoder fine-tuning and
remaining boundary/context errors. Continued threshold search on these same
development examples would not provide independent confirmation of quality.

## Reproduction and gates

Aggregates are in [error-audit-v2](../bench/results/error-audit-v2/). Scripts and
commands are documented in [the contextual README](../bench/contextual/README.md).
Selection validates split, input hashes, model identity, annotation counts, common
threshold grids, and recomputed precision/recall. Both metrics must independently
meet the target; no accuracy/F1 substitution or pooling of datasets is permitted.

New tests cover exclusive error reasons, typed-label projection, exact original
UTF-8 offsets, reproducible corpus generation and disjointness, candidate selection,
word-boundary encoder chunking, and private cache/checkpoint integrity. Existing
Go, dataset and dmz browser CI gates remain unchanged.
