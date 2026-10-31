# Exact other-type veto experiment

Predeclared on 5 October 2026 before collecting typed neural predictions or
scoring this composition. This is a new reused-development experiment. It does
not revise the rejected [nine-label training trial](quality-99-protocol.md).

The epoch-two audit finds 32 exact other-entity false positives in the final
Person combination, versus 29 in the frozen reference. The current evaluator
converts Organization, Location and Miscellaneous predictions to background
before merging Person proposals. Therefore explicit other-type supervision
cannot directly remove a CRF name proposal. Test whether this discarded signal
helps precision without sacrificing the frozen reference's accuracy or recall.

## Fixed recipe

Use both complete checkpoints from the unchanged nine-label trial, epochs one
and two, with their existing hashes and training manifest. No training, averaging,
weight changes, threshold search or additional candidates. Preserve CRF .75,
neural .0 and the existing CRF-priority merge. Decode the nine-class token argmax
into complete BIO spans, treating orphan I tags as new spans and type changes as
boundaries. Preserve the existing Person decoder and minimum token confidence.

Remove a CRF Person proposal only when its exact start and end match a complete
neural Organization, Location or Miscellaneous span. Partial overlap does not
veto. Background predictions do not veto. No new confidence threshold applies;
this deliberately tests the unthresholded argmax signal. Run the unchanged merge
on the remaining CRF proposals and neural Person proposals. Count both removed
false positives and removed gold Person spans, including any recall harm.

Replay each checkpoint on the same two existing development inputs, with the
same canonical general/original domain text modes, local tokenizer, FP32 model,
batch eight and three CPU threads. These are additional inference runs for a
new composition, not first evaluations or independent evidence. Store full typed
predictions privately. Require exact equality of the replayed Person proposals
and confidences with each original private prediction cache before scoring.
Abort on any identity, record, annotation or parity mismatch; do not silently
replace the original cache. Preserve all prior reports.

Use the same exact-set scorer and unchanged research gate: both strata must
preserve frozen accuracy and precision, and the lower stratum's recall must
improve. Compare both complete epochs; prefer higher lower-stratum recall, with
earlier epoch breaking a tie. Report the separate 99% exact-accuracy/recall point
targets. Also show the original same-checkpoint composition for attribution.
The original `compare_quality.compare` gate can be reused after this experiment's
method and parity checks; its earlier comparison artifact must remain unchanged.

## Boundaries

All source/model/cache hashes and both-stratum populations must match the
existing frozen references. Full typed caches and any individual predictions
remain outside Git; commit aggregate counts and provenance only. Do not read or
score the blind review packets. A successful research gate would still require
nine-label export, real browser parity, full-entity checks and independent
labels before any promotion. The current three-label dmz package stays fixed.
