# Bounded refinement after initial OSS screening

The initial 100 synthetic probes are now calibration data. No new variant is
called independent validation. Raw candidates and failures remain recorded.

The broad Presidio, GLiNER, GLiNER2, Privacy Filter and compact BERT additions all
reduced mixed-language exact accuracy. None is accepted as a default model.

Next fixed variants, before running refinement scores:

- Replay each original candidate with Person-only, address-only, and phone-only
  supplemental additions, preserving the frozen trained pipeline on overlaps.
- For confidence-bearing models, thresholds are original, 0.90 and 0.99.
  Privacy Filter does not expose a comparable span confidence; do not threshold
  its placeholder ordering score.
- Compact BERT additionally gets one documented name-boundary adaptation:
  join adjacent PERSON spans separated only by horizontal whitespace, strip
  an English honorific at the start (Mr, Mrs, Ms, Miss, Dr, Prof, Sir, Dame;
  optional period), and extend a token end across immediately following Unicode
  combining marks. Do not join across punctuation or words such as 'and'.
  Preserve raw outputs and compare both full precision and quantized variants.
- GLiNER2 additionally gets one schema trial: `full_name` in place of `person`,
  with the same other labels and default 0.5 threshold. This tests whether its
  original person ontology includes generic roles. No hand-authored role denylist.

Use results to choose at most one contextual integration path for a fresh,
separately authored diagnostic. English gains do not justify Icelandic or
mixed-language regressions. Model weights remain outside Git. Browser fit,
latency, exact UTF-8 spans, restoration and offline behavior are required.

The native libphonenumber implementation compiled with TinyGo but crashed at
runtime in protobuf reflection. The next phone variant imports its versioned
numbering metadata at build time and uses the existing portable Go detector.
It must reproduce the frozen phone probes and pass native/WASM conformance.
This is a restricted explicit-country-code adapter, not a claim to implement
every upstream parsing behavior.

GLiNER2's first Icelandic guard stopped at development row 500: it returned an
end character offset of 8 for a 7-character input. The raw failure is retained.
For the restarted guard, invalid offsets abstain for the entire candidate
excerpt; additive composition keeps the frozen pipeline. Do not clip offsets or
keep a partially valid model response. Report the failure count and measurement
revision. This is error handling, not a correction to corpus labels.

Fresh follow-up selection (before predictions on its 90 excerpts): the contextual
integration candidate is GLiNER2's original seven-label schema at 0.5, accepting
only address additions after frozen core/name matches. On calibration it added
eight true positives without adding false positives; its deployment size remains
a concern. All full-model outputs can be retained diagnostically, but do not
select a different threshold or type subset on the fresh set and call that
validation. The portable phone adapter is evaluated separately and combined.
