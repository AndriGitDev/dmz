# Separate Person head with auxiliary entity supervision

Predeclared on 5 October 2026 before training or candidate evaluation. This is a
new, single-recipe research trial following the failed nine-label training,
exact-other-type veto and closed 36-configuration composition family. Those
results and decisions remain unchanged. Development has been reused repeatedly;
this adaptive hypothesis is not independent validation.

## Hypothesis and fixed recipe

The nine-way softmax directly competed Person logits against other entity types.
A separate Person head could retain useful name decisions while auxiliary entity
supervision shapes shared encoder features. This is a hypothesis, not an inferred
cause of the previous errors or an expectation of success.

Use exactly the original 46,196-row training mixture, SHA-256
`9f5020c0ea357b6f85fca5162aa6d01d6328669c189ae67ddf9e27731a22387a`.
Initialize the encoder and three-label Person head from frozen FP32 epoch two,
SHA-256 `ec2ecc9579816fe059b83cc76cd61a18c7a6aad74253565fa98d0654536d8e9a`.
Copy that head into a separate auxiliary head; expand only its final layer to
O/B/I-Person, B/I-Organization, B/I-Location, B/I-Miscellaneous. Copy the first
three rows and initialize the other six using the seeded PyTorch initializer.
Check exact primary-logit parity in evaluation mode before any optimizer step.

Run one complete epoch, seed 45, encoder LR 5e-6, both head LRs 5e-5, AdamW weight
decay .01, batch 8, four-step accumulation, combined gradient clipping 1.0,
10% warmup and linear decay. Use FP32, unchanged mean subword pooling and three
CPU threads. Optimize primary three-label cross-entropy plus **0.25 times**
nine-label auxiliary cross-entropy over the same nonpadding words. Do not change
weights, seed, epoch count or thresholds after inspecting results.

The primary target maps every non-Person entity label to O and retains ignored
padding. The two heads share one encoder forward pass. Save the ordinary
three-label checkpoint and the auxiliary head separately, with hashes and both
losses. The auxiliary head is not used during inference. Maximum training time
is 9,000 seconds; a partial epoch is ineligible. Run a two-optimizer-step,
training-only probe first and discard its weights. No blind/review pool is read.

## Evaluation and stopping

Evaluate the one complete candidate once on general development and original
synthetic-domain text. Retain neural-only and fixed CRF-priority composition
(CRF .75, neural .0) reports. Use the same metric criterion as previous trials:
maintain both strata's frozen precision and exact excerpt accuracy, and strictly
improve the lower stratum recall. Also report the separate ≥99% accuracy and
recall point targets on each stratum. Do not search thresholds, priorities,
checkpoint averages or auxiliary-loss weights in this trial.

Validate source, initialization, complete epoch, recipe, checkpoint and evaluation
identities before comparison. An incomplete run or failed criterion retains the
frozen reference. Any qualifying research result still needs exporter/runtime
parity, browser masking/restoration and unchanged per-type release gates before
model promotion can be considered. No automatic promotion is authorized by a
passing research result, and no result establishes independent all-entity quality.
