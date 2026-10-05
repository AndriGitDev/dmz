# Open-source PII integration survey

Checked against upstream sources on 5 October 2026. Model-card scores are not
comparable to DMZ's exact typed excerpt-set metric. The initial probe is only
100 authored synthetic excerpts and cannot establish independent field quality.
Pinned model/file identities and package metadata are retained privately in the
session's `sources/` and `models/` directories.

| Candidate | Upstream terms observed | Fit and decision at initial review |
| --- | --- | --- |
| [Presidio](https://github.com/data-privacy-stack/presidio) | MIT code; dependencies/models have separate terms | Extensible Python recognizer framework, existing pinned English baseline. Appropriate local server/research comparator. It is not a drop-in Go/TinyGo or browser runtime. Evaluate its added value rather than replacing the Icelandic core. The old Microsoft URL redirects to Data Privacy Stack. |
| [Knowledgator GLiNER-PII base](https://huggingface.co/knowledgator/gliner-pii-base-v1.0) | Model card Apache-2.0; GLiNER code Apache-2.0 | Initial FP32 ONNX is 664,764,803 bytes; tokenizer ~8.6 MB. Existing ONNX/JavaScript path makes a local browser spike feasible. Published quantized variants need their own quality comparisons. |
| [Fastino GLiNER2-PII](https://huggingface.co/fastino/gliner2-privacy-filter-PII-multi) | Model card and [code](https://github.com/fastino-ai/GLiNER2) Apache-2.0 | Current multilingual model, 42 advertised labels, no Icelandic language claim. Safetensors ~1.23 GB. Evaluate CPU inference; direct browser export requires separate work. The initial `person` label also finds role words, so its ontology needs scrutiny. |
| [dslim BERT NER](https://huggingface.co/dslim/bert-base-NER) | Model card MIT | English Person control, not a complete PII detector. ~433 MB safetensors; supports ordinary token-classification export. Do not relabel LOC or ORG as addresses or people. |
| [OpenAI Privacy Filter](https://github.com/openai/privacy-filter) | Code and [model](https://huggingface.co/openai/privacy-filter) Apache-2.0 | Local CPU/runtime support, primarily English, eight privacy categories and a distinct span-decoding contract. Original checkpoint ~2.80 GB; memory and browser cost must be measured. Account numbers, dates and URLs do not map one-to-one to current DMZ types. |
| [Piiranha](https://huggingface.co/iiiorg/piiranha-v1-detect-personal-information) | Current card CC-BY-NC-ND-4.0 | Excluded from the commercial integration shortlist under these terms. Its token-level headline accuracy is not DMZ's acceptance metric. |
| [NVIDIA GLiNER-PII](https://huggingface.co/nvidia/gliner-PII) | NVIDIA Open Model License Agreement | Separate model terms; not an Apache substitute. Lower priority than permissively licensed candidates for this bounded session. |
| [scrubadub](https://github.com/LeapBeyond/scrubadub) | Inspect exact package/license before integration | Python detector/cleanup framework. Does not itself solve browser-compatible English contextual detection. Reviewed as an alternative framework, not assumed superior to Presidio. |
| [detect-secrets](https://github.com/Yelp/detect-secrets) | Inspect exact package/license before integration | Secret scanning for code; useful specialist comparison, not a Person/address model. Disable any verification/network path if evaluated. |
| [libphonenumber](https://github.com/google/libphonenumber) and [Go port](https://github.com/nyaruka/phonenumbers) | Google Apache-2.0; Go port MIT with upstream data/code notices | Strong candidate for validated international telephone coverage shared between server and WASM. Investigate build compatibility, complete spans and false positives before changing rules. Valid number shape does not prove assignment or ownership. |

## Initial diagnostic finding

Broad additive composition is not automatically an improvement. Presidio and both
GLiNER models add English detections, but also add false positives. The frozen
trained DMZ path gets 25/40 general-English, 27/36 workplace-English and 22/24 mixed
excerpts exactly right. Presidio added without replacing existing spans reaches
27/40 and 30/36 English, but falls to 15/24 mixed. GLiNER2 added the same way reaches
32/40, 26/36 and 21/24. Retain all candidates/results; none is accepted as a default
on this evidence. These counts use the same frozen labels and deterministic
non-overlapping composition; they are not production or independent scores.

A more suitable integration may preserve structured validators and Icelandic
names while using a carefully bounded supplemental model. Schema selection,
normalization and any confidence calibration are model-selection work and must
be recorded as such, followed by separate probes and regression guards.
