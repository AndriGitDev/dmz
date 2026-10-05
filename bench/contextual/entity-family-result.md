# Bounded combination family result

**Retain the frozen reference.** All 36 configurations in the
[predeclared family](entity-combination-family.md), frozen at `c3f1c52`, were
scored from the verified caches. None passes the research gate, and none meets
both 99% point targets in both strata. No configuration is selected or exported.

Every configuration fails to improve the lower stratum's recall. Nineteen also
fall below frozen general exact accuracy, five below general precision, and one
below domain exact accuracy. These failure counts overlap. Seventeen preserve
both strata's frozen accuracy and precision, but their recall still fails the
guard. Higher exact accuracy alone is not a successful result.

The [complete comparison](../results/quality-99-v1/entity-family-comparison.json)
retains all configurations, per-stratum exact counts, deltas, veto harm, failed
criteria, cache/code/protocol hashes and the null selection. The separate original
training and exact-veto comparisons remain unchanged. No new inference, training,
threshold grid expansion, browser replacement or blind-packet evaluation occurred.

This closes the declared family. It is adaptive selection on repeatedly reused,
Person-scoped development data. Even a passing configuration would have required
new export/runtime and independent quality checks; these failed results cannot
support broader accuracy claims.
