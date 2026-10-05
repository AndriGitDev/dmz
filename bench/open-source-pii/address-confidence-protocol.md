# Compact address confidence selection

This additional model-selection experiment uses only the untouched projected
Gretel **validation** split (4,995 excerpts, SHA-256
`0363cb20728c0fc53b3c27cae5ecf9d03d04388ce4147907d01e9260bf5fb1de`).
Do not use public test predictions or either existing synthetic diagnostic to
select its threshold. The data remains public synthetic material with unknown
model-training overlap, not independent field evidence.

Use the pinned publisher QUInt8 GLiNER graph, original eight labels, flat
decoding and 0.3 inference threshold. Retain only decoded **address** spans for
this experiment. Filter these fixed decoded spans at thresholds
`0.3, 0.5, 0.7, 0.9, 0.95, 0.99, 0.995, 0.999`. Do not re-decode overlaps or
change labels for different thresholds.

Require at least 50 exact true address spans and a two-sided 95% Wilson lower
confidence bound of at least 0.99 for address-span precision. Among qualifying
thresholds, select the highest recall; break ties with the higher threshold.
If none qualifies, reject this confidence-only route without relaxing the gate.
Count malformed candidate responses as abstentions. Preserve all trials.

This precision filter is a model-selection guard, **not** DMZ's acceptance
metric. DMZ still requires 99% exact typed entity-set accuracy per excerpt and
99% entity recall per primary stratum. Passing this filter would require a
separate fixed-policy follow-up, mixed-language checks, original-byte validation
and browser-runtime evaluation before any product integration. Native confidence
selection cannot establish browser quality: the runtimes already differ on six
of the reused 90 diagnostic excerpts. No default feature is enabled by this work.
