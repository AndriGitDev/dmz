# Bounded entity-aware combination family

Predeclared on 5 October 2026, after the exact-veto result and before scoring
this family. This is adaptive research on reused development labels, not an
independent test. The original training and veto trials remain rejected.

The exact veto removes false names but also true names, so it cannot recover
recall alone. Test whether its precision margin permits additional CRF proposals,
and whether neural overlap priority recovers exact names displaced by CRF spans.
Use the already verified typed caches; do not retrain or run new inference.

Freeze 36 configurations: both complete nine-label epochs, CRF thresholds
`0, .5, .6, .65, .7, .75, .8, .9, .95` (the pre-existing ensemble grid), and either
CRF or neural overlap priority. Neural Person threshold stays zero. Every
configuration applies the same exact non-Person span veto before merging.
No confidence threshold on the veto, no partial-overlap veto, no extra candidates.
Retain every result, including failed configurations and true names removed.

Validate each cache against the completed veto report's hashes, original Person
cache parity and checkpoint/input/CRF identities. Validate the original training
recipe, both complete checkpoint identities and the same general/domain sample
populations. Use the shared exact-set scorer. Apply the unchanged research
criterion: preserve frozen precision and exact accuracy on both strata, and
strictly improve the lower stratum's recall. Separately report both 99% point
targets for each stratum and whether both strata meet them.

If more than one configuration qualifies, prefer higher lower-stratum recall,
then higher lower-stratum exact accuracy, then higher lower-stratum precision,
then a CRF threshold nearer .75, then CRF priority, then the earlier epoch, then the higher CRF threshold.
This tie-break is fixed before scoring. No result changes the default Shield
model; a qualifying research candidate still needs export/runtime parity,
full-system regression and independent annotation. Retain the reference if none
qualifies. Do not expand the grid after seeing these results.

The family has 36 comparisons in addition to the preceding experiments. Shared
reused data and adaptive hypothesis selection limit all resulting estimates.
Weights and individual predictions remain private; publish aggregate results
and identities only. The model-blind review packets are outside this work.
